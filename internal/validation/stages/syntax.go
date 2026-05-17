package stages

import (
	"context"
	"errors"
	"net/mail"
	"regexp"
	"strings"

	"github.com/rafaelwdornelas/mailclear/internal/validation"
)

// Regex fast-path. Não pretende ser estritamente RFC 5322 — pega 99% dos casos
// rapidamente; o restante cai no mail.ParseAddress.
var emailFastPath = regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,63}$`)

// Limites canônicos (RFC 5321 §4.5.3.1).
const (
	maxLocal   = 64
	maxDomain  = 253
	maxOverall = 320
)

// Syntax aplica regras RFC + tamanhos. Faz short-circuit.
type Syntax struct{}

// Name devolve o nome da stage.
func (Syntax) Name() string { return "syntax" }

// ShortCircuit aborta o pipeline em falha.
func (Syntax) ShortCircuit() bool { return true }

// Run valida sintaxe e tamanho.
func (Syntax) Run(_ context.Context, e *validation.Email) error {
	s := e.Normalized
	if s == "" {
		s = strings.TrimSpace(e.Original)
	}
	if len(s) == 0 {
		e.AddReason("EMPTY", "Email vazio")
		return errors.New("vazio")
	}
	if len(s) > maxOverall {
		e.AddReason("TOO_LONG", "Email excede 320 caracteres")
		return errors.New("muito longo")
	}

	// Fast-path
	if emailFastPath.MatchString(s) {
		return checkParts(e, s)
	}

	// Fallback robusto
	addr, err := mail.ParseAddress(s)
	if err != nil {
		e.AddReason("INVALID_SYNTAX", "Formato de email inválido")
		return errors.New("syntax")
	}
	e.Normalized = strings.ToLower(addr.Address)
	return checkParts(e, e.Normalized)
}

func checkParts(e *validation.Email, addr string) error {
	at := strings.LastIndexByte(addr, '@')
	if at <= 0 || at == len(addr)-1 {
		e.AddReason("INVALID_SYNTAX", "Falta local-part ou domínio")
		return errors.New("syntax")
	}
	local := addr[:at]
	domain := addr[at+1:]

	if len(local) > maxLocal {
		e.AddReason("LOCAL_TOO_LONG", "Local-part excede 64 caracteres")
		return errors.New("local-part muito longo")
	}
	if len(domain) > maxDomain {
		e.AddReason("DOMAIN_TOO_LONG", "Domínio excede 253 caracteres")
		return errors.New("domain muito longo")
	}
	if !strings.Contains(domain, ".") {
		e.AddReason("DOMAIN_NO_DOT", "Domínio sem ponto")
		return errors.New("domain sem ponto")
	}
	if strings.HasPrefix(local, ".") || strings.HasSuffix(local, ".") || strings.Contains(local, "..") {
		e.AddReason("INVALID_SYNTAX", "Pontos consecutivos ou nas pontas do local-part")
		return errors.New("syntax")
	}
	// Populamos campos derivados se não vieram da normalize
	if e.LocalPart == "" {
		e.LocalPart = local
	}
	if e.Domain == "" {
		e.Domain = domain
	}
	return nil
}
