package stages

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/rafaelwdornelas/mailclear/assets"
	"github.com/rafaelwdornelas/mailclear/internal/validation"
)

// TLD verifica se o sufixo final do domínio está na lista IANA embarcada.
type TLD struct {
	once sync.Once
	set  map[string]struct{}
}

// Name devolve o nome da stage.
func (*TLD) Name() string { return "tld" }

// ShortCircuit aborta o pipeline em falha.
func (*TLD) ShortCircuit() bool { return true }

// Run verifica o TLD.
func (s *TLD) Run(_ context.Context, e *validation.Email) error {
	s.once.Do(s.load)
	if e.Domain == "" {
		return nil
	}
	dot := strings.LastIndexByte(e.Domain, '.')
	if dot < 0 {
		e.AddReason("INVALID_TLD", "Domínio sem TLD")
		return errors.New("sem tld")
	}
	tld := e.Domain[dot+1:]
	if _, ok := s.set[tld]; !ok {
		e.AddReason("INVALID_TLD", "TLD desconhecido: "+tld)
		return errors.New("tld inválido")
	}
	return nil
}

func (s *TLD) load() {
	s.set = make(map[string]struct{}, 1024)
	for _, line := range strings.Split(assets.TLDs, "\n") {
		line = strings.TrimSpace(strings.ToLower(line))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		s.set[line] = struct{}{}
	}
}
