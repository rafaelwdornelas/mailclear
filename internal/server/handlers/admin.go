package handlers

import (
	"context"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/rafaelwdornelas/mailclear/internal/cache"
	mdns "github.com/rafaelwdornelas/mailclear/internal/dns"
	"github.com/rafaelwdornelas/mailclear/internal/storage"
	"github.com/rafaelwdornelas/mailclear/internal/validation/disposable"
)

// AdminHandlers expõe endpoints de status agregado e ações administrativas.
// Usado pelo dashboard HTML em GET /.
type AdminHandlers struct {
	DB           *pgxpool.Pool
	Redis        *redis.Client
	DNSServer    string
	DNSCache     *cache.Multi[mdns.MXResult]
	DispRegistry *disposable.Registry
	DispRepo     *storage.DisposableRepo
	JobsRepo     *storage.JobsRepo
	ResultsRepo  *storage.ResultsRepo
	RedisKeyPrefix string
	Version      string
	StartedAt    time.Time
}

// StatusResponse agrega tudo o que o dashboard consome.
type StatusResponse struct {
	Version       string                 `json:"version"`
	UptimeSeconds int64                  `json:"uptime_seconds"`
	Now           time.Time              `json:"now"`
	Services      map[string]string      `json:"services"`
	Stats         *storage.GlobalStats   `json:"stats"`
	Jobs          []*storage.Job         `json:"jobs"`
	Cache         CacheStats             `json:"cache"`
	Disposable    DisposableStats        `json:"disposable"`
	Runtime       RuntimeStats           `json:"runtime"`
}

// CacheStats agrega contadores de cache.
type CacheStats struct {
	RedisKeys      int64 `json:"redis_keys"`
	RedisMemoryHuman string `json:"redis_memory_human"`
}

// DisposableStats info sobre a lista de disposable.
type DisposableStats struct {
	InMemorySize int `json:"in_memory_size"`
	DBSize       int64 `json:"db_size"`
}

// RuntimeStats info do runtime Go.
type RuntimeStats struct {
	Goroutines int    `json:"goroutines"`
	NumCPU     int    `json:"num_cpu"`
	GoVersion  string `json:"go_version"`
	MemAllocMB uint64 `json:"mem_alloc_mb"`
	MemSysMB   uint64 `json:"mem_sys_mb"`
}

// Status devolve o estado agregado em JSON. Consumido pelo dashboard via fetch().
//
// Todas as queries de I/O rodam em paralelo via goroutines — assim o
// endpoint não fica limitado pela query mais lenta (Global, que faz
// COUNT(*) FILTER... numa tabela de milhões de linhas).
//
// Timeout generoso (15s): em carga pesada (256 workers fazendo COPY),
// até queries simples podem demorar segundos.
func (h *AdminHandlers) Status(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	resp := StatusResponse{
		Version:       h.Version,
		UptimeSeconds: int64(time.Since(h.StartedAt).Seconds()),
		Now:           time.Now(),
		Services:      map[string]string{"api": "ok"},
	}

	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)

	setService := func(name, val string) {
		mu.Lock()
		resp.Services[name] = val
		mu.Unlock()
	}

	// ── Probes de serviços em paralelo ─────────────────────────────
	if h.DB != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := h.DB.Ping(ctx); err != nil {
				setService("db", "fail")
			} else {
				setService("db", "ok")
			}
		}()
	}
	if h.Redis != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := h.Redis.Ping(ctx).Err(); err != nil {
				setService("redis", "fail")
			} else {
				setService("redis", "ok")
			}
		}()
	}
	if h.DNSServer != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := pingDNS(ctx, h.DNSServer); err != nil {
				setService("dns", "fail")
			} else {
				setService("dns", "ok")
			}
		}()
	}

	// ── Queries de DB em paralelo ──────────────────────────────────
	if h.ResultsRepo != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if s, err := h.ResultsRepo.Global(ctx); err == nil {
				mu.Lock()
				resp.Stats = s
				mu.Unlock()
			}
		}()
	}
	if h.JobsRepo != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if jobs, err := h.JobsRepo.List(ctx, 10, 0); err == nil {
				mu.Lock()
				resp.Jobs = jobs
				mu.Unlock()
			}
		}()
	}

	// ── Redis stats em paralelo ────────────────────────────────────
	if h.Redis != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if n, err := h.Redis.DBSize(ctx).Result(); err == nil {
				mu.Lock()
				resp.Cache.RedisKeys = n
				mu.Unlock()
			}
			if info, err := h.Redis.Info(ctx, "memory").Result(); err == nil {
				mu.Lock()
				resp.Cache.RedisMemoryHuman = extractInfoLine(info, "used_memory_human:")
				mu.Unlock()
			}
		}()
	}

	// ── Disposable ──────────────────────────────────────────────────
	// In-memory (sem I/O): direto
	if h.DispRegistry != nil {
		resp.Disposable.InMemorySize = h.DispRegistry.Size()
	}
	if h.DispRepo != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if n, err := h.DispRepo.Count(ctx); err == nil {
				mu.Lock()
				resp.Disposable.DBSize = n
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// ── Runtime ─────────────────────────────────────────────────────
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	resp.Runtime = RuntimeStats{
		Goroutines: runtime.NumGoroutine(),
		NumCPU:     runtime.NumCPU(),
		GoVersion:  runtime.Version(),
		MemAllocMB: m.Alloc / 1024 / 1024,
		MemSysMB:   m.Sys / 1024 / 1024,
	}

	WriteJSON(w, http.StatusOK, resp)
}

// ClearCache limpa o LRU in-memory e (opcionalmente) a partição Redis com
// o prefixo da app. Não toca em outras keys do Redis.
func (h *AdminHandlers) ClearCache(w http.ResponseWriter, r *http.Request) {
	deleted := int64(0)
	if h.DNSCache != nil {
		h.DNSCache.PurgeL1()
	}
	if h.Redis != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// SCAN + DEL para apagar apenas as chaves com nosso prefixo
		iter := h.Redis.Scan(ctx, 0, h.RedisKeyPrefix+"*", 1000).Iterator()
		var keys []string
		for iter.Next(ctx) {
			keys = append(keys, iter.Val())
			if len(keys) >= 1000 {
				h.Redis.Del(ctx, keys...)
				deleted += int64(len(keys))
				keys = keys[:0]
			}
		}
		if len(keys) > 0 {
			h.Redis.Del(ctx, keys...)
			deleted += int64(len(keys))
		}
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"l1_purged":     true,
		"redis_deleted": deleted,
	})
}

// ReloadDisposable força recarregar a lista de disposable do banco para a memória.
func (h *AdminHandlers) ReloadDisposable(w http.ResponseWriter, r *http.Request) {
	if h.DispRegistry == nil || h.DispRepo == nil {
		WriteError(w, http.StatusServiceUnavailable, "NOT_AVAILABLE", "Registry/Repo indisponível")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	domains, err := h.DispRepo.List(ctx)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	h.DispRegistry.LoadDomains(domains)
	WriteJSON(w, http.StatusOK, map[string]any{
		"loaded": len(domains),
	})
}

// CancelAllRunning cancela todos os jobs em pending/running/paused.
func (h *AdminHandlers) CancelAllRunning(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	tag, err := h.DB.Exec(ctx, `
		UPDATE jobs SET status='cancelled', finished_at=now()
		WHERE status IN ('pending', 'running', 'paused')
	`)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"cancelled": tag.RowsAffected(),
	})
}

// extractInfoLine extrai um valor "key:value" do INFO do Redis.
func extractInfoLine(info, prefix string) string {
	for i := 0; i < len(info); i++ {
		if i+len(prefix) > len(info) {
			break
		}
		if info[i:i+len(prefix)] == prefix {
			end := i + len(prefix)
			for end < len(info) && info[end] != '\r' && info[end] != '\n' {
				end++
			}
			return info[i+len(prefix) : end]
		}
	}
	return ""
}
