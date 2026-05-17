package stages

import (
	"context"

	"github.com/rafaelwdornelas/mailclear/internal/validation"
	"github.com/rafaelwdornelas/mailclear/internal/validation/role"
)

// Role marca o email como role-account (admin@, support@, ...).
// Não invalida; apenas seta flag e Reason.
type Role struct {
	det *role.Detector
}

// NewRole cria a stage.
func NewRole(det *role.Detector) *Role { return &Role{det: det} }

// Name devolve o nome da stage.
func (*Role) Name() string { return "role" }

// ShortCircuit é falso: role não invalida.
func (*Role) ShortCircuit() bool { return false }

// Run consulta o detector.
func (r *Role) Run(_ context.Context, e *validation.Email) error {
	if e.LocalPart == "" || r.det == nil {
		return nil
	}
	if r.det.IsRole(e.LocalPart) {
		e.IsRole = true
		e.Classification = validation.ClassificationRole
		e.AddReason("ROLE", "Conta genérica (role): "+e.LocalPart)
	}
	return nil
}
