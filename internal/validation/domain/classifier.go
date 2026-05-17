// Package domain agrega utilitários sobre o domínio (split, agrupamento, classificação).
package domain

import (
	"strings"
	"sync"

	"github.com/rafaelwdornelas/mailclear/assets"
)

// Classifier rotula o domínio como free|corporate|personal etc.
type Classifier struct {
	once sync.Once
	free map[string]struct{}
}

// NewClassifier cria o classifier com a lista embedded.
func NewClassifier() *Classifier {
	c := &Classifier{}
	c.once.Do(c.load)
	return c
}

func (c *Classifier) load() {
	c.free = make(map[string]struct{}, 64)
	for _, line := range strings.Split(assets.FreeProviders, "\n") {
		line = strings.TrimSpace(strings.ToLower(line))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		c.free[line] = struct{}{}
	}
}

// IsFree devolve true se o domínio for um provedor gratuito conhecido.
func (c *Classifier) IsFree(domain string) bool {
	_, ok := c.free[strings.ToLower(domain)]
	return ok
}

// Classify devolve uma classificação heurística.
func (c *Classifier) Classify(domain string) string {
	d := strings.ToLower(domain)
	if c.IsFree(d) {
		return "free"
	}
	switch {
	case strings.HasSuffix(d, ".edu") || strings.HasSuffix(d, ".edu.br") || strings.Contains(d, ".ac."):
		return "educational"
	case strings.HasSuffix(d, ".gov") || strings.HasSuffix(d, ".gov.br") || strings.HasSuffix(d, ".gob.") || strings.HasSuffix(d, ".mil"):
		return "government"
	default:
		return "corporate"
	}
}
