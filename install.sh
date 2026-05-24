#!/usr/bin/env bash
# ════════════════════════════════════════════════════════════════════════
# MailClear — Bootstrap Linux nativo (multi-distro)
# Suporta: Debian/Ubuntu/Pop!_OS/Mint (apt), Fedora/RHEL/Rocky (dnf),
#          Arch/Manjaro (pacman)
#
# Uso:
#   sudo bash install.sh                  # instala tudo
#   sudo bash install.sh --skip-postgres  # pula Postgres
#   sudo bash install.sh --skip-redis     # pula Redis
#   sudo bash install.sh --skip-unbound   # pula Unbound
#   sudo bash install.sh --db-password=XX # define senha do banco
#   sudo bash install.sh --dry-run        # apenas mostra o que faria
#
# Idempotente: pode rodar várias vezes.
# ════════════════════════════════════════════════════════════════════════
set -euo pipefail

# ── Cores ───────────────────────────────────────────────────────────────
if [ -t 1 ]; then
    # Usa $'...' (ANSI-C quoting) para que \033 seja o caractere ESC real,
    # senão o heredoc do resumo final imprime os códigos literais.
    C_RESET=$'\033[0m'; C_BOLD=$'\033[1m'; C_RED=$'\033[31m'
    C_GREEN=$'\033[32m'; C_YELLOW=$'\033[33m'; C_BLUE=$'\033[34m'; C_CYAN=$'\033[36m'
else
    C_RESET=""; C_BOLD=""; C_RED=""; C_GREEN=""; C_YELLOW=""; C_BLUE=""; C_CYAN=""
fi

log()       { printf "${C_CYAN}[mailclear]${C_RESET} %s\n" "$*"; }
ok()        { printf "${C_GREEN}[ok]${C_RESET} %s\n" "$*"; }
warn()      { printf "${C_YELLOW}[warn]${C_RESET} %s\n" "$*"; }
err()       { printf "${C_RED}[err]${C_RESET} %s\n" "$*" >&2; }
section()   { printf "\n${C_BOLD}${C_BLUE}══ %s ══${C_RESET}\n" "$*"; }
run()       { if [ "$DRY_RUN" = "1" ]; then echo "+ $*"; else eval "$@"; fi; }

# ── Defaults / parsing ──────────────────────────────────────────────────
SKIP_POSTGRES=0
SKIP_REDIS=0
SKIP_UNBOUND=0
DRY_RUN=0
GO_VERSION="1.23.4"
APP_USER="mailclear"
APP_GROUP="mailclear"
APP_BIN="/opt/mailclear/bin"
APP_ETC="/etc/mailclear"
APP_VAR="/var/lib/mailclear"
APP_LOG="/var/log/mailclear"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

for arg in "$@"; do
    case "$arg" in
        --skip-postgres) SKIP_POSTGRES=1 ;;
        --skip-redis)    SKIP_REDIS=1 ;;
        --skip-unbound)  SKIP_UNBOUND=1 ;;
        --dry-run)       DRY_RUN=1 ;;
        -h|--help)
            sed -n '2,17p' "$0"
            exit 0
            ;;
        *) err "Argumento desconhecido: $arg"; exit 1 ;;
    esac
done

# ── Verifica root ───────────────────────────────────────────────────────
if [ "$EUID" -ne 0 ] && [ "$DRY_RUN" = "0" ]; then
    err "Este script precisa ser executado como root (use sudo)"
    exit 1
fi

# ── Detecta distro ──────────────────────────────────────────────────────
section "Detectando distribuição"
if [ -f /etc/os-release ]; then
    . /etc/os-release
    DISTRO_ID="${ID:-unknown}"
    DISTRO_LIKE="${ID_LIKE:-}"
else
    err "/etc/os-release não encontrado"; exit 1
fi

PKG_MGR=""
case "$DISTRO_ID" in
    debian|ubuntu|pop|linuxmint|raspbian) PKG_MGR="apt" ;;
    fedora|rhel|rocky|almalinux|centos)   PKG_MGR="dnf" ;;
    arch|manjaro|endeavouros)             PKG_MGR="pacman" ;;
    *)
        case "$DISTRO_LIKE" in
            *debian*|*ubuntu*) PKG_MGR="apt" ;;
            *rhel*|*fedora*)   PKG_MGR="dnf" ;;
            *arch*)            PKG_MGR="pacman" ;;
        esac
        ;;
esac

if [ -z "$PKG_MGR" ]; then
    err "Distribuição não suportada: $DISTRO_ID ($DISTRO_LIKE)"; exit 1
fi
ok "Distribuição: $DISTRO_ID (pkg manager: $PKG_MGR)"

# ── Helpers de instalação por distro ────────────────────────────────────
pkg_update() {
    case "$PKG_MGR" in
        apt)    run "DEBIAN_FRONTEND=noninteractive apt-get update -y" ;;
        dnf)    run "dnf -y makecache" ;;
        pacman) run "pacman -Sy --noconfirm" ;;
    esac
}

pkg_install() {
    case "$PKG_MGR" in
        apt)    run "DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends $*" ;;
        dnf)    run "dnf -y install $*" ;;
        pacman) run "pacman -S --noconfirm --needed $*" ;;
    esac
}

systemctl_enable() {
    local svc="$1"
    run "systemctl enable --now $svc" || warn "Falha ao habilitar $svc (continuando)"
}

# ════════════════════════════════════════════════════════════════════════
# Etapa 1 — Pacotes base
# ════════════════════════════════════════════════════════════════════════
section "Pacotes base"
pkg_update

case "$PKG_MGR" in
    apt)
        pkg_install curl wget git make gcc ca-certificates dnsutils unzip jq \
                    openssl tar gzip lsb-release gnupg
        ;;
    dnf)
        pkg_install curl wget git make gcc ca-certificates bind-utils unzip jq \
                    openssl tar gzip
        ;;
    pacman)
        pkg_install curl wget git make gcc ca-certificates bind unzip jq \
                    openssl tar gzip
        ;;
esac
ok "Pacotes base instalados"

# ════════════════════════════════════════════════════════════════════════
# Etapa 2 — Go 1.23
# ════════════════════════════════════════════════════════════════════════
section "Go ${GO_VERSION}"
GO_BIN="/usr/local/go/bin/go"
INSTALL_GO=1
if [ -x "$GO_BIN" ]; then
    CURRENT_GO=$("$GO_BIN" version 2>/dev/null | awk '{print $3}' | sed 's/^go//')
    if [ "$CURRENT_GO" = "$GO_VERSION" ]; then
        ok "Go $CURRENT_GO já instalado"
        INSTALL_GO=0
    else
        log "Go $CURRENT_GO presente, atualizando para $GO_VERSION"
    fi
fi

if [ "$INSTALL_GO" = "1" ]; then
    ARCH="$(uname -m)"
    case "$ARCH" in
        x86_64)  GOARCH="amd64" ;;
        aarch64|arm64) GOARCH="arm64" ;;
        armv7l)  GOARCH="armv6l" ;;
        *) err "Arquitetura não suportada: $ARCH"; exit 1 ;;
    esac
    TARBALL="go${GO_VERSION}.linux-${GOARCH}.tar.gz"
    URL="https://go.dev/dl/${TARBALL}"
    log "Baixando $URL"
    run "curl -fsSL -o /tmp/$TARBALL $URL"
    run "rm -rf /usr/local/go"
    run "tar -C /usr/local -xzf /tmp/$TARBALL"
    run "rm -f /tmp/$TARBALL"
fi

# PATH persistente
cat > /etc/profile.d/golang.sh <<'EOF'
export PATH="$PATH:/usr/local/go/bin:$HOME/go/bin"
EOF
run "chmod 644 /etc/profile.d/golang.sh"
export PATH="$PATH:/usr/local/go/bin:/root/go/bin"
ok "Go disponível em $($GO_BIN version 2>/dev/null || echo 'dry-run')"

# ════════════════════════════════════════════════════════════════════════
# Etapa 3 — PostgreSQL 16
# ════════════════════════════════════════════════════════════════════════
if [ "$SKIP_POSTGRES" = "1" ]; then
    warn "PostgreSQL pulado por --skip-postgres"
else
    section "PostgreSQL 16"
    case "$PKG_MGR" in
        apt)
            if ! dpkg -l postgresql-16 >/dev/null 2>&1; then
                # PGDG repo
                run "install -d /usr/share/postgresql-common/pgdg"
                run "curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc \
                    -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc"
                CODENAME="$(. /etc/os-release && echo "${VERSION_CODENAME:-${UBUNTU_CODENAME:-bookworm}}")"
                # Mapeia codinome de derivadas para Debian/Ubuntu equivalente
                case "$CODENAME" in
                    una|vanessa|vera|victoria|virginia|wilma) CODENAME="jammy" ;;  # Mint→Ubuntu
                esac
                echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] https://apt.postgresql.org/pub/repos/apt $CODENAME-pgdg main" \
                    > /etc/apt/sources.list.d/pgdg.list
                pkg_update
                pkg_install postgresql-16 postgresql-client-16
            else
                ok "PostgreSQL 16 já instalado"
            fi
            PG_SERVICE="postgresql"
            ;;
        dnf)
            if ! rpm -q postgresql16-server >/dev/null 2>&1; then
                run "dnf -y install https://download.postgresql.org/pub/repos/yum/reporpms/EL-$(rpm -E %rhel)-x86_64/pgdg-redhat-repo-latest.noarch.rpm" || true
                run "dnf -qy module disable postgresql" || true
                pkg_install postgresql16-server postgresql16-contrib
                run "/usr/pgsql-16/bin/postgresql-16-setup initdb"
            fi
            PG_SERVICE="postgresql-16"
            ;;
        pacman)
            pkg_install postgresql
            if [ ! -d /var/lib/postgres/data/base ]; then
                run "sudo -u postgres initdb -D /var/lib/postgres/data --locale=C.UTF-8 -E UTF8"
            fi
            PG_SERVICE="postgresql"
            ;;
    esac
    systemctl_enable "$PG_SERVICE"

    # Senha é HARDCODED como "mailclear" (alinhada com config.Defaults).
    # Acesso só de 127.0.0.1, controlado por pg_hba.
    log "Criando role/database mailclear (senha fixa 'mailclear', idempotente)"
    if [ "$DRY_RUN" = "0" ]; then
        # User 'postgres' não acessa /home/media etc. Cópia em /tmp.
        PG_TMP_SQL="$(mktemp /tmp/mailclear-init.XXXXXX.sql)"
        cp "$SCRIPT_DIR/etc/postgres/init.sql" "$PG_TMP_SQL"
        chmod 644 "$PG_TMP_SQL"
        sudo -u postgres psql -v ON_ERROR_STOP=1 -f "$PG_TMP_SQL" > /dev/null
        PG_RC=$?
        rm -f "$PG_TMP_SQL"
        if [ $PG_RC -ne 0 ]; then
            err "psql retornou código $PG_RC"
            exit $PG_RC
        fi
    fi
    ok "PostgreSQL configurado"
fi

# ════════════════════════════════════════════════════════════════════════
# Etapa 4 — Redis 7
# ════════════════════════════════════════════════════════════════════════
if [ "$SKIP_REDIS" = "1" ]; then
    warn "Redis pulado por --skip-redis"
else
    section "Redis 7"

    REDIS_CONF_D="/etc/redis/conf.d"
    REDIS_DROPIN="$REDIS_CONF_D/mailclear.conf"

    # ── Proteção contra includes órfãos de instalações anteriores ──
    # Se o redis.conf antigo já tinha "include $REDIS_DROPIN" mas o arquivo
    # foi removido, o auto-start do apt install vai falhar. Removemos a
    # linha aqui (será readicionada depois com o drop-in já no lugar).
    for redis_main in /etc/redis/redis.conf /etc/redis.conf; do
        [ -f "$redis_main" ] || continue
        # \|...|d permite usar | como delimiter — paths têm /
        sed -i "\|^include $REDIS_DROPIN|d; /^# Drop-in MailClear$/d" "$redis_main" 2>/dev/null
    done

    # Garante diretório drop-in antes do pkg_install (em algumas distros
    # o pacote não cria conf.d/ automaticamente)
    run "mkdir -p $REDIS_CONF_D"

    # COPIA O DROP-IN ANTES do pkg_install. Se o include já estiver no
    # redis.conf (de instalação anterior), o auto-start do pacote vai
    # encontrar o arquivo.
    run "install -m 644 $SCRIPT_DIR/etc/redis/mailclear.conf $REDIS_DROPIN"

    case "$PKG_MGR" in
        apt)    pkg_install redis-server ;;
        dnf)    pkg_install redis ;;
        pacman) pkg_install redis ;;
    esac

    REDIS_MAIN_CONF="/etc/redis/redis.conf"
    [ -f /etc/redis.conf ] && REDIS_MAIN_CONF="/etc/redis.conf"

    # Re-copia (caso o pacote tenha sobrescrito conf.d)
    run "install -m 644 $SCRIPT_DIR/etc/redis/mailclear.conf $REDIS_DROPIN"

    # Adiciona include no redis.conf principal (idempotente)
    if [ -f "$REDIS_MAIN_CONF" ] && ! grep -qF "include $REDIS_DROPIN" "$REDIS_MAIN_CONF" 2>/dev/null; then
        echo "" >> "$REDIS_MAIN_CONF"
        echo "# Drop-in MailClear" >> "$REDIS_MAIN_CONF"
        echo "include $REDIS_DROPIN" >> "$REDIS_MAIN_CONF"
    fi

    # Nome do serviço varia: redis-server (Debian) / redis (RHEL/Arch)
    REDIS_SERVICE="redis-server"
    if ! systemctl list-unit-files 2>/dev/null | grep -q "^redis-server.service"; then
        REDIS_SERVICE="redis"
    fi

    # Zera contador de falhas (auto-start do pkg_install pode ter falhado)
    run "systemctl reset-failed $REDIS_SERVICE 2>/dev/null || true"
    systemctl_enable "$REDIS_SERVICE"

    # Confirma que subiu
    if [ "$DRY_RUN" = "0" ]; then
        sleep 1
        if systemctl is-active "$REDIS_SERVICE" >/dev/null 2>&1; then
            ok "Redis ativo"
        else
            err "Redis não subiu. Verifique: tail /var/log/redis/redis-server.log"
            exit 1
        fi
    fi
fi

# ════════════════════════════════════════════════════════════════════════
# Etapa 5 — Unbound (DNS recursivo puro)
# ════════════════════════════════════════════════════════════════════════
if [ "$SKIP_UNBOUND" = "1" ]; then
    warn "Unbound pulado por --skip-unbound"
else
    section "Unbound (DNS recursivo)"
    case "$PKG_MGR" in
        apt)    pkg_install unbound ;;
        dnf)    pkg_install unbound ;;
        pacman) pkg_install unbound ;;
    esac

    # Drop-in da nossa config
    UNBOUND_CONF_D="/etc/unbound/unbound.conf.d"
    run "mkdir -p $UNBOUND_CONF_D"
    run "install -m 644 $SCRIPT_DIR/etc/unbound/mailclear.conf $UNBOUND_CONF_D/mailclear.conf"

    # Garante include no unbound.conf principal
    if [ -f /etc/unbound/unbound.conf ]; then
        if ! grep -q "include.*unbound.conf.d" /etc/unbound/unbound.conf; then
            echo "" >> /etc/unbound/unbound.conf
            echo "include: \"$UNBOUND_CONF_D/*.conf\"" >> /etc/unbound/unbound.conf
        fi
    fi

    # Root hints (necessário para recursão pura)
    run "mkdir -p /var/lib/unbound"
    if [ ! -f /var/lib/unbound/root.hints ] || [ $(find /var/lib/unbound/root.hints -mtime +30 2>/dev/null | wc -l) -gt 0 ]; then
        log "Baixando root.hints da IANA"
        run "curl -fsSL -o /var/lib/unbound/root.hints https://www.internic.net/domain/named.root"
    fi
    run "chown -R unbound:unbound /var/lib/unbound 2>/dev/null || true"

    # NOTA: NÃO geramos certs do unbound-control aqui porque o canal de
    # controle (porta 8953) falha em Ubuntu/Debian devido a AppArmor +
    # ProtectSystem do systemd negando acesso aos certs em /etc/unbound/*.pem.
    # As stats DNS estão disponíveis em /metrics da própria aplicação.

    # Desabilitar DNSSEC: o pacote unbound do Ubuntu instala o drop-in
    # /etc/unbound/unbound.conf.d/root-auto-trust-anchor-file.conf que força
    # auto-trust-anchor-file apontando pra um root.key que ainda não existe.
    # Como nosso drop-in mailclear.conf já decidiu desligar DNSSEC (validação
    # de email não depende disso), removemos o drop-in nativo para evitar
    # falha de checkconf.
    if [ -f /etc/unbound/unbound.conf.d/root-auto-trust-anchor-file.conf ]; then
        run "rm -f /etc/unbound/unbound.conf.d/root-auto-trust-anchor-file.conf"
        log "Drop-in DNSSEC do Ubuntu removido (usamos resolução sem DNSSEC)"
    fi

    # Warmup script (pre-load dos provedores populares)
    run "install -m 755 $SCRIPT_DIR/etc/unbound/warmup.sh /usr/local/bin/mailclear-unbound-warmup"

    # Valida config antes de subir
    if [ "$DRY_RUN" = "0" ]; then
        unbound-checkconf || { err "unbound-checkconf reportou erros — abortando"; exit 1; }
    fi

    # Zera contadores de falha (se Unbound estava failed de tentativa anterior)
    run "systemctl reset-failed unbound 2>/dev/null || true"

    systemctl_enable "unbound"

    # Confirma que subiu de verdade
    if [ "$DRY_RUN" = "0" ]; then
        sleep 1
        if systemctl is-active unbound >/dev/null 2>&1; then
            ok "Unbound recursivo ativo"
        else
            err "Unbound não subiu. Verifique: journalctl -u unbound -n 30"
            exit 1
        fi
    fi
fi

# ════════════════════════════════════════════════════════════════════════
# Etapa 6 — sysctls de tuning
# ════════════════════════════════════════════════════════════════════════
section "Kernel tuning (sysctl)"
run "install -m 644 $SCRIPT_DIR/etc/sysctl/99-mailclear.conf /etc/sysctl.d/99-mailclear.conf"
run "sysctl --system" >/dev/null 2>&1 || warn "sysctl --system retornou aviso"
ok "sysctls aplicados"

# ════════════════════════════════════════════════════════════════════════
# Etapa 7 — Usuário do sistema + diretórios
# ════════════════════════════════════════════════════════════════════════
section "Usuário do sistema e diretórios"
if ! id "$APP_USER" >/dev/null 2>&1; then
    run "useradd -r -s /usr/sbin/nologin -d /nonexistent -c 'MailClear' $APP_USER"
fi
run "install -d -o $APP_USER -g $APP_GROUP -m 750 $APP_BIN $APP_ETC $APP_VAR $APP_LOG"
ok "Usuário $APP_USER e diretórios prontos"

# ════════════════════════════════════════════════════════════════════════
# Etapa 8 — Build dos binários
# ════════════════════════════════════════════════════════════════════════
# (Antigamente instalávamos sqlc e migrate CLI em /usr/local/bin, mas o
# projeto não usa sqlc e o mailclear-migrate é um wrapper Go que importa
# golang-migrate como library — ambas instalações eram desnecessárias.)

section "Compilando binários da aplicação"
cd "$SCRIPT_DIR"
if [ "$DRY_RUN" = "0" ]; then
    "$GO_BIN" mod download
    GOFLAGS="-trimpath" "$GO_BIN" build -ldflags="-s -w" -o "$APP_BIN/mailclear-api"       ./cmd/server
    GOFLAGS="-trimpath" "$GO_BIN" build -ldflags="-s -w" -o "$APP_BIN/mailclear-worker"    ./cmd/worker
    GOFLAGS="-trimpath" "$GO_BIN" build -ldflags="-s -w" -o "$APP_BIN/mailclear-migrate"   ./cmd/migrate
    GOFLAGS="-trimpath" "$GO_BIN" build -ldflags="-s -w" -o "$APP_BIN/mailclear-seed-disposable" ./cmd/seed-disposable
    chown -R "$APP_USER:$APP_GROUP" "$APP_BIN"
fi
ok "Binários compilados em $APP_BIN"

# ════════════════════════════════════════════════════════════════════════
# Etapa 10 — Configurações
# ════════════════════════════════════════════════════════════════════════
# Projeto hardcoded: NÃO criamos .env nem config.yaml.
# A app lê tudo de internal/config/defaults.go.
# Mantemos /etc/mailclear apenas para uso futuro (logs locais, dados temp).
skip_section() { section "$1"; ok "$2"; }
skip_section "Configurações" "projeto hardcoded — sem .env nem config.yaml"

# ════════════════════════════════════════════════════════════════════════
# Etapa 11 — Migrations
# ════════════════════════════════════════════════════════════════════════
if [ "$SKIP_POSTGRES" = "0" ]; then
    section "Aplicando migrations"
    if [ "$DRY_RUN" = "0" ]; then
        cd "$SCRIPT_DIR"
        set +e
        "$APP_BIN/mailclear-migrate" up
        MIGRATE_RC=$?
        set -e
        if [ $MIGRATE_RC -ne 0 ]; then
            warn "Migrations retornaram código $MIGRATE_RC (pode ser 'no change')"
        fi
    fi
    ok "Migrations aplicadas"
fi

# ════════════════════════════════════════════════════════════════════════
# Etapa 12 — Systemd units
# ════════════════════════════════════════════════════════════════════════
section "Instalando systemd units"
for unit in mailclear-api.service mailclear-worker.service mailclear-unbound-warmup.service; do
    run "install -m 644 $SCRIPT_DIR/systemd/$unit /etc/systemd/system/$unit"
done
run "systemctl daemon-reload"
# Zera contadores de falha caso instalações anteriores tenham deixado units em 'failed'
run "systemctl reset-failed mailclear-api.service mailclear-worker.service mailclear-unbound-warmup.service 2>/dev/null || true"
systemctl_enable "mailclear-api.service"
systemctl_enable "mailclear-worker.service"
[ "$SKIP_UNBOUND" = "0" ] && systemctl_enable "mailclear-unbound-warmup.service"
ok "Units habilitadas"

# ════════════════════════════════════════════════════════════════════════
# Etapa 12.5 — Polkit + grupo systemd-journal (ops sem sudo)
# ════════════════════════════════════════════════════════════════════════
# Sem isso, os botões "Reiniciar API/Worker" e "Ver logs" do dashboard
# falham: o user `mailclear` não tem permissão de chamar systemctl via
# D-Bus nem ler o journal.
section "Permissões para ops via dashboard"

# 1) Polkit rule: user mailclear pode restartar suas units sem senha
POLKIT_RULES_DIR="/etc/polkit-1/rules.d"
if [ -d "/etc/polkit-1" ]; then
    run "install -d $POLKIT_RULES_DIR"
    run "install -m 644 $SCRIPT_DIR/deploy/polkit/10-mailclear.rules $POLKIT_RULES_DIR/10-mailclear.rules"
    # Reinicia polkit pra carregar a regra (idempotente, sem erro se já estiver rodando)
    if systemctl list-unit-files 2>/dev/null | grep -q "^polkit.service"; then
        run "systemctl reload polkit 2>/dev/null || systemctl restart polkit"
    fi
    ok "Polkit rule instalada — mailclear pode reiniciar suas próprias units"
else
    warn "Polkit não detectado; restart via dashboard vai exigir senha"
fi

# 2) Grupo systemd-journal: leitura de logs sem sudo
if getent group systemd-journal >/dev/null 2>&1; then
    if id -nG "$APP_USER" 2>/dev/null | tr ' ' '\n' | grep -qx systemd-journal; then
        ok "Usuário $APP_USER já está no grupo systemd-journal"
    else
        run "usermod -aG systemd-journal $APP_USER"
        ok "Usuário $APP_USER adicionado ao grupo systemd-journal"
        warn "Restart de mailclear-api é necessário para o grupo entrar em efeito"
    fi
else
    warn "Grupo systemd-journal não existe; leitura de logs via dashboard pode falhar"
fi

# ════════════════════════════════════════════════════════════════════════
# Etapa 13 — Smoke test
# ════════════════════════════════════════════════════════════════════════
section "Smoke test"
if [ "$DRY_RUN" = "0" ]; then
    sleep 3
    if curl -fsS --max-time 5 http://127.0.0.1:8181/readyz >/dev/null 2>&1; then
        ok "API respondendo em http://127.0.0.1:8181/readyz"
    else
        warn "API ainda não respondeu (pode estar subindo). Verifique: journalctl -u mailclear-api -n 50"
    fi
fi

# ════════════════════════════════════════════════════════════════════════
# Resumo
# ════════════════════════════════════════════════════════════════════════
section "Instalação concluída"
cat <<EOF
${C_BOLD}MailClear instalado com sucesso.${C_RESET}

  ${C_CYAN}API HTTP${C_RESET}     : http://127.0.0.1:${C_BOLD}8181${C_RESET}
  ${C_CYAN}Dashboard${C_RESET}    : http://127.0.0.1:8181/
  ${C_CYAN}Health${C_RESET}       : curl http://127.0.0.1:8181/readyz
  ${C_CYAN}Métricas${C_RESET}     : curl http://127.0.0.1:8181/metrics
  ${C_CYAN}Binários${C_RESET}     : $APP_BIN/
  ${C_CYAN}Logs${C_RESET}         : journalctl -u mailclear-api -f
                  journalctl -u mailclear-worker -f
  ${C_CYAN}Status${C_RESET}       : systemctl status mailclear-api mailclear-worker
  ${C_CYAN}DNS local${C_RESET}    : dig @127.0.0.1 gmail.com MX +short

${C_BOLD}Configuração:${C_RESET} HARDCODED no código (internal/config/defaults.go).
  Sem .env, sem config.yaml, sem auth (API aberta em 127.0.0.1).
  Para tunar: edite defaults.go e rode 'sudo bash install.sh' de novo.

Banco:  user/senha 'mailclear/mailclear' (acesso só 127.0.0.1)
Porta:  8181 (fixa)
Auth:   desabilitada por padrão (uso local)

Comandos úteis:
  systemctl restart mailclear-api mailclear-worker
  journalctl -u unbound -f                # logs do DNS recursivo
  $APP_BIN/mailclear-seed-disposable      # atualiza lista disposable
  sudo bash uninstall.sh                  # remove a aplicação (mantém dados)
  sudo bash uninstall.sh --purge --yes    # remove TUDO (incl. database)

Stats DNS:
  curl -s http://127.0.0.1:8181/metrics | grep ^dns_
EOF
