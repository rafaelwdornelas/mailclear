package config

import (
	"runtime"
	"time"
)

// AutoWorkerCount calcula um default razoável pro Count baseado em CPU.
// Cada worker passa a maior parte do tempo bloqueado em I/O (DNS, DB),
// então faz sentido ter muito mais workers que cores.
//   1 vCPU   → 8 workers
//   4 vCPU   → 32 workers
//   8 vCPU   → 64 workers
//   16 vCPU  → 128 workers
//   32+ vCPU → 256 workers (cap)
func AutoWorkerCount() int {
	n := runtime.NumCPU() * 8
	if n < 8 {
		n = 8
	}
	if n > 256 {
		n = 256
	}
	return n
}

// Defaults devolve a configuração HARDCODED da aplicação.
// Para tunar, edite esta função e recompile (sem .env, sem config.yaml).
func Defaults() *Config {
	return &Config{
		// ── HTTP ────────────────────────────────────────────────────
		HTTP: HTTPConfig{
			Addr:               ":8181", // PORTA FIXA
			ReadTimeout:        15 * time.Second,
			WriteTimeout:       60 * time.Second,
			IdleTimeout:        120 * time.Second,
			RateLimitPerMinute: 600,
		},

		// ── Logging ─────────────────────────────────────────────────
		Log: LogConfig{
			Level:  "info",
			Format: "json",
		},

		// ── PostgreSQL (instalado pelo install.sh com user "mailclear") ─
		DB: DBConfig{
			URL:             "postgres://mailclear:mailclear@127.0.0.1:5432/mailclear?sslmode=disable",
			MaxConns:        50,
			MinConns:        5,
			MaxConnLifetime: time.Hour,
		},

		// ── Redis ────────────────────────────────────────────────────
		Redis: RedisConfig{
			URL:       "redis://127.0.0.1:6379/0",
			PoolSize:  50,
			KeyPrefix: "mailclear:",
		},

		// ── DNS (Unbound local) ─────────────────────────────────────
		DNS: DNSConfig{
			Server:     "127.0.0.1:53",
			Timeout:    4 * time.Second,
			Retries:    2,
			EDNSBuffer: 4096,
		},

		// ── Workers — auto-tune por NumCPU + paralelismo intra-batch ─
		// BatchSize 5000 (subido de 2000): menos transições entre batches,
		// melhor aproveitamento do pre-warm (cada batch pre-aquece mais
		// domínios juntos, amortizando o custo).
		// EmailConcurrency 32 (subido de 16): com cache hit alto na fase 1,
		// a fase 2 (validação) escala com goroutines; saturação real é o
		// AdaptiveSemaphore do resolver, não esse limite local.
		Workers: WorkersConfig{
			Count:            AutoWorkerCount(),
			BatchSize:        5000,
			JobQueueBuffer:   20000,
			EmailConcurrency: 32,
		},

		// ── Rate Limit por domínio (cache + singleflight evitam excesso) ─
		RateLimit: RateLimitConfig{
			PerDomain: 500,
		},

		// ── Cache L1 (LRU) — 500k entradas ≈ 100MB RAM ──────────────
		Cache: CacheConfig{
			LRUSize:     500000,
			TTLMin:      5 * time.Minute,
			TTLMax:      24 * time.Hour,
			TTLNegative: 10 * time.Minute,
		},

		// ── Score Engine ────────────────────────────────────────────
		Score: ScoreConfig{
			ValidMin: 80,
			RiskyMin: 40,
			Weights: ScoreWeights{
				Syntax:     30,
				TLD:        10,
				MX:         35,
				AFallback:  15,
				Disposable: -60,
				Role:       -15,
				Typo:       -10,
				CatchAll:   -20,
			},
		},

		// ── Catch-all SMTP probe (custoso; off por default) ─────────
		CatchAll: CatchAllConfig{
			ProbeEnabled: false,
		},

		// ── Auth — VAZIO = sem auth (uso local). Para ativar, mude aqui. ─
		// ── Disposable: pull periódico da lista pública ─────────────
		Disposable: DisposableConfig{
			UpdateEnabled:  true,
			UpdateInterval: 24 * time.Hour,
			SourceURL:      "https://raw.githubusercontent.com/disposable-email-domains/disposable-email-domains/master/disposable_email_blocklist.conf",
		},
	}
}
