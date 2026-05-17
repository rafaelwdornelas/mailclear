package dns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/miekg/dns"
)

// Client é um cliente DNS de baixo nível que fala UDP→TCP com EDNS, retries,
// e timeout por query. Aponta sempre para o Unbound local (127.0.0.1:53).
type Client struct {
	udp        *dns.Client
	tcp        *dns.Client
	upstream   string        // ex: "127.0.0.1:53"
	timeout    time.Duration // por tentativa
	retries    int           // tentativas UDP antes de desistir
	ednsBuffer uint16        // payload EDNS (4096 default)
}

// NewClient cria um cliente apontando para upstream.
func NewClient(upstream string, timeout time.Duration, retries int, ednsBuffer uint16) *Client {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	if retries < 0 {
		retries = 0
	}
	if ednsBuffer == 0 {
		ednsBuffer = 4096
	}
	// Resolver custom: o miekg/dns aceita "host:port" diretamente.
	return &Client{
		udp:        &dns.Client{Net: "udp", Timeout: timeout, UDPSize: ednsBuffer},
		tcp:        &dns.Client{Net: "tcp", Timeout: timeout},
		upstream:   upstream,
		timeout:    timeout,
		retries:    retries,
		ednsBuffer: ednsBuffer,
	}
}

// Query envia uma query DNS para upstream e devolve a resposta.
// Faz fallback UDP→TCP se a resposta vier truncada e tenta retries em erro de UDP.
func (c *Client) Query(ctx context.Context, name string, qtype uint16) (*dns.Msg, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), qtype)
	m.SetEdns0(c.ednsBuffer, false)
	m.RecursionDesired = true

	var (
		resp   *dns.Msg
		lastErr error
	)

	// 1) Retries em UDP
	for i := 0; i <= c.retries; i++ {
		ctxQ, cancel := context.WithTimeout(ctx, c.timeout)
		resp, _, lastErr = c.udp.ExchangeContext(ctxQ, m, c.upstream)
		cancel()
		if lastErr == nil {
			break
		}
		// Erros temporários (timeout, conn refused transitório) — tenta de novo
		if !isRetryable(lastErr) {
			break
		}
	}

	// 2) Fallback TCP se truncado OR erro persistente
	if (resp != nil && resp.Truncated) || (lastErr != nil && isRetryable(lastErr)) {
		ctxT, cancel := context.WithTimeout(ctx, c.timeout)
		r2, _, err := c.tcp.ExchangeContext(ctxT, m, c.upstream)
		cancel()
		if err == nil {
			resp = r2
			lastErr = nil
		} else if lastErr == nil {
			lastErr = err
		}
	}

	if lastErr != nil {
		return nil, classifyNetErr(lastErr)
	}
	if resp == nil {
		return nil, ErrTimeout
	}

	// Classifica RCODE
	switch resp.Rcode {
	case dns.RcodeSuccess:
		return resp, nil
	case dns.RcodeNameError:
		return nil, ErrNXDomain
	case dns.RcodeServerFailure:
		return nil, ErrServFail
	case dns.RcodeRefused:
		return nil, ErrRefused
	default:
		return nil, fmt.Errorf("dns rcode: %d", resp.Rcode)
	}
}

// Upstream devolve o endereço configurado (útil para healthchecks).
func (c *Client) Upstream() string { return c.upstream }

// classifyNetErr converte erros do miekg/net para os erros canônicos.
func classifyNetErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return ErrTimeout
	}
	return err
}

// isRetryable indica se vale a pena tentar novamente após esse erro.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && (ne.Timeout() || ne.Temporary()) { // nolint:staticcheck — Temporary ainda informativo
		return true
	}
	return true // network/UDP é volátil; em geral vale a pena
}
