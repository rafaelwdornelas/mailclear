package stages

import (
	"context"

	"github.com/rafaelwdornelas/mailclear/internal/validation"
	"github.com/rafaelwdornelas/mailclear/internal/validation/disposable"
)

// Disposable marca o email como disposable se o domínio estiver na lista.
// Não invalida — apenas seta a flag e adiciona Reason; score decide.
type Disposable struct {
	reg *disposable.Registry
}

// NewDisposable cria a stage.
func NewDisposable(reg *disposable.Registry) *Disposable {
	return &Disposable{reg: reg}
}

// Name devolve o nome da stage.
func (*Disposable) Name() string { return "disposable" }

// ShortCircuit é falso: marca como disposable mas deixa o pipeline seguir.
func (*Disposable) ShortCircuit() bool { return false }

// Run consulta o registry.
func (d *Disposable) Run(_ context.Context, e *validation.Email) error {
	if e.Domain == "" || d.reg == nil {
		return nil
	}
	if d.reg.Contains(e.Domain) {
		e.IsDisposable = true
		e.AddReason("DISPOSABLE", "Domínio descartável conhecido")
	}
	return nil
}
