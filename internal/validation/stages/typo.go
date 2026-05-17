package stages

import (
	"context"

	"github.com/rafaelwdornelas/mailclear/internal/validation"
	"github.com/rafaelwdornelas/mailclear/internal/validation/typo"
)

// Typo anota uma sugestão de correção quando o domínio é similar (DL ≤ 2)
// a um domínio canônico. Não invalida o email — apenas marca Suggested e
// adiciona um Reason que reduz o score.
type Typo struct {
	corrector *typo.Corrector
	maxDist   int
}

// NewTypo cria a stage com um corretor já carregado.
func NewTypo(corrector *typo.Corrector, maxDist int) *Typo {
	if maxDist <= 0 {
		maxDist = 2
	}
	return &Typo{corrector: corrector, maxDist: maxDist}
}

// Name devolve o nome da stage.
func (*Typo) Name() string { return "typo" }

// ShortCircuit retorna falso: typo só sugere, não invalida.
func (*Typo) ShortCircuit() bool { return false }

// Run executa a sugestão.
func (t *Typo) Run(_ context.Context, e *validation.Email) error {
	if e.Domain == "" || t.corrector == nil {
		return nil
	}
	suggested := t.corrector.Suggest(e.Domain, t.maxDist)
	if suggested != "" {
		e.Suggested = e.LocalPart + "@" + suggested
		e.AddReason("TYPO", "Domínio similar a "+suggested)
	}
	return nil
}
