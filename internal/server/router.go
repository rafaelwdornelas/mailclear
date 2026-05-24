package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMW "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/rafaelwdornelas/mailclear/internal/logger"
	"github.com/rafaelwdornelas/mailclear/internal/metrics"
	"github.com/rafaelwdornelas/mailclear/internal/server/handlers"
	mw "github.com/rafaelwdornelas/mailclear/internal/server/middleware"

	"github.com/rs/zerolog"
)

// Routes monta o router Chi com todos os middlewares e rotas.
//
// AUTH REMOVIDA: a API roda completamente aberta. Uso esperado é local
// (127.0.0.1) ou atrás de firewall/proxy. Sem token, sem header.
type Routes struct {
	Log       zerolog.Logger
	Metrics   *metrics.Metrics
	Health    *handlers.Health
	Validate  *handlers.ValidateHandlers
	Jobs      *handlers.JobsHandlers
	Import    *handlers.ImportHandlers
	Stats     *handlers.StatsHandlers
	Admin     *handlers.AdminHandlers
	AdminOps  *handlers.AdminOpsHandlers
	Dashboard *handlers.Dashboard
}

// Build devolve o http.Handler raiz.
func (r *Routes) Build() http.Handler {
	router := chi.NewRouter()

	// Middlewares globais (ordem importa)
	router.Use(chiMW.RequestID)
	router.Use(chiMW.RealIP)
	router.Use(chiMW.Recoverer)
	router.Use(chiMW.Compress(5))
	router.Use(logger.HTTPMiddleware(r.Log))
	router.Use(mw.Metrics(r.Metrics.HTTP))

	// ── Dashboard + health + métricas ───────────────────────────────
	if r.Dashboard != nil {
		router.Get("/", r.Dashboard.Index)
	}
	router.Get("/healthz", r.Health.Live)
	router.Get("/readyz", r.Health.Ready)
	router.Handle("/metrics", promhttp.HandlerFor(r.Metrics.Reg, promhttp.HandlerOpts{}))

	// ── Admin (status + ações de manutenção) ───────────────────────
	if r.Admin != nil {
		router.Get("/admin/status", r.Admin.Status)
		router.Post("/admin/cache/clear", r.Admin.ClearCache)
		router.Post("/admin/disposable/reload", r.Admin.ReloadDisposable)
		router.Post("/admin/jobs/cancel-all", r.Admin.CancelAllRunning)
	}

	// ── Admin ops (restart, logs, force-fail) ───────────────────────
	if r.AdminOps != nil {
		router.Post("/admin/services/restart", r.AdminOps.RestartService)
		router.Get("/admin/logs", r.AdminOps.Logs)
		router.Get("/admin/logs/stream", r.AdminOps.LogsStream)
		router.Post("/admin/jobs/{id}/force-fail", r.AdminOps.ForceFail)
	}

	// ── API v1 ──────────────────────────────────────────────────────
	router.Route("/api/v1", func(g chi.Router) {
		g.Post("/validate", r.Validate.Single)
		g.Post("/validate/batch", r.Validate.Batch)

		g.Post("/import/csv", r.Import.CSV)

		g.Route("/jobs", func(j chi.Router) {
			j.Get("/", r.Jobs.List)
			j.Get("/{id}", r.Jobs.Get)
			j.Get("/{id}/results", r.Jobs.Results)
			j.Get("/{id}/export.csv", r.Jobs.ExportCSV)
			j.Post("/{id}/cancel", r.Jobs.Cancel)
			j.Post("/{id}/requeue", r.Jobs.Requeue)
		})

		g.Get("/stats", r.Stats.Global)
	})

	return router
}
