package validation

import "context"

// Validator é a interface pública usada pelo resto da aplicação.
// Esconde o pipeline e o score engine atrás de um único Validate.
type Validator interface {
	Validate(ctx context.Context, raw string) (*Email, error)
	ValidateBatch(ctx context.Context, raws []string) ([]*Email, error)
}

// DefaultValidator combina Pipeline + ScoreEngine.
type DefaultValidator struct {
	pipe   *Pipeline
	scorer *ScoreEngine
}

// NewDefaultValidator monta o validator.
func NewDefaultValidator(pipe *Pipeline, scorer *ScoreEngine) *DefaultValidator {
	return &DefaultValidator{pipe: pipe, scorer: scorer}
}

// Validate processa um email único.
func (v *DefaultValidator) Validate(ctx context.Context, raw string) (*Email, error) {
	e := NewEmail(raw)
	if err := v.pipe.Run(ctx, e); err != nil {
		return e, err
	}
	v.scorer.Apply(e)
	if v.pipe.met != nil && e.Status != "" {
		v.pipe.met.ResultStatus.WithLabelValues(string(e.Status)).Inc()
	}
	return e, nil
}

// ValidateBatch processa um lote em sequência.
// Para concorrência real, usar internal/workers.Pool com este validator.
//
// Erros de ctx (cancel/deadline) abortam o batch imediatamente. Erros de
// um email específico ficam refletidos no próprio *Email (Status=unknown
// + AddReason("validation_error", ...)).
func (v *DefaultValidator) ValidateBatch(ctx context.Context, raws []string) ([]*Email, error) {
	out := make([]*Email, 0, len(raws))
	for _, r := range raws {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		e, err := v.Validate(ctx, r)
		if err != nil {
			if ctx.Err() != nil {
				return out, err
			}
			if e != nil {
				e.Status = StatusUnknown
				e.AddReason("validation_error", err.Error())
			}
		}
		out = append(out, e)
	}
	return out, nil
}
