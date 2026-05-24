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
.card.warn { border-color: #d29922; }
.row { display: flex; justify-content: space-between; padding: 6px 0; border-bottom: 1px solid #21262d; }
.row:last-child { border-bottom: 0; }
.row .l { color: #7d8590; font-size: 13px; }
.row .v { color: #f0f6fc; font-weight: 500; font-variant-numeric: tabular-nums; }

.bar { background: #0d1117; border-radius: 4px; height: 6px; overflow: hidden; margin-top: 4px; }
.bar > span { display: block; height: 100%; background: #58a6ff; }
.bar > span.high { background: #d29922; }
.bar > span.critical { background: #f85149; }

.header-services { display: flex; gap: 6px; flex-wrap: wrap; }
.svc { padding: 3px 10px; border-radius: 12px; font-size: 11px; font-weight: 600; }
.svc.ok { background: #14361e; color: #56d364; }
.svc.fail { background: #5a1e25; color: #f85149; }
header .right { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }

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
.btn.warn { color: #d29922; border-color: #3b2e0c; }
.btn.warn:hover { background: #3b2e0c; color: #fff; }
.btn.small { padding: 4px 10px; font-size: 11px; }

footer {
  text-align: center; color: #6e7681; font-size: 12px; margin-top: 24px;
  padding-top: 16px; border-top: 1px solid #2d333b;
}

/* Modal de logs */
.modal-bg {
  position: fixed; inset: 0; background: rgba(0,0,0,0.75); z-index: 100;
  display: none; align-items: center; justify-content: center;
}
.modal-bg.open { display: flex; }
.modal {
  background: #0d1117; border: 1px solid #2d333b; border-radius: 8px;
  width: 92%; max-width: 1100px; height: 80vh; display: flex; flex-direction: column;
}
.modal-head {
  display: flex; justify-content: space-between; align-items: center;
  padding: 12px 16px; border-bottom: 1px solid #2d333b;
}
.modal-head h3 { font-size: 14px; color: #f0f6fc; }
.modal-head .x { cursor: pointer; color: #7d8590; font-size: 18px; padding: 0 8px; }
.modal-head .x:hover { color: #f85149; }
.modal-body {
  flex: 1; overflow: auto; padding: 12px 16px; background: #010409;
  font-family: ui-monospace, "SF Mono", Menlo, Consolas, monospace;
  font-size: 11px; color: #c9d1d9; white-space: pre-wrap; word-break: break-all;
}

#flash { position: fixed; bottom: 20px; right: 20px; padding: 10px 16px;
  background: #161b22; border: 1px solid #2d333b; border-radius: 6px;
  font-size: 13px; color: #f0f6fc; display: none; z-index: 200; max-width: 400px; }
#flash.ok { border-color: #14361e; }
#flash.err { border-color: #5a1e25; }
</style>
</head>
<body>
<div class="wrap">
  <header>
    <h1>MailClear <span class="v" id="version">—</span></h1>
    <div class="right">
      <div class="header-services" id="services">—</div>
      <span id="overall" class="tag ok">conectando…</span>
      <span style="color:#7d8590;font-size:12px" id="updated">—</span>
    </div>
  </header>

  <div class="grid">
    <div class="card">
      <h2>Runtime</h2>
      <div class="row"><span class="l">Uptime</span><span class="v" id="uptime">—</span></div>
      <div class="row"><span class="l">Goroutines</span><span class="v" id="goroutines">—</span></div>
      <div class="row"><span class="l">Memória alocada</span><span class="v" id="mem">—</span></div>
      <div class="row"><span class="l">Go version</span><span class="v" id="goversion">—</span></div>
    </div>

    <div class="card">
      <h2>Cache</h2>
      <div class="row"><span class="l">Memória Redis</span><span class="v" id="redis-mem">—</span></div>
      <div class="row"><span class="l">Disposable em memória</span><span class="v" id="disp-mem">—</span></div>
    </div>

    <div class="card">
      <h2>Throughput</h2>
      <div class="row">
        <span class="l">Fila de batches</span>
        <span class="v" id="queue">—</span>
      </div>
      <div class="bar"><span id="queue-bar" style="width:0%"></span></div>
      <div class="row"><span class="l">DNS in-flight</span><span class="v" id="dns-inflight">—</span></div>
      <div class="row"><span class="l">DNS limite (AIMD)</span><span class="v" id="dns-limit">—</span></div>
      <div class="row"><span class="l">Jobs em execução</span><span class="v" id="jobs-running">—</span></div>
    </div>

    <div class="card">
      <h2>Estatísticas globais</h2>
      <div class="row"><span class="l">Total processado</span><span class="v" id="total">—</span></div>
      <div class="row"><span class="l">Válidos</span><span class="v" id="s-valid" style="color:#56d364">—</span></div>
      <div class="row"><span class="l">Risky</span><span class="v" id="s-risky" style="color:#d29922">—</span></div>
      <div class="row"><span class="l">Inválidos</span><span class="v" id="s-invalid" style="color:#f85149">—</span></div>
      <div class="row"><span class="l">Disposable</span><span class="v" id="s-disposable" style="color:#a371f7">—</span></div>
    </div>
  </div>

  <div class="card warn" id="stuck-card" style="margin-bottom:24px;display:none">
    <h2 style="color:#d29922">⚠ Jobs travados (running > 5min sem progresso)</h2>
    <table>
      <thead>
        <tr><th>Nome</th><th class="num">Progresso</th><th>Última atualização</th><th></th></tr>
      </thead>
      <tbody id="stuck"></tbody>
    </table>
  </div>

  <div class="card" style="margin-bottom:24px">
    <h2>Jobs recentes</h2>
    <table>
      <thead>
        <tr><th>Status</th><th>Nome</th><th class="num">Progresso</th><th class="num">✓</th><th class="num">⚠</th><th class="num">✗</th><th class="num">♻</th><th>Criado</th><th></th></tr>
      </thead>
      <tbody id="jobs"><tr><td colspan="9" style="color:#7d8590">carregando…</td></tr></tbody>
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
    </div>

    <div class="card">
      <h2>Operação dos serviços</h2>
      <div class="actions">
        <button class="btn" onclick="openLogs('api')">Ver logs API</button>
        <button class="btn" onclick="openLogs('worker')">Ver logs Worker</button>
        <button class="btn warn" onclick="confirmAction('Reiniciar mailclear-api? Vai derrubar o servidor por alguns segundos.', '/admin/services/restart?service=api', 'POST')">Reiniciar API</button>
        <button class="btn warn" onclick="confirmAction('Reiniciar mailclear-worker?', '/admin/services/restart?service=worker', 'POST')">Reiniciar Worker</button>
      </div>
      <p style="color:#7d8590;font-size:12px;margin-top:12px">
        Restart usa polkit (sem sudo) — ver deploy/polkit/10-mailclear.rules.
      </p>
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

<!-- Modal de logs -->
<div class="modal-bg" id="log-modal-bg">
  <div class="modal">
    <div class="modal-head">
      <h3 id="log-title">Logs</h3>
      <span class="x" onclick="closeLogs()">✕</span>
    </div>
    <div class="modal-body" id="log-body">carregando…</div>
  </div>
</div>

<div id="flash"></div>

<script>
const $ = (id) => document.getElementById(id);
let logStream = null;

function flash(msg, kind) {
  const el = $('flash');
  el.textContent = msg;
  el.className = kind || '';
  el.style.display = 'block';
  setTimeout(() => { el.style.display = 'none'; }, 2500);
}

function fmtNumber(n) { return n == null ? '—' : n.toLocaleString('pt-BR'); }
function fmtUptime(s) {
  if (!s) return '—';
  const d = Math.floor(s/86400), h = Math.floor((s%86400)/3600), m = Math.floor((s%3600)/60);
  if (d > 0) return d + 'd ' + h + 'h';
  if (h > 0) return h + 'h ' + m + 'm';
  return m + 'm';
}
function fmtTime(iso) { if (!iso) return '—'; try { return new Date(iso).toLocaleString('pt-BR'); } catch (e) { return iso; } }
function fmtPercent(f) { if (f == null) return '—'; return Math.round(f * 100) + '%'; }

async function doAction(path, method) {
  try {
    const r = await fetch(path, { method });
    const txt = await r.text();
    if (r.ok) { flash('OK: ' + txt.substring(0, 100), 'ok'); refresh(); }
    else { flash('Erro ' + r.status + ': ' + txt.substring(0, 200), 'err'); }
  } catch (e) { flash('Falha: ' + e.message, 'err'); }
}

function confirmAction(msg, path, method) { if (confirm(msg)) doAction(path, method); }

async function forceFail(id, name) {
  if (!confirm('Marcar job "' + name + '" como failed?')) return;
  await doAction('/admin/jobs/' + id + '/force-fail', 'POST');
}

async function requeue(id, name) {
  if (!confirm('Re-enfileirar job "' + name + '"? Vai criar um job novo com os mesmos emails.')) return;
  await doAction('/api/v1/jobs/' + id + '/requeue', 'POST');
}

function openLogs(service) {
  $('log-title').textContent = 'Logs · mailclear-' + service;
  $('log-body').textContent = 'conectando…';
  $('log-modal-bg').classList.add('open');
  closeLogStream();
  // Stream via SSE
  logStream = new EventSource('/admin/logs/stream?service=' + service);
  let buf = '';
  logStream.onmessage = (ev) => {
    buf += ev.data + '\n';
    if (buf.length > 200000) buf = buf.substring(buf.length - 150000);
    $('log-body').textContent = buf;
    $('log-body').scrollTop = $('log-body').scrollHeight;
  };
  logStream.onerror = () => { $('log-body').textContent = buf + '\n[stream encerrado]'; closeLogStream(); };
}
function closeLogs() { $('log-modal-bg').classList.remove('open'); closeLogStream(); }
function closeLogStream() { if (logStream) { logStream.close(); logStream = null; } }

async function refresh() {
  try {
    const r = await fetch('/admin/status');
    if (!r.ok) { $('overall').textContent = 'erro ' + r.status; $('overall').className = 'tag fail'; return; }
    const d = await r.json();
    render(d);
  } catch (e) { $('overall').textContent = 'offline'; $('overall').className = 'tag fail'; }
}

function render(d) {
  $('version').textContent = 'v' + (d.version || 'dev');
  $('updated').textContent = 'atualizado ' + new Date().toLocaleTimeString('pt-BR');

  const allOk = Object.values(d.services || {}).every(v => v === 'ok');
  $('overall').textContent = allOk ? '✓ tudo ok' : '✗ degradado';
  $('overall').className = 'tag ' + (allOk ? 'ok' : 'fail');

  $('services').innerHTML = Object.entries(d.services || {})
    .map(([k, v]) => '<span class="svc ' + v + '">' + k + ' ' + (v === 'ok' ? '✓' : '✗') + '</span>').join('');

  $('uptime').textContent = fmtUptime(d.uptime_seconds);
  $('goroutines').textContent = fmtNumber(d.runtime?.goroutines);
  $('mem').textContent = (d.runtime?.mem_alloc_mb || 0) + ' MB / ' + (d.runtime?.mem_sys_mb || 0) + ' MB';
  $('goversion').textContent = d.runtime?.go_version || '—';

  $('redis-mem').textContent = d.cache?.redis_memory_human || '—';
  $('disp-mem').textContent = fmtNumber(d.disposable?.in_memory_size);

  // Throughput
  const t = d.throughput || {};
  const usage = t.queue_usage || 0;
  $('queue').textContent = fmtNumber(t.queue_len) + ' / ' + fmtNumber(t.queue_cap) + ' (' + fmtPercent(usage) + ')';
  const bar = $('queue-bar');
  bar.style.width = Math.min(100, usage * 100) + '%';
  bar.className = usage >= 0.9 ? 'critical' : (usage >= 0.6 ? 'high' : '');
  $('dns-inflight').textContent = fmtNumber(t.dns_inflight);
  $('dns-limit').textContent = fmtNumber(t.dns_limit);
  $('jobs-running').textContent = fmtNumber(t.jobs_running);

  const s = d.stats || {};
  $('total').textContent = fmtNumber(s.total);
  $('s-valid').textContent = fmtNumber(s.valid);
  $('s-risky').textContent = fmtNumber(s.risky);
  $('s-invalid').textContent = fmtNumber(s.invalid);
  $('s-disposable').textContent = fmtNumber(s.disposable);

  // Stuck jobs
  const stuck = d.stuck_jobs || [];
  if (stuck.length === 0) {
    $('stuck-card').style.display = 'none';
  } else {
    $('stuck-card').style.display = 'block';
    $('stuck').innerHTML = stuck.map(j => {
      const pct = j.total > 0 ? Math.floor(100 * j.processed / j.total) : 0;
      return '<tr>' +
        '<td>' + escapeHtml(j.name || '—') + '</td>' +
        '<td class="num">' + fmtNumber(j.processed) + ' / ' + fmtNumber(j.total) + ' (' + pct + '%)</td>' +
        '<td style="color:#7d8590">' + fmtTime(j.updated_at) + '</td>' +
        '<td><button class="btn small warn" onclick="forceFail(\'' + j.id + '\', \'' + escapeAttr(j.name) + '\')">Marcar failed</button></td>' +
        '</tr>';
    }).join('');
  }

  // Recent jobs
  const jobs = d.jobs || [];
  if (jobs.length === 0) {
    $('jobs').innerHTML = '<tr><td colspan="9" style="color:#7d8590">nenhum job ainda</td></tr>';
  } else {
    $('jobs').innerHTML = jobs.map(j => {
      const pct = j.total > 0 ? Math.floor(100 * j.processed / j.total) : 0;
      const actionBtn = j.status === 'failed'
        ? '<button class="btn small" onclick="requeue(\'' + j.id + '\', \'' + escapeAttr(j.name) + '\')">Re-enfileirar</button>'
        : '';
      return '<tr>' +
        '<td><span class="badge ' + j.status + '">' + j.status + '</span></td>' +
        '<td style="max-width:240px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" title="' + escapeAttr(j.name || '') + '">' + escapeHtml(j.name || '—') + '</td>' +
        '<td class="num">' + fmtNumber(j.processed) + ' / ' + fmtNumber(j.total) + ' (' + pct + '%)</td>' +
        '<td class="num" style="color:#56d364">' + fmtNumber(j.valid) + '</td>' +
        '<td class="num" style="color:#d29922">' + fmtNumber(j.risky) + '</td>' +
        '<td class="num" style="color:#f85149">' + fmtNumber(j.invalid) + '</td>' +
        '<td class="num" style="color:#a371f7">' + fmtNumber(j.disposable) + '</td>' +
        '<td style="color:#7d8590">' + fmtTime(j.created_at) + '</td>' +
        '<td>' + actionBtn + '</td>' +
        '</tr>';
    }).join('');
  }
}

function escapeHtml(s) { return String(s).replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'})[c]); }
function escapeAttr(s) { return String(s).replace(/['"\\]/g, '\\$&'); }

// ESC fecha o modal
document.addEventListener('keydown', (e) => { if (e.key === 'Escape') closeLogs(); });

refresh();
setInterval(refresh, 5000);
</script>
</body>
</html>
`
