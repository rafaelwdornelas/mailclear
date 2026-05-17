#!/usr/bin/env bash
# ════════════════════════════════════════════════════════════════════════
# MailClear — Desinstalador robusto
#
# Uso:
#   sudo bash uninstall.sh                 remove só a aplicação (mantém DB, infra)
#   sudo bash uninstall.sh --purge         remove TUDO (database, configs, sysctls,
#                                           drop-ins de redis/unbound, root.key, PATH go)
#   sudo bash uninstall.sh --yes           pula confirmação interativa
#   sudo bash uninstall.sh --purge --yes
#
# Idempotente: pode rodar várias vezes; cada etapa tolera falha.
# ════════════════════════════════════════════════════════════════════════

# Não usamos `set -e` propositalmente: queremos seguir mesmo se algo falhar.
set -u

# ── Cores ──────────────────────────────────────────────────────────────
if [ -t 1 ]; then
    C_RESET=$'\033[0m'; C_BOLD=$'\033[1m'; C_DIM=$'\033[2m'
    C_RED=$'\033[31m'; C_GREEN=$'\033[32m'; C_YELLOW=$'\033[33m'
    C_BLUE=$'\033[34m'; C_CYAN=$'\033[36m'
else
    C_RESET=""; C_BOLD=""; C_DIM=""; C_RED=""; C_GREEN=""; C_YELLOW=""; C_BLUE=""; C_CYAN=""
fi

log()     { printf "${C_CYAN}[mailclear]${C_RESET} %s\n" "$*"; }
ok()      { printf "  ${C_GREEN}✓${C_RESET} %s\n" "$*"; }
skip()    { printf "  ${C_DIM}- %s${C_RESET}\n" "$*"; }
warn()    { printf "  ${C_YELLOW}!${C_RESET} %s\n" "$*"; }
err()     { printf "  ${C_RED}✗${C_RESET} %s\n" "$*" >&2; }
section() { printf "\n${C_BOLD}${C_BLUE}══ %s ══${C_RESET}\n" "$*"; }
hr()      { printf "${C_DIM}%s${C_RESET}\n" "────────────────────────────────────────────────"; }

# try CMD... — executa, ignora erro, devolve OK/skip/warn baseado no exit code.
try() {
    if "$@" >/dev/null 2>&1; then
        ok "$*"
    else
        skip "$* (já feito ou não aplicável)"
    fi
}

# remove_line FILE REGEX — remove linhas correspondentes em FILE (idempotente).
# Usa | como delimiter do sed para não conflitar com / nos paths.
remove_line() {
    local file="$1" regex="$2"
    [ -f "$file" ] || { skip "remove '$regex' em $file (arquivo não existe)"; return; }
    if grep -qE "$regex" "$file" 2>/dev/null; then
        # \|...|d permite usar | como delimiter — essencial pra paths com /
        sed -i.mailclear-bak "\|${regex}|d" "$file" && ok "removida linha '$regex' de $file"
        rm -f "$file.mailclear-bak"
    else
        skip "'$regex' não encontrado em $file"
    fi
}

# ── Args ───────────────────────────────────────────────────────────────
PURGE=0
ASSUME_YES=0
for arg in "$@"; do
    case "$arg" in
        --purge) PURGE=1 ;;
        --yes|-y) ASSUME_YES=1 ;;
        -h|--help) sed -n '2,15p' "$0"; exit 0 ;;
        *) err "argumento desconhecido: $arg"; exit 1 ;;
    esac
done

if [ "$EUID" -ne 0 ]; then
    err "Execute como root (sudo bash uninstall.sh)"
    exit 1
fi

# ── Confirmação ────────────────────────────────────────────────────────
if [ "$ASSUME_YES" -eq 0 ]; then
    echo ""
    if [ "$PURGE" -eq 1 ]; then
        echo "${C_BOLD}${C_RED}MODO PURGE${C_RESET} — vai remover IRREVERSIVELMENTE:"
        echo "  • binários, configs, units e usuário mailclear"
        echo "  • DATABASE mailclear (todos os jobs e resultados)"
        echo "  • drop-ins de redis e unbound + linhas include nos *.conf principais"
        echo "  • /var/lib/unbound/root.key (regerado no próximo install)"
        echo "  • /etc/profile.d/golang.sh"
        echo "  • /etc/sysctl.d/99-mailclear.conf"
    else
        echo "${C_BOLD}MODO PADRÃO${C_RESET} — vai remover:"
        echo "  • binários, configs, units e usuário mailclear"
        echo "  ${C_DIM}(mantém database, drop-ins de redis/unbound, sysctls)${C_RESET}"
    fi
    echo ""
    read -rp "Confirma? [y/N]: " ans
    case "$ans" in
        y|Y|yes|YES|s|S|sim) ;;
        *) echo "Abortado."; exit 0 ;;
    esac
fi

# ════════════════════════════════════════════════════════════════════════
# Etapa 1 — Para e desabilita services do mailclear
# ════════════════════════════════════════════════════════════════════════
section "Parando services do mailclear"
for u in mailclear-api.service mailclear-worker.service mailclear-unbound-warmup.service; do
    if systemctl list-unit-files "$u" 2>/dev/null | grep -q "$u"; then
        systemctl stop "$u" 2>/dev/null && ok "stopped $u" || skip "$u já parado"
        systemctl disable "$u" 2>/dev/null && ok "disabled $u" || skip "$u já desabilitado"
        rm -f "/etc/systemd/system/$u" && ok "unit removido" || skip "unit não existia"
    else
        skip "$u (unit não instalada)"
    fi
done
systemctl daemon-reload 2>/dev/null

# ════════════════════════════════════════════════════════════════════════
# Etapa 2 — Binários e diretórios da aplicação
# ════════════════════════════════════════════════════════════════════════
section "Removendo binários e diretórios"
for path in /opt/mailclear /usr/local/bin/mailclear-unbound-warmup; do
    if [ -e "$path" ]; then
        rm -rf "$path" && ok "removido $path"
    else
        skip "$path não existia"
    fi
done

# ════════════════════════════════════════════════════════════════════════
# Etapa 3 (somente --purge) — limpeza profunda
# ════════════════════════════════════════════════════════════════════════
if [ "$PURGE" -eq 1 ]; then
    section "PURGE: configs e dados da aplicação"
    for path in /etc/mailclear /var/lib/mailclear /var/log/mailclear; do
        if [ -e "$path" ]; then
            rm -rf "$path" && ok "removido $path"
        else
            skip "$path não existia"
        fi
    done

    section "PURGE: sysctls"
    if [ -f /etc/sysctl.d/99-mailclear.conf ]; then
        rm -f /etc/sysctl.d/99-mailclear.conf && ok "removido 99-mailclear.conf"
        sysctl --system >/dev/null 2>&1 && ok "sysctl --system recarregado" \
            || warn "sysctl --system falhou (sem impacto)"
    else
        skip "99-mailclear.conf não existia"
    fi

    section "PURGE: Go PATH global"
    if [ -f /etc/profile.d/golang.sh ]; then
        rm -f /etc/profile.d/golang.sh && ok "removido /etc/profile.d/golang.sh"
    else
        skip "/etc/profile.d/golang.sh não existia"
    fi

    # ── Redis: remove drop-in + linhas include ─────────────────────
    section "PURGE: Redis drop-in"
    REDIS_DROPIN="/etc/redis/conf.d/mailclear.conf"
    if [ -f "$REDIS_DROPIN" ]; then
        rm -f "$REDIS_DROPIN" && ok "removido $REDIS_DROPIN"
    else
        skip "$REDIS_DROPIN não existia"
    fi
    for redis_main in /etc/redis/redis.conf /etc/redis.conf; do
        remove_line "$redis_main" "^include $REDIS_DROPIN\$"
        remove_line "$redis_main" "^# Drop-in MailClear\$"
    done
    # Reinicia se conseguir
    if systemctl list-unit-files redis-server.service 2>/dev/null | grep -q redis-server; then
        systemctl restart redis-server 2>/dev/null && ok "redis-server reiniciado" \
            || warn "redis-server não reiniciou (verifique 'systemctl status redis-server')"
    elif systemctl list-unit-files redis.service 2>/dev/null | grep -q redis; then
        systemctl restart redis 2>/dev/null && ok "redis reiniciado" \
            || warn "redis não reiniciou"
    fi

    # ── Unbound: remove drop-in + linhas include + root.key ─────────
    section "PURGE: Unbound drop-in e root.key"
    UNBOUND_DROPIN="/etc/unbound/unbound.conf.d/mailclear.conf"
    if [ -f "$UNBOUND_DROPIN" ]; then
        rm -f "$UNBOUND_DROPIN" && ok "removido $UNBOUND_DROPIN"
    else
        skip "$UNBOUND_DROPIN não existia"
    fi
    remove_line "/etc/unbound/unbound.conf" "^include: \"/etc/unbound/unbound.conf.d/\\*.conf\"\$"
    # root.key pode estar duplicado/corrompido; remove para o install recriar
    if [ -f /var/lib/unbound/root.key ]; then
        rm -f /var/lib/unbound/root.key && ok "removido root.key"
    else
        skip "root.key não existia"
    fi
    # Reinicia ou recarrega
    systemctl restart unbound 2>/dev/null && ok "unbound reiniciado" \
        || warn "unbound não reiniciou (provavelmente outro erro de config; rode 'sudo unbound-checkconf')"

    # ── Postgres: drop DB e role ───────────────────────────────────
    section "PURGE: Postgres database e role"
    if command -v psql >/dev/null 2>&1; then
        # Drop só funciona se nenhuma conexão estiver ativa
        sudo -u postgres psql -tAc \
            "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='mailclear' AND pid <> pg_backend_pid();" \
            >/dev/null 2>&1 && ok "conexões ao DB mailclear encerradas"

        if sudo -u postgres psql -lqt 2>/dev/null | cut -d'|' -f1 | grep -qw mailclear; then
            sudo -u postgres psql -c "DROP DATABASE IF EXISTS mailclear;" >/dev/null 2>&1 \
                && ok "database mailclear removido" \
                || err "falha ao dropar database (verifique 'sudo -u postgres psql -l')"
        else
            skip "database mailclear não existia"
        fi

        if sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='mailclear'" 2>/dev/null | grep -q 1; then
            sudo -u postgres psql -c "DROP ROLE IF EXISTS mailclear;" >/dev/null 2>&1 \
                && ok "role mailclear removida" \
                || warn "role mailclear ainda existe (talvez tenha objetos próprios)"
        else
            skip "role mailclear não existia"
        fi
    else
        warn "psql não encontrado — pulando drop de DB/role"
    fi
fi

# ════════════════════════════════════════════════════════════════════════
# Etapa 4 — Usuário do sistema (sempre, mesmo sem --purge)
# ════════════════════════════════════════════════════════════════════════
section "Removendo usuário mailclear"
if id mailclear >/dev/null 2>&1; then
    # Mata processos do usuário antes
    pkill -KILL -u mailclear 2>/dev/null
    sleep 0.3
    if userdel mailclear 2>/dev/null; then
        ok "usuário mailclear removido"
    else
        warn "userdel falhou (talvez ainda haja processo do usuário; tente 'sudo userdel -f mailclear')"
    fi
    # Limpa grupo órfão se ainda existir
    groupdel mailclear 2>/dev/null && ok "grupo mailclear removido" || true
else
    skip "usuário mailclear não existia"
fi

# ════════════════════════════════════════════════════════════════════════
# Resumo
# ════════════════════════════════════════════════════════════════════════
section "Resumo"
echo ""
echo "${C_BOLD}Verificação pós-uninstall:${C_RESET}"
hr
printf "  units mailclear-*       : "
if systemctl list-unit-files 2>/dev/null | grep -q '^mailclear-'; then
    echo "${C_RED}ainda existem${C_RESET}"
else
    echo "${C_GREEN}removidas${C_RESET}"
fi
printf "  /opt/mailclear          : "
[ -e /opt/mailclear ] && echo "${C_RED}existe${C_RESET}" || echo "${C_GREEN}removido${C_RESET}"
printf "  /etc/mailclear          : "
[ -e /etc/mailclear ] && echo "${C_YELLOW}existe (mantido)${C_RESET}" || echo "${C_GREEN}removido${C_RESET}"
printf "  usuário mailclear       : "
id mailclear >/dev/null 2>&1 && echo "${C_RED}existe${C_RESET}" || echo "${C_GREEN}removido${C_RESET}"
if [ "$PURGE" -eq 1 ]; then
    printf "  database mailclear      : "
    if sudo -u postgres psql -lqt 2>/dev/null | cut -d'|' -f1 | grep -qw mailclear; then
        echo "${C_RED}ainda existe${C_RESET}"
    else
        echo "${C_GREEN}removido${C_RESET}"
    fi
    printf "  redis drop-in           : "
    [ -e /etc/redis/conf.d/mailclear.conf ] && echo "${C_RED}existe${C_RESET}" || echo "${C_GREEN}removido${C_RESET}"
    printf "  unbound drop-in         : "
    [ -e /etc/unbound/unbound.conf.d/mailclear.conf ] && echo "${C_RED}existe${C_RESET}" || echo "${C_GREEN}removido${C_RESET}"
    printf "  redis-server status     : "
    systemctl is-active redis-server 2>/dev/null || systemctl is-active redis 2>/dev/null || echo "inativo"
    printf "  unbound status          : "
    systemctl is-active unbound 2>/dev/null || echo "inativo"
fi
hr
echo ""

if [ "$PURGE" -eq 1 ]; then
    echo "${C_GREEN}${C_BOLD}Uninstall completo (purge).${C_RESET}"
else
    echo "${C_GREEN}${C_BOLD}Uninstall concluído.${C_RESET} Para remover dados + infra config:"
    echo "  sudo bash uninstall.sh --purge --yes"
fi
echo ""
echo "Para reinstalar:"
echo "  sudo bash install.sh"
echo ""
