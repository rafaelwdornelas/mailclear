// Package role detecta local-parts genéricos (admin, info, no-reply, ...).
package role

import (
	"strings"
	"sync"

	"github.com/rafaelwdornelas/mailclear/assets"
)

// Detector mantém um set de prefixos role.
type Detector struct {
	mu  sync.RWMutex
	set map[string]struct{}
}

// New cria um detector com a lista embedded.
func New() *Detector {
	d := &Detector{set: make(map[string]struct{}, 128)}
	d.LoadString(assets.RolePrefixes)
	return d
}

// LoadString substitui a lista atual.
func (d *Detector) LoadString(raw string) {
	newSet := make(map[string]struct{}, 128)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(strings.ToLower(line))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		newSet[line] = struct{}{}
	}
	d.mu.Lock()
	d.set = newSet
	d.mu.Unlock()
}

// LoadList substitui a lista atual pelos prefixos fornecidos.
func (d *Detector) LoadList(prefixes []string) {
	newSet := make(map[string]struct{}, len(prefixes))
	for _, p := range prefixes {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			newSet[p] = struct{}{}
		}
	}
	d.mu.Lock()
	d.set = newSet
	d.mu.Unlock()
}

// IsRole verifica se o local-part é um prefixo de role.
func (d *Detector) IsRole(local string) bool {
	l := strings.ToLower(strings.TrimSpace(local))
	d.mu.RLock()
	_, ok := d.set[l]
	d.mu.RUnlock()
	return ok
}
