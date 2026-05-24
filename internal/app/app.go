// Package app compõe todas as dependências (DI manual) e cuida do lifecycle
// dos componentes (start/stop ordenado).
package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/rafaelwdornelas/mailclear/internal/cache"
	"github.com/rafaelwdornelas/mailclear/internal/config"
	mdns "github.com/rafaelwdornelas/mailclear/internal/dns"
	"github.com/rafaelwdornelas/mailclear/internal/jobs"
	"github.com/rafaelwdornelas/mailclear/internal/logger"
	"github.com/rafaelwdornelas/mailclear/internal/metrics"
	"github.com/rafaelwdornelas/mailclear/internal/server"
	"github.com/rafaelwdornelas/mailclear/internal/server/handlers"
	"github.com/rafaelwdornelas/mailclear/internal/storage"
	"github.com/rafaelwdornelas/mailclear/internal/validation"
	"github.com/rafaelwdornelas/mailclear/internal/validation/disposable"
	vdomain "github.com/rafaelwdornelas/mailclear/internal/validation/domain"
	"github.com/rafaelwdornelas/mailclear/internal/validation/role"
	"github.com/rafaelwdornelas/mailclear/internal/validation/stages"
	vtypo "github.com/rafaelwdornelas/mailclear/internal/validation/typo"
	"github.com/rafaelwdornelas/mailclear/internal/workers"
)

// App agrupa todos os componentes principais.
type App struct {
	Cfg      *config.Config
	Log      zerolog.Logger
	Metrics  *metrics.Metrics

	DB    *pgxpool.Pool
	Redis *redis.Client

	JobsRepo       *storage.JobsRepo
	ResultsRepo    *storage.ResultsRepo
	DomainsRepo    *storage.DomainsRepo
	DisposableRepo *storage.DisposableRepo

	DispReg   *disposable.Registry
	DispUpdt  *disposable.Updater
	RoleDet   *role.Detector
	Typo      *vtypo.Corrector
	DomClass  *vdomain.Classifier

	DNSClient  *mdns.Client
	DNSCache   *cache.Multi[mdns.MXResult]
	DNSLimiter *mdns.DomainLimiter
	Resolver   *mdns.Resolver
	AIMDCtrl   *mdns.AIMDController // adaptive concurrency controller

	Pipeline  *validation.Pipeline
	Scorer    *validation.ScoreEngine
	Validator validation.Validator

	Manager *jobs.Manager
	Pool    *workers.Pool

	HTTPServer *server.Server
}

// New monta o grafo de dependências.
func New(ctx context.Context, cfg *config.Config) (*App, error) {
	log := logger.New(cfg.Log.Level, cfg.Log.Format)
	met := metrics.New()

	// ── DB ───────────────────────────────────────────────────────────
	db, err := storage.NewPool(ctx, cfg.DB)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}

	// ── Redis ────────────────────────────────────────────────────────
	rdb, err := cache.NewRedisClient(ctx, cfg.Redis)
	if err != nil {
		return nil, fmt.Errorf("redis: %w", err)
	}

	// ── Repositories ─────────────────────────────────────────────────
	jobsRepo := storage.NewJobsRepo(db)
	resultsRepo := storage.NewResultsRepo(db)
	domainsRepo := storage.NewDomainsRepo(db)
	dispRepo := storage.NewDisposableRepo(db)

	// ── Validação: assets carregados ─────────────────────────────────
	dispReg := disposable.New()
	// Sobrescreve com lista persistida se disponível (não-bloqueante)
	if domains, err := dispRepo.List(ctx); err == nil && len(domains) > 0 {
		dispReg.LoadDomains(domains)
	}
	roleDet := role.New()
	if prefixes, err := dispRepo.ListRolePrefixes(ctx); err == nil && len(prefixes) > 0 {
		ps := make([]string, 0, len(prefixes))
		for _, p := range prefixes {
			ps = append(ps, p.Prefix)
		}
		roleDet.LoadList(ps)
	}
	typo := vtypo.NewCorrector()
	domClass := vdomain.NewClassifier()

	dispUpdt := disposable.NewUpdater(dispReg, cfg.Disposable.SourceURL, cfg.Disposable.UpdateInterval)

	// ── DNS Layer ────────────────────────────────────────────────────
	dnsClient := mdns.NewClient(cfg.DNS.Server, cfg.DNS.Timeout, cfg.DNS.Retries, cfg.DNS.EDNSBuffer)
	dnsLRU, err := cache.NewLRU[string, mdns.MXResult](cfg.Cache.LRUSize)
	if err != nil {
		return nil, fmt.Errorf("dns lru: %w", err)
	}
	dnsRedis := cache.NewRedis[mdns.MXResult](rdb, cfg.Redis.KeyPrefix+"dns:mx:")
	dnsCache := cache.NewMulti[mdns.MXResult](dnsLRU, dnsRedis)
	dnsLimiter := mdns.NewDomainLimiter(cfg.RateLimit.PerDomain, cfg.RateLimit.PerDomain)
	resolver := mdns.NewResolver(dnsClient, dnsCache, dnsLimiter, met.DNS,
		cfg.Cache.TTLMin, cfg.Cache.TTLMax, cfg.Cache.TTLNegative)

	// ── AIMD: ajuste adaptativo da concorrência DNS ────────────────
	// Lê erro rate dos contadores DNS a cada 5s. Sobe limit se < 1%
	// erro, divide por 2 se > 5%. Cap em [16, 4096].
	aimdReader := mdns.MakeRateReader(met.DNS)
	aimdCtrl := mdns.NewAIMDController(resolver.Inflight, aimdReader, log).
		WithMetrics(met.DNS)

	// ── Pipeline de validação ───────────────────────────────────────
	pipe := validation.NewPipeline(met.Validation,
		stages.Normalize{},
		stages.Syntax{},
		&stages.TLD{},
		stages.NewTypo(typo, 2),
		stages.NewDisposable(dispReg),
		stages.NewRole(roleDet),
		stages.NewMX(resolver, domClass),
	)
	scorer := validation.NewScoreEngine(cfg.Score)
	val := validation.NewDefaultValidator(pipe, scorer)

	// ── Worker pool + Jobs manager ──────────────────────────────────
	// Handler faz domain pre-warming + paralelismo intra-batch (errgroup).
	log.Info().
		Int("workers", cfg.Workers.Count).
		Int("batch_size", cfg.Workers.BatchSize).
		Int("email_concurrency", cfg.Workers.EmailConcurrency).
		Int("queue_buffer", cfg.Workers.JobQueueBuffer).
		Msg("worker pool")

	// Stale-job recovery: marca como failed todo job em pending/running com
	// updated_at > 5 min (sobrou de um run anterior que crashou ou foi
	// reiniciado). Sem isso, jobs órfãos ficavam running pra sempre.
	{
		recoverCtx, cancelRec := context.WithTimeout(ctx, 10*time.Second)
		stale, err := jobsRepo.MarkStaleAsFailed(recoverCtx, 5*time.Minute)
		cancelRec()
		if err != nil {
			log.Warn().Err(err).Msg("stale-job recovery falhou (ignorando)")
		} else if stale > 0 {
			log.Warn().Int64("count", stale).Msg("jobs órfãos marcados como failed (boot recovery)")
		}
	}

	taskHandler := jobs.MakeTaskHandler(val, resultsRepo, jobsRepo, resolver, cfg.Workers.EmailConcurrency, met.Jobs)
	pool := workers.NewPool(cfg.Workers.Count, cfg.Workers.JobQueueBuffer, taskHandler, log, met.Workers)
	mgr := jobs.NewManager(jobsRepo, resultsRepo, val, pool, cfg.Workers.BatchSize)

	// ── HTTP server ─────────────────────────────────────────────────
	health := handlers.NewHealth(db, rdb, cfg.DNS.Server)
	validateH := handlers.NewValidate(val)
	jobsH := handlers.NewJobs(mgr, resultsRepo)
	importH := handlers.NewImport(mgr, cfg.Workers.BatchSize)
	statsH := handlers.NewStats(resultsRepo)
	adminH := &handlers.AdminHandlers{
		DB:             db,
		Redis:          rdb,
		DNSServer:      cfg.DNS.Server,
		DNSCache:       dnsCache,
		DNSResolver:    resolver,
		DispRegistry:   dispReg,
		DispRepo:       dispRepo,
		JobsRepo:       jobsRepo,
		ResultsRepo:    resultsRepo,
		Pool:           pool,
		RedisKeyPrefix: cfg.Redis.KeyPrefix,
		Version:        "1.0",
		StartedAt:      time.Now(),
	}
	adminOpsH := handlers.NewAdminOps(jobsRepo, detectSelfUnit())
	dashH := handlers.NewDashboard()

	routes := &server.Routes{
		Log:       log,
		Metrics:   met,
		Health:    health,
		Validate:  validateH,
		Jobs:      jobsH,
		Import:    importH,
		Stats:     statsH,
		Admin:     adminH,
		AdminOps:  adminOpsH,
		Dashboard: dashH,
	}
	srv := server.New(cfg.HTTP, routes.Build(), log)

	a := &App{
		Cfg: cfg, Log: log, Metrics: met,
		DB: db, Redis: rdb,
		JobsRepo: jobsRepo, ResultsRepo: resultsRepo,
		DomainsRepo: domainsRepo, DisposableRepo: dispRepo,
		DispReg: dispReg, DispUpdt: dispUpdt, RoleDet: roleDet, Typo: typo, DomClass: domClass,
		DNSClient: dnsClient, DNSCache: dnsCache, DNSLimiter: dnsLimiter, Resolver: resolver,
		AIMDCtrl: aimdCtrl,
		Pipeline: pipe, Scorer: scorer, Validator: val,
		Manager: mgr, Pool: pool,
		HTTPServer: srv,
	}
	return a, nil
}

// Shutdown libera recursos na ordem correta.
//
// Ordem importa:
//  1. HTTP server → para aceitar requests novos (não cria mais jobs).
//  2. Worker pool → drena fila e espera batches em curso terminarem
//     (até 120s — batches de 5000 podem demorar sob carga).
//  3. Conexões externas (Redis + DB) — fechadas por último para que os
//     workers terminem suas escritas pendentes antes.
//
// Jobs cujos batches ficaram na fila não-consumida vão ser marcados como
// failed pelo stale-recovery no próximo boot (updated_at fica parado
// porque o handler que incrementa o progresso só roda quando batch executa).
func (a *App) Shutdown() {
	a.Log.Info().Msg("iniciando shutdown")

	// 1) HTTP server (para de aceitar requests novos)
	if a.HTTPServer != nil {
		if err := a.HTTPServer.Shutdown(20 * time.Second); err != nil {
			a.Log.Error().Err(err).Msg("erro no http shutdown")
		}
	}

	// 2) Worker pool (drena tasks pendentes — bumped para 120s)
	if a.Pool != nil {
		a.Pool.Stop(120 * time.Second)
	}

	// 3) Conexões externas
	if a.Redis != nil {
		_ = a.Redis.Close()
	}
	if a.DB != nil {
		a.DB.Close()
	}
	a.Log.Info().Msg("shutdown concluído")
}


// detectSelfUnit deduz a systemd unit do processo atual pelo nome do binário.
// Permite o handler de restart tratar self-restart de forma especial (responder
// antes do shutdown derrubar a conexão). Retorna "" se não conseguir detectar.
func detectSelfUnit() string {
	base := filepath.Base(os.Args[0])
	switch {
	case strings.Contains(base, "mailclear-api"):
		return "mailclear-api.service"
	case strings.Contains(base, "mailclear-worker"):
		return "mailclear-worker.service"
	}
	return ""
}
