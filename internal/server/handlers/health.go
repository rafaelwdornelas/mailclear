package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/miekg/dns"
	"github.com/redis/go-redis/v9"
)

// Health agrega checks para readiness probe.
type Health struct {
	DB        *pgxpool.Pool
	Redis     *redis.Client
	DNSServer string
}

// NewHealth cria os handlers.
func NewHealth(db *pgxpool.Pool, rdb *redis.Client, dnsServer string) *Health {
	return &Health{DB: db, Redis: rdb, DNSServer: dnsServer}
}

// Live é um liveness probe simples (sempre 200 se o processo está rodando).
func (h *Health) Live(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready verifica DB, Redis e DNS — readiness probe.
func (h *Health) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	out := map[string]string{}
	status := http.StatusOK

	if h.DB != nil {
		if err := h.DB.Ping(ctx); err != nil {
			out["db"] = "fail: " + err.Error()
			status = http.StatusServiceUnavailable
		} else {
			out["db"] = "ok"
		}
	}
	if h.Redis != nil {
		if err := h.Redis.Ping(ctx).Err(); err != nil {
			out["redis"] = "fail: " + err.Error()
			status = http.StatusServiceUnavailable
		} else {
			out["redis"] = "ok"
		}
	}
	if h.DNSServer != "" {
		if err := pingDNS(ctx, h.DNSServer); err != nil {
			out["dns"] = "fail: " + err.Error()
			status = http.StatusServiceUnavailable
		} else {
			out["dns"] = "ok"
		}
	}
	WriteJSON(w, status, out)
}

func pingDNS(ctx context.Context, server string) error {
	c := &dns.Client{Net: "udp", Timeout: 2 * time.Second}
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("."), dns.TypeNS)
	_, _, err := c.ExchangeContext(ctx, m, server)
	return err
}
