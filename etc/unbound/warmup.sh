#!/usr/bin/env bash
# ════════════════════════════════════════════════════════════════════════
# MailClear — Pre-load do cache do Unbound com os principais provedores
# de email. Executado pelo systemd unit mailclear-unbound-warmup.service.
# ════════════════════════════════════════════════════════════════════════
set -u

DNS_SERVER="${MAILCLEAR_DNS_SERVER:-127.0.0.1}"
DNS_PORT="${MAILCLEAR_DNS_PORT:-53}"
DIG="$(command -v dig || true)"

if [ -z "$DIG" ]; then
    echo "warmup: dig não encontrado, abortando" >&2
    exit 0
fi

DOMAINS=(
    # ── Provedores globais ──────────────────────────────────────────
    gmail.com googlemail.com outlook.com hotmail.com live.com
    yahoo.com ymail.com rocketmail.com
    icloud.com me.com mac.com
    aol.com protonmail.com proton.me pm.me
    gmx.com gmx.net mail.com
    zoho.com fastmail.com tutanota.com tutamail.com
    yandex.com yandex.ru

    # ── Brasil ──────────────────────────────────────────────────────
    uol.com.br bol.com.br terra.com.br ig.com.br
    globo.com globomail.com
    r7.com itelefonica.com.br
    oi.com.br oi.net.br
    superig.com.br

    # ── Empresariais comuns ─────────────────────────────────────────
    office365.com exchange.microsoft.com
)

QTYPES=(MX A AAAA)

for d in "${DOMAINS[@]}"; do
    for t in "${QTYPES[@]}"; do
        "$DIG" "@${DNS_SERVER}" -p "${DNS_PORT}" +tries=1 +time=3 +short "$d" "$t" \
            > /dev/null 2>&1 || true
    done
done

echo "warmup: pre-load de ${#DOMAINS[@]} domínios concluído"
exit 0
