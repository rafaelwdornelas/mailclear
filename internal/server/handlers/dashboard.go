package handlers

import "net/http"

// Dashboard serve a página HTML de monitoramento na raiz.
type Dashboard struct{}

// NewDashboard cria o handler.
func NewDashboard() *Dashboard { return &Dashboard{} }

// Index responde GET / com o HTML do dashboard.
// O HTML é uma SPA mínima que polla /admin/status a cada 5s.
func (Dashboard) Index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(dashboardHTML))
}

const dashboardHTML = `<!doctype html>
<html lang="pt-BR">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>MailClear · Dashboard</title>
<style>
* { box-sizing: border-box; margin: 0; padding: 0; }
body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  background: #0e1116; color: #d8dee9; line-height: 1.5;
  padding: 20px; min-height: 100vh;
}
.wrap { max-width: 1280px; margin: 0 auto; }
header {
  display: flex; justify-content: space-between; align-items: center;
  padding-bottom: 16px; border-bottom: 1px solid #2d333b; margin-bottom: 24px;
}
h1 { font-size: 22px; font-weight: 600; color: #f0f6fc; }
h1 .v { color: #7d8590; font-size: 13px; font-weight: 400; margin-left: 8px; }
.tag { padding: 4px 10px; border-radius: 12px; font-size: 12px; font-weight: 500; }
.tag.ok { background: #14361e; color: #56d364; }
.tag.fail { background: #5a1e25; color: #f85149; }
.grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px,1fr)); gap: 16px; margin-bottom: 24px; }
.card {
  background: #161b22; border: 1px solid #2d333b; border-radius: 8px;
  padding: 16px;
}
.card h2 {
  font-size: 11px; text-transform: uppercase; letter-spacing: 0.05em;
  color: #7d8590; margin-bottom: 12px; font-weight: 600;
}
.row { display: flex; justify-content: space-between; padding: 6px 0; border-bottom: 1px solid #21262d; }
.row:last-child { border-bottom: 0; }
.row .l { color: #7d8590; font-size: 13px; }
.row .v { color: #f0f6fc; font-weight: 500; font-variant-numeric: tabular-nums; }

.services { display: flex; gap: 8px; flex-wrap: wrap; }
.svc { padding: 8px 14px; border-radius: 6px; font-size: 12px; font-weight: 600; }
.svc.ok { background: #14361e; color: #56d364; }
.svc.fail { background: #5a1e25; color: #f85149; }

table { width: 100%; border-collapse: collapse; font-size: 13px; }
th, td { text-align: left; padding: 8px 6px; border-bottom: 1px solid #21262d; }
th { color: #7d8590; font-weight: 500; font-size: 11px; text-transform: uppercase; }
td.num { text-align: right; font-variant-numeric: tabular-nums; }
.badge { padding: 2px 8px; border-radius: 10px; font-size: 11px; font-weight: 500; }
.badge.completed { background: #14361e; color: #56d364; }
.badge.running   { background: #1c2d57; color: #58a6ff; }
.badge.pending   { background: #3b2e0c; color: #d29922; }
.badge.failed    { background: #5a1e25; color: #f85149; }
.badge.cancelled { background: #2d333b; color: #7d8590; }

.actions { display: flex; gap: 8px; flex-wrap: wrap; }
.btn {
  background: #21262d; color: #d8dee9; border: 1px solid #2d333b;
  padding: 8px 14px; border-radius: 6px; cursor: pointer; font-size: 13px;
}
.btn:hover { background: #2d333b; }
.btn.danger { color: #f85149; border-color: #5a1e25; }
.btn.danger:hover { background: #5a1e25; color: #fff; }

pre {
  background: #0d1117; border: 1px solid #2d333b; border-radius: 6px;
  padding: 10px; font-size: 12px; color: #d8dee9; overflow-x: auto;
  position: relative; cursor: pointer;
}
pre:hover { border-color: #58a6ff; }
pre::after {
  content: "📋"; position: absolute; top: 6px; right: 8px;
  opacity: 0.5; font-size: 11px;
}
pre.copied::after { content: "✓"; opacity: 1; color: #56d364; }

footer {
  text-align: center; color: #6e7681; font-size: 12px; margin-top: 24px;
  padding-top: 16px; border-top: 1px solid #2d333b;
}
.spinner { display: inline-block; width: 12px; height: 12px; border: 2px solid #2d333b;
  border-top-color: #58a6ff; border-radius: 50%; animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>
</head>
<body>
<div class="wrap">
  <header>
    <h1>MailClear <span class="v" id="version">—</span></h1>
    <div>
      <span id="overall" class="tag ok">conectando…</span>
      <span style="color:#7d8590;font-size:12px;margin-left:12px" id="updated">—</span>
    </div>
  </header>

  <div class="grid">
    <div class="card">
      <h2>Serviços</h2>
      <div class="services" id="services">—</div>
    </div>

    <div class="card">
      <h2>Runtime</h2>
      <div class="row"><span class="l">Uptime</span><span class="v" id="uptime">—</span></div>
      <div class="row"><span class="l">Goroutines</span><span class="v" id="goroutines">—</span></div>
      <div class="row"><span class="l">Memória alocada</span><span class="v" id="mem">—</span></div>
      <div class="row"><span class="l">Go version</span><span class="v" id="goversion">—</span></div>
    </div>

    <div class="card">
      <h2>Cache</h2>
      <div class="row"><span class="l">Chaves Redis</span><span class="v" id="redis-keys">—</span></div>
      <div class="row"><span class="l">Memória Redis</span><span class="v" id="redis-mem">—</span></div>
      <div class="row"><span class="l">Disposable mem</span><span class="v" id="disp-mem">—</span></div>
      <div class="row"><span class="l">Disposable DB</span><span class="v" id="disp-db">—</span></div>
    </div>

    <div class="card">
      <h2>Estatísticas globais</h2>
      <div class="row"><span class="l">Total processado</span><span class="v" id="total">—</span></div>
      <div class="row"><span class="l">Válidos</span><span class="v" id="s-valid" style="color:#56d364">—</span></div>
      <div class="row"><span class="l">Risky</span><span class="v" id="s-risky" style="color:#d29922">—</span></div>
      <div class="row"><span class="l">Inválidos</span><span class="v" id="s-invalid" style="color:#f85149">—</span></div>
      <div class="row"><span class="l">Disposable</span><span class="v" id="s-disposable" style="color:#a371f7">—</span></div>
      <div class="row"><span class="l">Domínios únicos</span><span class="v" id="s-domains">—</span></div>
    </div>
  </div>

  <div class="card" style="margin-bottom:24px">
    <h2>Jobs recentes</h2>
    <table>
      <thead>
        <tr><th>Status</th><th>Nome</th><th class="num">Progresso</th><th class="num">✓</th><th class="num">⚠</th><th class="num">✗</th><th class="num">♻</th><th>Criado</th></tr>
      </thead>
      <tbody id="jobs"><tr><td colspan="8" style="color:#7d8590">carregando…</td></tr></tbody>
    </table>
  </div>

  <div class="grid">
    <div class="card">
      <h2>Ações da aplicação</h2>
      <div class="actions">
        <button class="btn" onclick="doAction('/admin/cache/clear', 'POST')">Limpar cache (LRU + Redis)</button>
        <button class="btn" onclick="doAction('/admin/disposable/reload', 'POST')">Recarregar disposable</button>
        <button class="btn danger" onclick="confirmAction('Cancelar TODOS os jobs em execução?', '/admin/jobs/cancel-all', 'POST')">Cancelar todos jobs</button>
      </div>
      <p style="color:#7d8590;font-size:12px;margin-top:12px">
        Ações executam diretamente — API sem autenticação.
      </p>
    </div>

    <div class="card">
      <h2>Limpar logs (requer sudo no terminal)</h2>
      <pre onclick="copyPre(this)">sudo journalctl --vacuum-time=7d</pre>
      <pre onclick="copyPre(this)">sudo journalctl --vacuum-size=500M</pre>
      <pre onclick="copyPre(this)">sudo journalctl --rotate &amp;&amp; sudo journalctl --vacuum-time=1s</pre>
      <p style="color:#7d8590;font-size:12px;margin-top:8px">
        Clique no comando para copiar.
      </p>
    </div>

    <div class="card">
      <h2>Operação (terminal)</h2>
      <pre onclick="copyPre(this)">sudo systemctl restart mailclear-api mailclear-worker</pre>
      <pre onclick="copyPre(this)">sudo journalctl -u mailclear-api -f</pre>
      <pre onclick="copyPre(this)">sudo unbound-control stats_noreset | grep cache</pre>
      <pre onclick="copyPre(this)">redis-cli --scan --pattern "mailclear:*" | head -20</pre>
    </div>

    <div class="card">
      <h2>Links rápidos</h2>
      <div class="actions">
        <a class="btn" href="/metrics" target="_blank">Prometheus /metrics</a>
        <a class="btn" href="/readyz" target="_blank">/readyz</a>
        <a class="btn" href="/api/v1/stats" target="_blank">/api/v1/stats</a>
      </div>
    </div>
  </div>

  <footer>
    MailClear · auto-refresh 5s · <a href="https://github.com/rafaelwdornelas/mailclear" style="color:#58a6ff">github</a>
  </footer>
</div>

<script>
const $ = (id) => document.getElementById(id);

function flash(msg) {
  const el = $('updated');
  const old = el.textContent;
  el.textContent = msg;
  setTimeout(() => { el.textContent = old; }, 1500);
}

function fmtNumber(n) {
  if (n == null) return '—';
  return n.toLocaleString('pt-BR');
}

function fmtUptime(s) {
  if (!s) return '—';
  const d = Math.floor(s / 86400);
  const h = Math.floor((s % 86400) / 3600);
  const m = Math.floor((s % 3600) / 60);
  if (d > 0) return d + 'd ' + h + 'h';
  if (h > 0) return h + 'h ' + m + 'm';
  return m + 'm';
}

function fmtTime(iso) {
  if (!iso) return '—';
  try { return new Date(iso).toLocaleString('pt-BR'); } catch (e) { return iso; }
}

function copyPre(el) {
  navigator.clipboard.writeText(el.textContent).then(() => {
    el.classList.add('copied');
    setTimeout(() => el.classList.remove('copied'), 1200);
  });
}

async function doAction(path, method) {
  try {
    const r = await fetch(path, { method });
    const txt = await r.text();
    if (r.ok) { flash('OK: ' + txt.substring(0, 80)); refresh(); }
    else { flash('Erro ' + r.status + ': ' + txt); }
  } catch (e) {
    flash('Falha: ' + e.message);
  }
}

function confirmAction(msg, path, method) {
  if (confirm(msg)) doAction(path, method);
}

async function refresh() {
  try {
    const r = await fetch('/admin/status');
    if (!r.ok) {
      $('overall').textContent = 'erro ' + r.status;
      $('overall').className = 'tag fail';
      return;
    }
    const d = await r.json();
    render(d);
  } catch (e) {
    $('overall').textContent = 'offline';
    $('overall').className = 'tag fail';
  }
}

function render(d) {
  // Header
  $('version').textContent = 'v' + (d.version || 'dev');
  $('updated').textContent = 'atualizado ' + new Date().toLocaleTimeString('pt-BR');

  // Overall status
  const allOk = Object.values(d.services || {}).every(v => v === 'ok');
  $('overall').textContent = allOk ? '✓ tudo ok' : '✗ degradado';
  $('overall').className = 'tag ' + (allOk ? 'ok' : 'fail');

  // Services
  $('services').innerHTML = Object.entries(d.services || {})
    .map(([k, v]) => '<span class="svc ' + v + '">' + k + ' ' + (v === 'ok' ? '✓' : '✗') + '</span>')
    .join('');

  // Runtime
  $('uptime').textContent = fmtUptime(d.uptime_seconds);
  $('goroutines').textContent = fmtNumber(d.runtime?.goroutines);
  $('mem').textContent = (d.runtime?.mem_alloc_mb || 0) + ' MB / ' + (d.runtime?.mem_sys_mb || 0) + ' MB';
  $('goversion').textContent = d.runtime?.go_version || '—';

  // Cache
  $('redis-keys').textContent = fmtNumber(d.cache?.redis_keys);
  $('redis-mem').textContent = d.cache?.redis_memory_human || '—';
  $('disp-mem').textContent = fmtNumber(d.disposable?.in_memory_size);
  $('disp-db').textContent = fmtNumber(d.disposable?.db_size);

  // Stats
  const s = d.stats || {};
  $('total').textContent = fmtNumber(s.total);
  $('s-valid').textContent = fmtNumber(s.valid);
  $('s-risky').textContent = fmtNumber(s.risky);
  $('s-invalid').textContent = fmtNumber(s.invalid);
  $('s-disposable').textContent = fmtNumber(s.disposable);
  $('s-domains').textContent = fmtNumber(s.unique_domains);

  // Jobs
  const jobs = d.jobs || [];
  if (jobs.length === 0) {
    $('jobs').innerHTML = '<tr><td colspan="8" style="color:#7d8590">nenhum job ainda</td></tr>';
  } else {
    $('jobs').innerHTML = jobs.map(j => {
      const pct = j.total > 0 ? Math.floor(100 * j.processed / j.total) : 0;
      return '<tr>' +
        '<td><span class="badge ' + j.status + '">' + j.status + '</span></td>' +
        '<td style="max-width:240px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" title="' + (j.name || '') + '">' + (j.name || '—') + '</td>' +
        '<td class="num">' + fmtNumber(j.processed) + ' / ' + fmtNumber(j.total) + ' (' + pct + '%)</td>' +
        '<td class="num" style="color:#56d364">' + fmtNumber(j.valid) + '</td>' +
        '<td class="num" style="color:#d29922">' + fmtNumber(j.risky) + '</td>' +
        '<td class="num" style="color:#f85149">' + fmtNumber(j.invalid) + '</td>' +
        '<td class="num" style="color:#a371f7">' + fmtNumber(j.disposable) + '</td>' +
        '<td style="color:#7d8590">' + fmtTime(j.created_at) + '</td>' +
        '</tr>';
    }).join('');
  }
}

// Boot
refresh();
setInterval(refresh, 5000);
</script>
</body>
</html>
`
