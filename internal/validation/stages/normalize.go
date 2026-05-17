// Package stages contém as etapas concretas do pipeline de validação.
package stages

import (
	"context"
	"errors"
	"strings"

	"golang.org/x/net/idna"
	"golang.org/x/text/unicode/norm"

	"github.com/rafaelwdornelas/mailclear/internal/validation"
)

// Normalize aplica trim, lowercase, NFC, IDNA no domínio.
// Para gmail/googlemail: remove pontos no local-part e tudo após '+' (tagging).
type Normalize struct{}

// Name devolve o nome da stage.
func (Normalize) Name() string { return "normalize" }

// ShortCircuit é falso: normalização não invalida, apenas transforma.
func (Normalize) ShortCircuit() bool { return false }

// Run executa a normalização.
func (Normalize) Run(_ context.Context, e *validation.Email) error {
	raw := strings.TrimSpace(e.Original)
	if raw == "" {
		return errors.New("vazio")
	}
	raw = norm.NFC.String(raw)
	raw = strings.ToLower(raw)

	at := strings.LastIndexByte(raw, '@')
	if at <= 0 || at == len(raw)-1 {
		// formato inválido — passa adiante; syntax stage fará o short-circuit
		e.Normalized = raw
		return nil
	}

	local := raw[:at]
	domain := raw[at+1:]

	// IDNA: domínios internacionais → punycode (xn--)
	if asc, err := idna.Lookup.ToASCII(domain); err == nil {
		domain = asc
	}

	// Gmail/Googlemail: ignora pontos no local e tudo após '+' (alias tagging)
	switch domain {
	case "gmail.com", "googlemail.com":
		if plus := strings.IndexByte(local, '+'); plus >= 0 {
			local = local[:plus]
		}
		local = strings.ReplaceAll(local, ".", "")
		domain = "gmail.com" // consolida ambos
	}

	e.LocalPart = local
	e.Domain = domain
	e.Normalized = local + "@" + domain
	return nil
}
