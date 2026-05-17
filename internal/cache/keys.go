package cache

import "strings"

// Convenções de chaves para o cache (sempre usadas com prefix de namespace).
const (
	NSMX           = "dns:mx:"
	NSA            = "dns:a:"
	NSAAAA         = "dns:aaaa:"
	NSDomainStatus = "dom:status:"
)

// MXKey devolve a chave de cache para um lookup MX.
func MXKey(domain string) string {
	return NSMX + strings.ToLower(domain)
}

// AKey devolve a chave de cache para um lookup A.
func AKey(domain string) string {
	return NSA + strings.ToLower(domain)
}

// DomainStatusKey devolve a chave para o status agregado de um domínio.
func DomainStatusKey(domain string) string {
	return NSDomainStatus + strings.ToLower(domain)
}
