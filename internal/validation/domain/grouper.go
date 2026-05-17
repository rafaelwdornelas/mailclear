package domain

import (
	"strings"

	"github.com/rafaelwdornelas/mailclear/internal/validation"
)

// GroupByDomain agrupa emails por domínio (lowercase).
// Em modo batch, isso permite fazer 1 lookup DNS por domínio,
// servindo até milhões de emails do mesmo provedor.
func GroupByDomain(emails []*validation.Email) map[string][]*validation.Email {
	groups := make(map[string][]*validation.Email, 256)
	for _, e := range emails {
		d := strings.ToLower(e.Domain)
		if d == "" {
			d = "__no_domain__"
		}
		groups[d] = append(groups[d], e)
	}
	return groups
}
