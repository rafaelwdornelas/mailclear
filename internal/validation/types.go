// Package validation implementa o pipeline de pré-validação de emails.
// O caller cria um *Email, chama Pipeline.Run(ctx, email) e lê o resultado
// pelos campos de Email (Status, Score, Reasons, etc).
package validation

import (
	"strings"
	"time"
)

// Status final do email após pipeline.
type Status string

const (
	StatusValid      Status = "valid"
	StatusInvalid    Status = "invalid"
	StatusRisky      Status = "risky"
	StatusUnknown    Status = "unknown"
	StatusDisposable Status = "disposable"
)

// Classification rotula o domínio.
type Classification string

const (
	ClassificationUnknown     Classification = "unknown"
	ClassificationPersonal    Classification = "personal"
	ClassificationRole        Classification = "role"
	ClassificationFree        Classification = "free"
	ClassificationCorporate   Classification = "corporate"
	ClassificationEducational Classification = "educational"
	ClassificationGovernment  Classification = "government"
)

// Reason justifica uma decisão do pipeline.
type Reason struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// StageOutcome captura a passagem do email por uma stage.
type StageOutcome struct {
	Passed   bool          `json:"passed"`
	Reason   string        `json:"reason,omitempty"`
	Duration time.Duration `json:"-"`
}

// Email é a entidade que atravessa o pipeline.
// Os campos são mutáveis e populados ao longo das stages.
type Email struct {
	// ── Entrada ──────────────────────────────────────────────────────
	Original string `json:"email_original"`

	// ── Normalização ────────────────────────────────────────────────
	Normalized string `json:"email_normalized"`
	LocalPart  string `json:"local_part"`
	Domain     string `json:"domain"`

	// ── Sinais ──────────────────────────────────────────────────────
	HasMX        bool `json:"has_mx"`
	HasA         bool `json:"has_a"`
	IsDisposable bool `json:"is_disposable"`
	IsRole       bool `json:"is_role"`
	IsFree       bool `json:"is_free"`

	// ── Decisão / saída ────────────────────────────────────────────
	Suggested      string         `json:"suggested,omitempty"`
	Status         Status         `json:"status"`
	Score          int            `json:"score"`
	Classification Classification `json:"classification"`
	Reasons        []Reason       `json:"reasons"`

	// ── Debug / observabilidade ────────────────────────────────────
	FailedAt string                  `json:"failed_at,omitempty"`
	Stages   map[string]StageOutcome `json:"-"`
}

// NewEmail inicializa um Email a partir de uma string crua.
func NewEmail(raw string) *Email {
	return &Email{
		Original:       raw,
		Classification: ClassificationUnknown,
		Stages:         make(map[string]StageOutcome, 8),
		Reasons:        make([]Reason, 0, 2),
	}
}

// AddReason registra um motivo (idempotente por code).
func (e *Email) AddReason(code, msg string) {
	for _, r := range e.Reasons {
		if r.Code == code {
			return
		}
	}
	e.Reasons = append(e.Reasons, Reason{Code: code, Message: msg})
}

// DomainLower devolve o domínio em lowercase (helper).
func (e *Email) DomainLower() string { return strings.ToLower(e.Domain) }
