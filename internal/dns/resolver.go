package dns

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/sync/singleflight"

	"github.com/rafaelwdornelas/mailclear/internal/cache"
	"github.com/rafaelwdornelas/mailclear/internal/metrics"
)

// MXResult agrega o resultado de uma resolução MX (com fallback A).
type MXResult struct {
	Domain     string     `json:"domain"`
	MX         []MXRecord `json:"mx,omitempty"`
	HasMX      bool       `json:"has_mx"`
	HasA       bool       `json:"has_a"`
	A          []string   `json:"a,omitempty"`
	Status     string     `json:"status"` // ok|no_mx|nxdomain|timeout|servfail
	TTLSeconds int32      `json:"ttl_seconds"`
}

// MXRecord representa um único registro MX com prioridade.
type MXRecord struct {
	Host     string `json:"host"`
	Priority uint16 `json:"priority"`
}

// Constantes de robustez. A concorrência DNS real é controlada pelo
// AdaptiveSemaphore (AIMD), não por uma constante fixa.
const (
	// inflightInitial: limite inicial agressivo. Servidores dedicados (12+ cores,
	// Unbound com 16 threads) sustentam 256 fácil. O AIMD ajusta a partir daí:
	// sobe +64 a cada 3s se erro de timeout < 1%; corta /2 se > 5%.
	// Se a máquina for pequena, AIMD detecta saturação real e baixa rápido.
	inflightInitial = 256

	// inflightMax: teto absoluto. AIMD nunca passa disso.
	inflightMax = 4096

	// maxRetries para timeout/servfail. Total = 1 tentativa inicial + N retries.
	maxRetries = 2

	// retryBackoffBase é multiplicado por (tentativa+1) — ex: 150ms, 300ms.
	retryBackoffBase = 150 * time.Millisecond
)

// Resolver é a fachada de alto nível para lookups DNS com cache + dedup
// + rate-limit por domínio + adaptive in-flight + retry com backoff.
type Resolver struct {
	client   *Client
	cache    *cache.Multi[MXResult]
	sf       singleflight.Group
	limiter  *DomainLimiter
	met      *metrics.DNSMetrics
	ttlMin   time.Duration
	ttlMax   time.Duration
	ttlNeg   time.Duration
	Inflight *AdaptiveSemaphore // exportado pra o AIMDController ajustar
}

// NewResolver compõe o resolver final.
func NewResolver(client *Client, cache *cache.Multi[MXResult], limiter *DomainLimiter,
	met *metrics.DNSMetrics, ttlMin, ttlMax, ttlNeg time.Duration,
) *Resolver {
	return &Resolver{
		client:   client,
		cache:    cache,
		limiter:  limiter,
		met:      met,
		ttlMin:   ttlMin,
		ttlMax:   ttlMax,
		ttlNeg:   ttlNeg,
		Inflight: NewAdaptiveSemaphore(inflightInitial, inflightMax),
	}
}

// IsCached devolve true se o domínio já tem MX (definitivo) em cache L1+L2.
// Usado pelo pre-warm pra pular chamadas redundantes: domínios cacheados
// não passam por singleflight nem por slot do AdaptiveSemaphore.
//
// Não diferencia "cache hit OK" de "cache hit nxdomain" — ambos contam como
// "não precisa resolver de novo".
func (r *Resolver) IsCached(ctx context.Context, domain string) bool {
	if r.cache == nil {
		return false
	}
	domain = strings.ToLower(strings.TrimSuffix(domain, "."))
	if domain == "" {
		return false
	}
	_, err := r.cache.Get(ctx, cache.MXKey(domain))
	return err == nil
}

// LookupMX devolve o resultado MX (com fallback A) para o domínio.
//
// Defesas em camada (todas implementadas):
//   1. Cache L1+L2 — evita re-consulta em domínios já vistos.
//   2. Singleflight — várias goroutines pedindo o mesmo domínio = 1 query.
//   3. Rate-limit por domínio — não martela autoritativo único.
//   4. Limit global in-flight — não satura o Unbound (1024 queries paralelas).
//   5. Retry com backoff — em timeout/servfail, tenta de novo 2x.
//   6. Cache só de respostas DEFINITIVAS — não envenena com timeout transitório.
//
// Retorno: caller DEVE checar erro. timeout/servfail/nxdomain vêm como erros
// tipados — o caller distingue "domínio inválido" de "DNS falhou".
func (r *Resolver) LookupMX(ctx context.Context, domain string) (*MXResult, error) {
	domain = strings.ToLower(strings.TrimSuffix(domain, "."))
	if domain == "" {
		return nil, ErrNXDomain
	}

	key := cache.MXKey(domain)

	// 1) Cache (L1+L2)
	if r.cache != nil {
		if v, err := r.cache.Get(ctx, key); err == nil {
			r.met.CacheHitsTotal.WithLabelValues("multi", "MX").Inc()
			vc := v
			return &vc, errFromStatus(&vc)
		}
	}

	// 2) Singleflight + rate-limit por domínio + limit global + retry
	v, sfErr, shared := r.sf.Do(key, func() (any, error) {
		if r.limiter != nil {
			if rerr := r.limiter.Wait(ctx, domain); rerr != nil {
				r.met.RateLimited.WithLabelValues(domain).Inc()
				return nil, rerr
			}
		}

		// Adquire slot no semáforo adaptativo. Bloqueia se atingiu limit
		// atual (controlado pelo AIMD baseado em erro rate).
		if err := r.Inflight.Acquire(ctx); err != nil {
			return nil, err
		}
		defer r.Inflight.Release()

		// Tenta resolver com retry transparente em falhas transitórias.
		res := r.resolveMXWithRetry(ctx, domain)

		// Cache APENAS respostas definitivas (não cacheia timeout/servfail —
		// se o Unbound saturar agora, é injusto perpetuar o erro pros próximos
		// emails do mesmo domínio que entrarem em outro batch sem saturação).
		if r.cache != nil && isCacheable(res) {
			ttl := r.cacheTTLFor(res)
			_ = r.cache.Set(ctx, key, *res, ttl)
		}
		return res, nil
	})

	if shared {
		r.met.Singleflight.WithLabelValues("MX").Inc()
	}
	if sfErr != nil {
		return nil, sfErr
	}
	out := v.(*MXResult)
	return out, errFromStatus(out)
}

// resolveMXWithRetry tenta resolveMX algumas vezes em falha transitória.
// timeout/servfail = transitório (Unbound ou autoritativo engasgou).
// nxdomain/no_mx/ok = definitivo (não adianta retry).
func (r *Resolver) resolveMXWithRetry(ctx context.Context, domain string) *MXResult {
	var res *MXResult
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Backoff progressivo: 150ms, 300ms.
			delay := time.Duration(attempt) * retryBackoffBase
			select {
			case <-ctx.Done():
				return res
			case <-time.After(delay):
			}
		}
		res = r.resolveMX(ctx, domain)
		if res.Status != "timeout" && res.Status != "servfail" {
			return res // definitivo
		}
	}
	return res
}

// isCacheable indica se o resultado é "definitivo" e merece ir pro cache.
// Não cachear erros transitórios evita propagar problema de carga momentânea.
func isCacheable(res *MXResult) bool {
	if res == nil {
		return false
	}
	switch res.Status {
	case "ok", "no_mx", "nxdomain":
		return true
	}
	return false
}

// errFromStatus mapeia res.Status para o erro canônico esperado pelo caller.
func errFromStatus(res *MXResult) error {
	if res == nil {
		return nil
	}
	switch res.Status {
	case "nxdomain":
		return ErrNXDomain
	case "timeout":
		return ErrTimeout
	case "servfail":
		return ErrServFail
	}
	return nil
}

// resolveMX faz a resolução real chamando o client e classificando o resultado.
func (r *Resolver) resolveMX(ctx context.Context, domain string) *MXResult {
	res := &MXResult{Domain: domain, Status: "ok"}
	start := time.Now()
	defer func() {
		r.met.QueryDuration.WithLabelValues("MX").Observe(time.Since(start).Seconds())
	}()

	msg, err := r.client.Query(ctx, domain, dns.TypeMX)
	switch {
	case err == nil:
		ttl := uint32(3600)
		for _, ans := range msg.Answer {
			mx, ok := ans.(*dns.MX)
			if !ok {
				continue
			}
			res.MX = append(res.MX, MXRecord{Host: strings.TrimSuffix(mx.Mx, "."), Priority: mx.Preference})
			if ans.Header().Ttl < ttl {
				ttl = ans.Header().Ttl
			}
		}
		sort.SliceStable(res.MX, func(i, j int) bool { return res.MX[i].Priority < res.MX[j].Priority })
		res.HasMX = len(res.MX) > 0
		res.TTLSeconds = int32(ttl)

		if !res.HasMX {
			// MX vazio → fallback A (RFC 5321 §5.1)
			r.fallbackA(ctx, res)
		}
		r.met.QueriesTotal.WithLabelValues("MX", "ok").Inc()

	case errors.Is(err, ErrNXDomain):
		res.Status = "nxdomain"
		res.TTLSeconds = int32(r.ttlNeg.Seconds())
		r.met.QueriesTotal.WithLabelValues("MX", "nxdomain").Inc()

	case errors.Is(err, ErrServFail):
		res.Status = "servfail"
		res.TTLSeconds = int32(r.ttlNeg.Seconds())
		r.met.QueriesTotal.WithLabelValues("MX", "servfail").Inc()

	case errors.Is(err, ErrTimeout):
		res.Status = "timeout"
		res.TTLSeconds = int32(r.ttlNeg.Seconds())
		r.met.QueriesTotal.WithLabelValues("MX", "timeout").Inc()

	default:
		res.Status = "servfail"
		res.TTLSeconds = int32(r.ttlNeg.Seconds())
		r.met.QueriesTotal.WithLabelValues("MX", "error").Inc()
	}
	return res
}

// fallbackA consulta A quando MX está vazio. RFC 5321 §5.1 permite usar A.
func (r *Resolver) fallbackA(ctx context.Context, res *MXResult) {
	start := time.Now()
	defer func() {
		r.met.QueryDuration.WithLabelValues("A").Observe(time.Since(start).Seconds())
	}()

	msg, err := r.client.Query(ctx, res.Domain, dns.TypeA)
	if err != nil {
		r.met.QueriesTotal.WithLabelValues("A", "error").Inc()
		// Se o fallback A falhou por timeout/servfail, NÃO marcamos como
		// "no_mx" (que seria definitivo) — propagamos o erro de DNS.
		if errors.Is(err, ErrTimeout) {
			res.Status = "timeout"
		} else if errors.Is(err, ErrServFail) {
			res.Status = "servfail"
		} else if res.Status == "ok" {
			res.Status = "no_mx"
		}
		return
	}
	for _, ans := range msg.Answer {
		if a, ok := ans.(*dns.A); ok {
			res.A = append(res.A, a.A.String())
		}
	}
	res.HasA = len(res.A) > 0
	if res.HasA {
		// Sintetiza um MX implícito para o caller
		res.MX = []MXRecord{{Host: res.Domain, Priority: 0}}
		res.HasMX = false
		res.Status = "ok"
	} else if res.Status == "ok" {
		res.Status = "no_mx"
	}
	r.met.QueriesTotal.WithLabelValues("A", "ok").Inc()
}

// cacheTTLFor calcula o TTL de cache a partir do status do resultado.
// (chamado apenas em respostas cacheáveis — ver isCacheable).
func (r *Resolver) cacheTTLFor(res *MXResult) time.Duration {
	if res.Status == "nxdomain" {
		return r.ttlNeg
	}
	base := time.Duration(res.TTLSeconds) * time.Second
	return cache.JitteredTTL(base, r.ttlMin, r.ttlMax)
}
