#!/usr/bin/env bash
# ════════════════════════════════════════════════════════════════════════
# Smoke test + benchmark básico contra a API local (sem auth)
# ════════════════════════════════════════════════════════════════════════
set -e

API="${MAILCLEAR_API:-http://127.0.0.1:8181}"

echo "── /healthz ──────────────────────────────────────────"
curl -fsS "$API/healthz" && echo

echo "── /readyz ───────────────────────────────────────────"
curl -fsS "$API/readyz" && echo

echo "── Validar 1 email ───────────────────────────────────"
curl -fsS -X POST "$API/api/v1/validate" \
    -H "Content-Type: application/json" \
    -d '{"email":"test@gmial.com"}' | jq . || true
echo

echo "── Validar batch (5) ─────────────────────────────────"
curl -fsS -X POST "$API/api/v1/validate/batch" \
    -H "Content-Type: application/json" \
    -d '{"emails":["a@gmail.com","b@yahooo.com","admin@example.org","x@dontexistadnsabc.com","fake@yopmail.com"]}' \
    | jq . || true
echo

echo "── DNS metrics ───────────────────────────────────────"
curl -fsS "$API/metrics" | grep -E '^(dns_|validation_|http_)' | head -40
