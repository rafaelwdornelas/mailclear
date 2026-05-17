#!/usr/bin/env bash
# Wrapper para rodar o binário de seed da lista de disposable
set -e
cd "$(dirname "${BASH_SOURCE[0]}")/.."
if [ -f /etc/mailclear/.env ]; then
    set -a; . /etc/mailclear/.env; set +a
elif [ -f .env ]; then
    set -a; . ./.env; set +a
fi
BIN="${MAILCLEAR_BIN:-/opt/mailclear/bin/mailclear-seed-disposable}"
[ -x "$BIN" ] || BIN="./bin/mailclear-seed-disposable"
exec "$BIN" "$@"
