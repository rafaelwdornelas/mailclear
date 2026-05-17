package stages

import (
	"context"
	"errors"

	mdns "github.com/rafaelwdornelas/mailclear/internal/dns"
	"github.com/rafaelwdornelas/mailclear/internal/validation"
	"github.com/rafaelwdornelas/mailclear/internal/validation/domain"
)

// MX consulta o resolver para descobrir registros MX (ou fallback A) do domínio.
// Em NXDomain ou ausência total de MX+A: short-circuit invalid.
// Em ServFail/Timeout: marca risky (sem short-circuit) — o caller pode retry.
type MX struct {
	resolver   *mdns.Resolver
	classifier *domain.Classifier
}

// NewMX cria a stage.
func NewMX(r *mdns.Resolver, c *domain.Classifier) *MX {
	return &MX{resolver: r, classifier: c}
}

// Name devolve o nome da stage.
func (*MX) Name() string { return "mx" }

// ShortCircuit é true: NXDomain ou MX+A vazios encerram o pipeline como invalid.
func (*MX) ShortCircuit() bool { return true }

// Run faz o lookup.
func (s *MX) Run(ctx context.Context, e *validation.Email) error {
	if e.Domain == "" {
		return errors.New("sem domínio")
	}

	res, err := s.resolver.LookupMX(ctx, e.Domain)
	if err != nil {
		switch {
		case errors.Is(err, mdns.ErrNXDomain):
			e.AddReason("NXDOMAIN", "Domínio não existe")
			return errors.New("nxdomain")
		case errors.Is(err, mdns.ErrTimeout):
			e.AddReason("DNS_TIMEOUT", "Timeout consultando DNS")
			e.Status = validation.StatusRisky
			return nil
		case errors.Is(err, mdns.ErrServFail):
			e.AddReason("DNS_SERVFAIL", "Servidor DNS retornou falha")
			e.Status = validation.StatusRisky
			return nil
		case errors.Is(err, mdns.ErrRateLimited):
			e.AddReason("DNS_RATE_LIMITED", "Rate-limit por domínio")
			e.Status = validation.StatusRisky
			return nil
		default:
			e.AddReason("DNS_ERROR", err.Error())
			e.Status = validation.StatusRisky
			return nil
		}
	}

	e.HasMX = res.HasMX
	e.HasA = res.HasA

	if !e.HasMX && !e.HasA {
		e.AddReason("NO_MX", "Domínio sem MX nem A")
		return errors.New("no_mx")
	}

	// Classificação heurística do domínio
	if s.classifier != nil {
		if s.classifier.IsFree(e.Domain) {
			e.IsFree = true
			if e.Classification == "" || e.Classification == validation.ClassificationUnknown {
				e.Classification = validation.ClassificationFree
			}
		} else if e.Classification == "" || e.Classification == validation.ClassificationUnknown {
			c := s.classifier.Classify(e.Domain)
			e.Classification = validation.Classification(c)
		}
	}
	return nil
}
