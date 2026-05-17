// Package disposable mantém uma lista atualizável de domínios descartáveis.
package disposable

import (
	"strings"
	"sync"

	"github.com/rafaelwdornelas/mailclear/assets"
)

// Registry guarda a lista de disposable com swap atômico para reload.
type Registry struct {
	mu  sync.RWMutex
	set map[string]struct{}
}

// New cria um registry vazio e já carrega a lista embedded.
func New() *Registry {
	r := &Registry{set: make(map[string]struct{}, 4096)}
	r.LoadString(assets.DisposableDomains)
	return r
}

// LoadString substitui a lista atual pelo conteúdo (um domínio por linha).
func (r *Registry) LoadString(raw string) {
	newSet := make(map[string]struct{}, len(raw)/10)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(strings.ToLower(line))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		newSet[line] = struct{}{}
	}
	r.mu.Lock()
	r.set = newSet
	r.mu.Unlock()
}

// LoadDomains substitui a lista atual pelos domínios fornecidos.
func (r *Registry) LoadDomains(domains []string) {
	newSet := make(map[string]struct{}, len(domains))
	for _, d := range domains {
		d = strings.TrimSpace(strings.ToLower(d))
		if d != "" {
			newSet[d] = struct{}{}
		}
	}
	r.mu.Lock()
	r.set = newSet
	r.mu.Unlock()
}

// Contains verifica se o domínio é disposable.
func (r *Registry) Contains(domain string) bool {
	d := strings.ToLower(strings.TrimSpace(domain))
	r.mu.RLock()
	_, ok := r.set[d]
	r.mu.RUnlock()
	return ok
}

// Size devolve quantidade atual de domínios.
func (r *Registry) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.set)
}
