package importer

import (
	"strings"
	"sync"
)

// Dedup é um set thread-safe simples para deduplicar emails dentro de
// um job. Para milhões de entradas, é mais eficiente que reabrir o DB
// para checar UNIQUE — usamos sync.Map para concorrência.
type Dedup struct {
	m sync.Map
}

// NewDedup cria um dedup vazio.
func NewDedup() *Dedup { return &Dedup{} }

// SeenOrAdd devolve true se o email já foi visto. Caso contrário, adiciona.
func (d *Dedup) SeenOrAdd(email string) bool {
	key := strings.ToLower(strings.TrimSpace(email))
	if key == "" {
		return true
	}
	_, loaded := d.m.LoadOrStore(key, struct{}{})
	return loaded
}
