package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	mdns "github.com/rafaelwdornelas/mailclear/internal/dns"
	"github.com/rafaelwdornelas/mailclear/internal/metrics"
	"github.com/rafaelwdornelas/mailclear/internal/storage"
	"github.com/rafaelwdornelas/mailclear/internal/validation"
	"github.com/rafaelwdornelas/mailclear/internal/workers"
)

// MakeTaskHandler devolve um workers.TaskHandler que:
//  1. Pre-warm de domínios únicos do batch — SÓ os que NÃO estão em cache
//     (verifica L1+L2 antes via Resolver.IsCached, pulando singleflight
//     e slot do AdaptiveSemaphore pra hits cacheados).
//  2. Valida emails individualmente em paralelo (errgroup com SetLimit).
//  3. Persiste o batch (CopyFrom) e atualiza progresso.
//
// Cada fase é cronometrada e exportada em `job_batch_phase_duration_seconds`
// pra identificar gargalo real (DNS vs CPU vs DB).
//
// concurrency controla o paralelismo dentro do batch (default 64).
// resolver é opcional — se nil, pula o pre-warming.
// jobsMet é opcional — se nil, não emite métricas de fase.
func MakeTaskHandler(
	val validation.Validator,
	results *storage.ResultsRepo,
	jobsRepo *storage.JobsRepo,
	resolver *mdns.Resolver,
	concurrency int,
	jobsMet *metrics.JobsMetrics,
) workers.TaskHandler {
	if concurrency <= 0 {
		concurrency = 64
	}
	return func(ctx context.Context, t *workers.Task) error {
		if len(t.Emails) == 0 {
			return nil
		}

		// ── Fase 1: pre-warm com fast-path de cache ───────────────────
		// Só chama LookupMX dos domínios que NÃO têm MX em cache.
		// Domínios cacheados → 0 overhead (sem singleflight, sem slot).
		if resolver != nil {
			phaseStart := time.Now()
			domains := uniqueDomains(t.Emails)
			if len(domains) > 0 {
				// Filtra os que precisam de fato bater no resolver
				toResolve := make([]string, 0, len(domains))
				skipped := 0
				for _, d := range domains {
					if resolver.IsCached(ctx, d) {
						skipped++
						continue
					}
					toResolve = append(toResolve, d)
				}
				if jobsMet != nil && skipped > 0 {
					jobsMet.PrewarmSkips.Add(float64(skipped))
				}
				if len(toResolve) > 0 {
					g, gctx := errgroup.WithContext(ctx)
					g.SetLimit(concurrency)
					for _, d := range toResolve {
						d := d
						g.Go(func() error {
							_, _ = resolver.LookupMX(gctx, d)
							return nil
						})
					}
					_ = g.Wait()
				}
			}
			if jobsMet != nil {
				jobsMet.PhaseDuration.WithLabelValues("pre_warm").
					Observe(time.Since(phaseStart).Seconds())
			}
		}

		// ── Fase 2: validação em paralelo ──────────────────────────────
		phaseStart := time.Now()
		emails := make([]*validation.Email, len(t.Emails))
		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(concurrency)
		for i, raw := range t.Emails {
			i, raw := i, raw
			g.Go(func() error {
				e, err := val.Validate(gctx, raw)
				if err != nil {
					// ctx cancelado/timeout: aborta o batch inteiro pra evitar
					// persistência parcial. Outros erros são tratados no email
					// (Validate sempre retorna *Email não-nil).
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						return err
					}
					if e != nil {
						e.Status = validation.StatusUnknown
						e.AddReason("validation_error", err.Error())
					}
				}
				emails[i] = e
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return err
		}
		if jobsMet != nil {
			jobsMet.PhaseDuration.WithLabelValues("validate").
				Observe(time.Since(phaseStart).Seconds())
		}

		// ── Fase 3: monta records, contadores e persiste ───────────────
		phaseStart = time.Now()
		records := make([]*storage.EmailResult, 0, len(emails))
		delta := storage.ProgressDelta{Processed: int64(len(emails))}

		for _, e := range emails {
			if e == nil {
				continue
			}
			reasons, _ := json.Marshal(e.Reasons)
			var suggested *string
			if e.Suggested != "" {
				s := e.Suggested
				suggested = &s
			}
			classification := string(e.Classification)
			if classification == "" {
				classification = string(validation.ClassificationUnknown)
			}
			records = append(records, &storage.EmailResult{
				JobID:           t.JobID,
				EmailOriginal:   e.Original,
				EmailNormalized: orFallback(e.Normalized, e.Original),
				LocalPart:       e.LocalPart,
				Domain:          e.Domain,
				Status:          string(e.Status),
				Classification:  classification,
				Score:           int16(e.Score),
				Reasons:         reasons,
				SuggestedEmail:  suggested,
				IsDisposable:    e.IsDisposable,
				IsRole:          e.IsRole,
				HasMX:           e.HasMX,
			})
			switch e.Status {
			case validation.StatusValid:
				delta.Valid++
			case validation.StatusInvalid:
				delta.Invalid++
			case validation.StatusRisky:
				delta.Risky++
			case validation.StatusUnknown:
				delta.Unknown++
			case validation.StatusDisposable:
				delta.Disposable++
			}
		}

		if _, err := results.BulkInsert(ctx, records); err != nil {
			markJobFailed(jobsRepo, t.JobID, "bulk_insert", err)
			return err
		}
		if err := jobsRepo.IncrementProgress(ctx, t.JobID, delta); err != nil {
			markJobFailed(jobsRepo, t.JobID, "increment_progress", err)
			return err
		}
		if jobsMet != nil {
			jobsMet.PhaseDuration.WithLabelValues("insert").
				Observe(time.Since(phaseStart).Seconds())
		}
		return nil
	}
}

// markJobFailed transiciona o job para `failed` e grava o motivo em metadata.
// Usa context.Background() porque o ctx do batch pode estar cancelado.
// Timeout curto (5s) garante que não trave o worker.
func markJobFailed(jobsRepo *storage.JobsRepo, jobID uuid.UUID, phase string, cause error) {
	bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := jobsRepo.UpdateStatus(bgCtx, jobID, "failed"); err != nil {
		log.Error().Err(err).Str("job_id", jobID.String()).Msg("falha ao marcar job como failed")
		return
	}
	if err := jobsRepo.SetLastError(bgCtx, jobID, phase, cause.Error()); err != nil {
		log.Warn().Err(err).Str("job_id", jobID.String()).Msg("falha ao gravar last_error em metadata")
	}
}

// uniqueDomains extrai os domínios únicos do batch.
func uniqueDomains(raws []string) []string {
	seen := make(map[string]struct{}, len(raws)/4)
	out := make([]string, 0, len(seen))
	for _, raw := range raws {
		at := strings.LastIndexByte(raw, '@')
		if at <= 0 || at == len(raw)-1 {
			continue
		}
		d := strings.ToLower(strings.TrimSpace(raw[at+1:]))
		if d == "" {
			continue
		}
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		out = append(out, d)
	}
	return out
}

func orFallback(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}

var _ sync.WaitGroup
