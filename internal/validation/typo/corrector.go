package typo

import (
	"strings"
	"sync"

	"github.com/rafaelwdornelas/mailclear/assets"
)

// Corrector sugere correção de typos comparando contra uma lista canônica
// de domínios populares. Usa bucketing por primeira letra + comprimento ±2
// para evitar O(N) por lookup.
type Corrector struct {
	once     sync.Once
	domains  []string
	bucket   map[byte][]string // por 1ª letra
}

// NewCorrector cria um corretor já com a lista canônica embarcada.
func NewCorrector() *Corrector {
	c := &Corrector{}
	c.once.Do(c.load)
	return c
}

func (c *Corrector) load() {
	for _, line := range strings.Split(assets.CommonDomains, "\n") {
		line = strings.TrimSpace(strings.ToLower(line))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		c.domains = append(c.domains, line)
	}
	c.bucket = make(map[byte][]string, 32)
	for _, d := range c.domains {
		if len(d) == 0 {
			continue
		}
		b := d[0]
		c.bucket[b] = append(c.bucket[b], d)
	}
}

// Suggest devolve a sugestão de domínio com distância ≤ maxDist
// ou "" se não houver candidato próximo.
// Faz match exato → bucket(1ª letra) → fallback global.
func (c *Corrector) Suggest(domain string, maxDist int) string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return ""
	}
	// Match exato
	for _, d := range c.domains {
		if d == domain {
			return ""
		}
	}

	best := ""
	bestDist := maxDist + 1

	// Bucket por 1ª letra
	if cands, ok := c.bucket[domain[0]]; ok {
		for _, d := range cands {
			if abs(len(d)-len(domain)) > maxDist {
				continue
			}
			dist := DamerauLevenshtein(domain, d)
			if dist < bestDist {
				bestDist = dist
				best = d
			}
		}
	}

	// Fallback global se nada no bucket
	if best == "" {
		for _, d := range c.domains {
			if abs(len(d)-len(domain)) > maxDist {
				continue
			}
			dist := DamerauLevenshtein(domain, d)
			if dist < bestDist {
				bestDist = dist
				best = d
			}
		}
	}

	if bestDist <= maxDist && best != domain {
		return best
	}
	return ""
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
