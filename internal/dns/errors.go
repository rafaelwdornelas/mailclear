// Package dns implementa um resolver DNS customizado usando miekg/dns
// apontando para o Unbound local (127.0.0.1:53). Suporta UDP+TCP fallback,
// EDNS, retries, cache (L1+L2), singleflight e rate-limit por domínio.
package dns

import "errors"

// Erros canônicos do resolver. Os callers devem usar errors.Is para
// classificar o tipo de falha.
var (
	// ErrNXDomain indica que o domínio não existe (RCODE 3).
	ErrNXDomain = errors.New("nxdomain")
	// ErrServFail indica falha do servidor autoritativo (RCODE 2).
	ErrServFail = errors.New("servfail")
	// ErrRefused indica recusa do servidor (RCODE 5).
	ErrRefused = errors.New("refused")
	// ErrTimeout indica timeout (UDP+TCP esgotados).
	ErrTimeout = errors.New("dns timeout")
	// ErrNoRecords indica que a query foi bem-sucedida mas não retornou registros.
	ErrNoRecords = errors.New("no records")
	// ErrRateLimited indica que o rate-limit por domínio foi excedido.
	ErrRateLimited = errors.New("rate limited")
)
