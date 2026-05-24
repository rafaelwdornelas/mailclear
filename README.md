# MailClear

Pré-validação e limpeza de listas massivas de emails em Go. Roda local, classifica milhões de endereços antes do envio para APIs pagas (Ninja, ZeroBounce, NeverBounce) e elimina sintaxe inválida, typos, disposable e domínios sem MX — economizando o equivalente em chamadas pagas.

Instalação nativa Linux (sem Docker) via systemd. Stack: Go 1.23, PostgreSQL 16, Redis 7, Unbound DNS recursivo. Atinge ~1.500 emails/s sustentado por servidor com cache aquecido.

## Por que existe

Listas reais têm 30-70% de entradas inválidas: typos (`gmial.com`), disposable (`yopmail.com`), domínios mortos, role accounts (`noreply@`). Pagar API externa para validar essa parte é desperdício — todas essas categorias podem ser detectadas localmente sem rede externa ou com 1 lookup DNS por domínio.

MailClear pré-filtra a lista antes da API paga, devolvendo só os endereços que valem o custo de uma validação SMTP profunda.

## Como funciona

```
CSV/TXT upload → split em chunks → fila de workers → pipeline → cache + DNS → DB
                                                        │
                                                        ▼
                                            normalize → syntax → tld
                                            → typo → disposable → role
                                            → MX → score → classify
```

Componentes principais:

- **Pipeline de validação** ([internal/validation/stages](internal/validation/stages)): 7 estágios sequenciais por email. Cada estágio pode short-circuit (NXDOMAIN encerra na hora) ou só anotar (typo correction não muda fluxo).
- **DNS recursivo próprio** ([internal/dns](internal/dns)): Unbound local em `127.0.0.1:53`, sem depender de Google/Cloudflare. Cliente customizado com [miekg/dns](https://github.com/miekg/dns), `singleflight`, retry com backoff.
- **Cache em camadas** ([internal/cache](internal/cache)): L1 LRU in-memory + L2 Redis com jitter de TTL.
- **AIMD adaptive concurrency** ([internal/dns/adaptive.go](internal/dns/adaptive.go)): controlador TCP-style que ajusta paralelismo DNS baseado em taxa de timeout. Sobe aditivo (+64/tick) quando erro < 1%, corta multiplicativo (/2) quando > 5%, fast-decrease (/4) em colapso (> 20%).
- **Domain grouping**: 100k emails de `gmail.com` resultam em **1 único** MX lookup (singleflight + cache).
- **Workers concorrentes** ([internal/workers](internal/workers)) com backpressure no upload (HTTP 503 quando fila ≥ 90%).

## Performance

Servidor de teste: AMD Ryzen 9 7900 (24 threads), 60GB RAM, Ubuntu 24.04.

| Métrica | Valor |
|---|---|
| Rate por job (cache aquecido) | ~1.000 emails/s |
| Rate combinado (2 chunks paralelos) | ~1.500-1.800 emails/s |
| Cache hit ratio em re-runs | ~87% |
| Insert no Postgres (batch 5000) | ~90ms |
| Lista de 3M emails (estimado) | ~30 min |

O sistema é instrumentado com Prometheus — todas as métricas em `/metrics`. Pontos chave para monitorar:

- `dns_inflight_limit` — AIMD em ação (vê o limite subir/cair)
- `dns_query_total{result="timeout"}` — saturação real
- `job_batch_phase_duration_seconds{phase}` — gargalo por fase (pre_warm / validate / insert)
- `workers_queue_depth` — backpressure

## Stack

| Camada | Tecnologia |
|---|---|
| Linguagem | Go 1.23 |
| HTTP | go-chi/chi v5 |
| Banco | PostgreSQL 16 + pgx/v5 + golang-migrate |
| Cache | Redis 7 + hashicorp/golang-lru/v2 |
| DNS | Unbound 1.19+ (recursivo) + miekg/dns + singleflight |
| Logging | rs/zerolog |
| Métricas | prometheus/client_golang |
| Init system | systemd nativo |

## Instalação

Suportado: Debian/Ubuntu/Pop!_OS/Mint (apt), Fedora/RHEL/Rocky (dnf), Arch/Manjaro (pacman).

```bash
git clone https://github.com/rafaelwdornelas/mailclear.git
cd mailclear
sudo bash install.sh
```

O script (idempotente, pode rodar várias vezes):

1. Detecta distro e instala Go 1.23, PostgreSQL 16, Redis 7, Unbound.
2. Configura Unbound como recursivo puro (sem forwarder), com cache 1GB e prefetch.
3. Aplica sysctls de performance (rmem_max, somaxconn, file-max).
4. Cria usuário `mailclear` e diretórios `/opt/mailclear/`, `/etc/mailclear/`, `/var/lib/mailclear/`.
5. Compila os 4 binários para `/opt/mailclear/bin/`.
6. Roda migrations.
7. Instala e habilita `mailclear-api.service`, `mailclear-worker.service`, `mailclear-unbound-warmup.service`.
8. Instala regra polkit ([deploy/polkit/10-mailclear.rules](deploy/polkit/10-mailclear.rules)) que permite ao user `mailclear` reiniciar suas próprias units sem senha, e adiciona o user ao grupo `systemd-journal` para o dashboard ler logs sem sudo.
9. Smoke test em `localhost:8181/readyz`.

Toda configuração é hardcoded em [internal/config/defaults.go](internal/config/defaults.go) — sem `.env`, sem YAML. Para ajustar workers, batch size, timeouts: edita o arquivo e roda `sudo bash install.sh` de novo.

### Desinstalação

```bash
sudo bash uninstall.sh                  # remove app (mantém Postgres/Redis/Unbound)
sudo bash uninstall.sh --purge --yes    # remove tudo incluindo DB
```

## Uso

### Dashboard

`http://<servidor>:8181/` — SPA HTML que polla `/admin/status` a cada 5s. Mostra:

- Badges de serviços (api/db/dns/redis) inline no header.
- Cards de runtime (uptime, goroutines, memória), cache, throughput (queue depth, AIMD limit, jobs em execução) e estatísticas globais.
- Card destacado em amarelo **"Jobs travados"** (running > 5min sem progresso) com botão para marcar como `failed`.
- Tabela de jobs recentes com botão **"Re-enfileirar"** em jobs falhados.
- Botões para reiniciar `mailclear-api` ou `mailclear-worker` **direto pelo navegador** (via D-Bus + polkit, sem sudo).
- Modal de logs com tail ao vivo via Server-Sent Events (sem precisar SSH no servidor).

### Validar um email

```bash
curl -X POST http://localhost:8181/api/v1/validate \
  -H "Content-Type: application/json" \
  -d '{"email":"contato@gmial.com"}'
```

```json
{
  "email_original": "contato@gmial.com",
  "email_normalized": "contato@gmial.com",
  "domain": "gmial.com",
  "has_mx": false,
  "is_disposable": true,
  "suggested": "contato@gmail.com",
  "status": "risky",
  "score": 0,
  "classification": "unknown",
  "reasons": [
    {"code": "TYPO", "message": "Domínio similar a gmail.com"},
    {"code": "DISPOSABLE", "message": "Domínio descartável conhecido"}
  ]
}
```

### Importar lista CSV/TXT

```bash
curl -X POST -F file=@lista.csv http://localhost:8181/api/v1/import/csv
# → {"job_id": "abc...", "status": "queued", "total": 50000}

curl http://localhost:8181/api/v1/jobs/abc...
# → {"status":"running","processed":12000,"valid":234,...}

curl http://localhost:8181/api/v1/jobs/abc.../export.csv -o resultado.csv
```

### CLI portable (cliente)

Para processar listas gigantes (milhões de emails) sem precisar rodar `curl` na mão, existe um CLI separado: [mailclear-cli](https://github.com/rafaelwdornelas/mailclear-cli). Ele quebra a lista em chunks de 50k, envia em paralelo, faz checkpoint resilientee separa os resultados por status (`lista_valid.txt`, `lista_risky.txt`, `lista_invalid.txt`, `lista_disposable.txt`).

### Operação

A maior parte das operações comuns está exposta no dashboard sem precisar SSH:
reiniciar serviços, ler logs ao vivo, destravar/re-enfileirar jobs, limpar cache,
recarregar lista de disposable. A linha de comando continua disponível pra
quando o próprio dashboard estiver fora:

```bash
# Status dos serviços
systemctl status mailclear-api mailclear-worker unbound postgresql redis-server

# Logs ao vivo
journalctl -u mailclear-api -f
journalctl -u mailclear-worker -f

# Métricas Prometheus
curl -s http://localhost:8181/metrics | grep '^dns_\|^job_\|^workers_'

# Acompanhar AIMD adaptativo
journalctl -u mailclear-api -f | grep aimd
```

## Endpoints

| Método | Path | Descrição |
|---|---|---|
| GET  | `/` | Dashboard HTML |
| POST | `/api/v1/validate` | Validação síncrona de 1 email |
| POST | `/api/v1/validate/batch` | Validação síncrona em lote |
| POST | `/api/v1/import/csv` | Upload de CSV/TXT → cria job assíncrono |
| GET  | `/api/v1/jobs` | Lista paginada de jobs |
| GET  | `/api/v1/jobs/{id}` | Status detalhado |
| GET  | `/api/v1/jobs/{id}/results` | Resultados (paginação cursor) |
| GET  | `/api/v1/jobs/{id}/export.csv` | Stream CSV completo |
| POST | `/api/v1/jobs/{id}/cancel` | Cancela job em execução |
| POST | `/api/v1/jobs/{id}/requeue` | Cria job novo com os mesmos emails do original |
| GET  | `/api/v1/stats` | Estatísticas globais |
| GET  | `/admin/status` | JSON com estado do sistema (serviços, throughput, jobs travados) |
| POST | `/admin/cache/clear` | Limpa LRU L1 + keys Redis com prefixo `mailclear:` |
| POST | `/admin/disposable/reload` | Recarrega lista de disposable do banco pra memória |
| POST | `/admin/jobs/cancel-all` | Cancela em batch todos os jobs em pending/running/paused |
| POST | `/admin/jobs/{id}/force-fail` | Marca job como `failed` sem esperar o timeout natural (24h) |
| POST | `/admin/services/restart?service=api\|worker` | Restart de serviço via D-Bus (sem sudo, requer polkit rule) |
| GET  | `/admin/logs?service=api\|worker&lines=N` | Últimas N linhas do journal (texto plano) |
| GET  | `/admin/logs/stream?service=api\|worker` | Tail -f ao vivo do journal via Server-Sent Events |
| GET  | `/healthz` | Liveness probe |
| GET  | `/readyz` | Readiness (DB + Redis + DNS) |
| GET  | `/metrics` | Prometheus exposition |

Sem autenticação por padrão — uso esperado é local (127.0.0.1) ou atrás de firewall/proxy.
Spec OpenAPI completa em [api/openapi.yaml](api/openapi.yaml).

## Score Engine

Cada email recebe um score conforme passa pelo pipeline:

| Sinal | Delta |
|---|---|
| Sintaxe RFC válida | +30 |
| TLD reconhecido (IANA) | +10 |
| MX presente | +35 |
| Apenas A (fallback RFC 5321 §5.1) | +15 |
| Disposable conhecido | −60 |
| Role account (`noreply@`, `admin@`, ...) | −15 |
| Typo detectado (com sugestão) | −10 |
| Catch-all heurística | −20 |
| DNS timeout/servfail | sinaliza `risky` |

Classificação:

- **≥ 80** → `valid`
- **40-79** → `risky` (vale enviar pra API paga)
- **< 40** → `invalid` (descarta)

Tabela de pesos em [internal/validation/scoring.go](internal/validation/scoring.go).

## Desenvolvimento

```bash
make build              # compila ./bin/{mailclear-api,worker,migrate,seed-disposable}
make run-api            # roda API em foreground
make test-race          # testes com -race
make migrate-up         # aplica migrations
make seed-disposable    # atualiza lista de disposable do GitHub
```

Estrutura de pastas:

```
mailclear/
├── cmd/                 # 4 binários (server, worker, migrate, seed-disposable)
├── internal/
│   ├── app/             # bootstrap + lifecycle
│   ├── config/          # defaults hardcoded
│   ├── dns/             # client + cache + AIMD + singleflight + rate-limit
│   ├── cache/           # L1 (LRU) + L2 (Redis) com cascade
│   ├── validation/      # pipeline + stages + score engine
│   ├── jobs/            # manager + handler (pre-warm + validate + persist)
│   ├── workers/         # pool com backpressure
│   ├── storage/         # repositórios pgx
│   ├── server/          # HTTP (chi) + handlers + middlewares
│   └── metrics/         # registry Prometheus
├── migrations/          # golang-migrate (.up/.down)
├── queries/             # SQL puro (caso queira regerar via sqlc)
├── assets/              # listas embarcadas (disposable, roles, TLDs)
├── api/openapi.yaml     # spec da API
├── etc/                 # configs de /etc copiadas pelo install.sh
├── systemd/             # units .service
├── deploy/polkit/       # regra polkit pra restart sem sudo
└── scripts/             # bench, seed wrappers
```

## Licença

MIT.
