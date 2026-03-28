package common

const DashboardHTML = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>RunFabric — %s</title>
  <style>
    * { box-sizing: border-box; }
    body { font-family: system-ui, -apple-system, sans-serif; max-width: 720px; margin: 2rem auto; padding: 0 1rem; background: #f8f9fa; color: #1a1a1a; }
    .card { background: #fff; border-radius: 10px; box-shadow: 0 1px 3px rgba(0,0,0,.08); padding: 1.25rem; margin-bottom: 1rem; }
    .card-title { font-size: 0.75rem; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; color: #6b7280; margin: 0 0 0.5rem 0; }
    h1 { font-size: 1.35rem; margin: 0 0 0.25rem 0; }
    .meta { color: #6b7280; font-size: 0.875rem; margin: 0.25rem 0; }
    .outputs { background: #f5f5f5; padding: 0.75rem; border-radius: 6px; margin-top: 0.5rem; }
    .outputs dt { font-weight: 600; margin-top: 0.5rem; font-size: 0.875rem; }
    .outputs dd { margin-left: 0; word-break: break-all; font-size: 0.875rem; }
    .stages { margin: 0; }
    .stages a { display: inline-block; margin: 0.25rem 0.25rem 0 0; padding: 0.35rem 0.65rem; background: #f1f5f9; color: #0f172a; border-radius: 6px; text-decoration: none; font-size: 0.875rem; }
    .stages a:hover { background: #e2e8f0; }
    .none { color: #94a3b8; font-size: 0.875rem; }
    .actions { display: flex; gap: 0.5rem; flex-wrap: wrap; }
    .actions button { padding: 0.5rem 1rem; cursor: pointer; border: 1px solid #e2e8f0; border-radius: 6px; background: #fff; font-size: 0.875rem; }
    .actions button:hover { background: #f1f5f9; }
    .action-result-placeholder { margin-top: 0.75rem; padding: 0.75rem; border-radius: 6px; font-size: 0.875rem; color: #9ca3af; border: 1px dashed #e5e7eb; background: #fafafa; display: flex; align-items: center; gap: 0.75rem; }
    .action-loader-spinner { width: 20px; height: 20px; border: 2px solid #e5e7eb; border-top-color: #6366f1; border-radius: 50%%; animation: action-spin 0.7s linear infinite; flex-shrink: 0; }
    @keyframes action-spin { to { transform: rotate(360deg); } }
    .actions button:disabled { opacity: 0.6; cursor: not-allowed; }
    .action-result-card .card-title { margin-bottom: 0.25rem; }
    .action-result-card .result-summary { font-size: 0.875rem; margin: 0.25rem 0 0.5rem 0; }
    .action-result-card.ok .result-summary { color: #065f46; }
    .action-result-card.err .result-summary { color: #991b1b; }
    .action-result-card .result-json { display: none; margin-top: 0.75rem; padding: 1rem; background: #1e293b; color: #e2e8f0; border-radius: 8px; overflow: auto; font-size: 0.8125rem; font-family: ui-monospace, monospace; white-space: pre-wrap; word-break: break-all; max-height: 360px; }
    .action-result-card .result-json.visible { display: block; }
    .json-toggle { font-size: 0.8125rem; color: #6366f1; cursor: pointer; text-decoration: none; margin-top: 0.5rem; display: inline-block; }
    .json-toggle:hover { text-decoration: underline; }
    .logs-card { font-family: ui-monospace, monospace; font-size: 0.8125rem; }
    .logs-card .log-line { padding: 0.2rem 0; border-bottom: 1px solid #f1f5f9; }
    .logs-card .log-line:last-child { border-bottom: none; }
    .logs-card .log-ok { color: #059669; }
    .logs-card .log-err { color: #dc2626; }
    .logs-card #logsList { max-height: 200px; overflow-y: auto; background: #f8fafc; padding: 0.5rem; border-radius: 6px; margin-top: 0.5rem; }
  </style>
</head>
<body>
  <div class="card">
    <p class="card-title">Project</p>
    <h1>%s</h1>
    <p class="meta">Stage: <strong>%s</strong></p>
    %s
  </div>
  %s
  <div class="card">
    <p class="card-title">Deployment</p>
    %s
  </div>
  %s
  <div class="card">
    <p class="card-title">Actions</p>
    <div class="actions">
      <button type="button" onclick="runAction('doctor')">Doctor</button>
      <button type="button" onclick="runAction('plan')">Plan</button>
      <button type="button" onclick="runAction('deploy')">Deploy</button>
      <button type="button" onclick="runAction('remove')">Remove</button>
      <button type="button" onclick="runAction('unlock')" title="Force release deploy lock if stuck">Unlock</button>
    </div>
    <div id="actionResultPlaceholder" class="action-result-placeholder">Run Doctor, Plan, Deploy, or Remove to see the result here.</div>
    <div id="actionResultCard" class="card action-result-card" style="display:none;">
      <p class="card-title" id="actionResultTitle"></p>
      <p class="result-summary" id="actionResultSummary"></p>
      <a href="#" class="json-toggle" id="actionResultJsonToggle">View JSON</a>
      <pre class="result-json" id="actionResultJson"></pre>
    </div>
  </div>
  <div class="card logs-card">
    <p class="card-title">Logs</p>
    <div id="logsList">Loading...</div>
  </div>
  <p class="meta" style="margin-top:1.5rem;">Config: <code>runfabric.yml</code></p>
  <script>
    (function() {
      var actionJsonToggle = document.getElementById('actionResultJsonToggle');
      var actionJsonPre = document.getElementById('actionResultJson');
      if (actionJsonToggle && actionJsonPre) {
        actionJsonToggle.addEventListener('click', function(e) {
          e.preventDefault();
          var visible = actionJsonPre.classList.toggle('visible');
          actionJsonToggle.textContent = visible ? 'Hide JSON' : 'View JSON';
        });
      }
      refreshLogs();
    })();
    function refreshLogs() {
      var el = document.getElementById('logsList');
      if (!el) return;
      fetch('/api/logs').then(function(r) { return r.json(); }).then(function(entries) {
        if (!entries || entries.length === 0) { el.innerHTML = '<span class="none">No activity yet.</span>'; return; }
        el.innerHTML = entries.map(function(e) {
          var cls = e.action ? (e.ok ? 'log-ok' : 'log-err') : '';
          var text = e.action ? e.time + ' ' + e.action + ' stage=' + (e.stage || '') + ' ' + (e.ok ? 'ok' : 'err') + (e.message ? ' ' + e.message : '')
            : e.time + ' ' + (e.method || '') + ' ' + (e.path || '');
          return '<div class="log-line ' + cls + '">' + escapeHtml(text) + '</div>';
        }).join('');
      }).catch(function() { el.innerHTML = '<span class="none">Could not load logs.</span>'; });
    }
    function escapeHtml(s) {
      var div = document.createElement('div');
      div.textContent = s;
      return div.innerHTML;
    }
    function runAction(action) {
      var stage = new URLSearchParams(window.location.search).get('stage') || 'dev';
      var placeholder = document.getElementById('actionResultPlaceholder');
      var card = document.getElementById('actionResultCard');
      var titleEl = document.getElementById('actionResultTitle');
      var summaryEl = document.getElementById('actionResultSummary');
      var jsonPre = document.getElementById('actionResultJson');
      var jsonToggle = document.getElementById('actionResultJsonToggle');
      var buttons = document.querySelectorAll('.actions button');
      if (placeholder) {
        placeholder.style.display = 'flex';
        placeholder.innerHTML = '<div class="action-loader-spinner"></div><span>Running ' + escapeHtml(action) + '...</span>';
      }
      if (card) card.style.display = 'none';
      for (var i = 0; i < buttons.length; i++) buttons[i].disabled = true;
      fetch('/action/' + action + '?stage=' + encodeURIComponent(stage), { method: 'POST' })
        .then(function(r) { return r.json().then(function(j) { return { ok: r.ok, json: j }; }); })
        .then(function(x) {
          var actionLabel = action.charAt(0).toUpperCase() + action.slice(1);
          if (placeholder) placeholder.style.display = 'none';
          if (card) {
            if (titleEl) titleEl.textContent = actionLabel + ' result';
            if (summaryEl) summaryEl.textContent = x.ok ? 'Success' : (x.json && x.json.error ? x.json.error : 'Failed');
            if (jsonPre) {
              jsonPre.textContent = JSON.stringify(x.json, null, 2);
              jsonPre.classList.remove('visible');
            }
            if (jsonToggle) jsonToggle.textContent = 'View JSON';
            card.className = 'card action-result-card ' + (x.ok ? 'ok' : 'err');
            card.style.display = 'block';
          }
          for (var j = 0; j < buttons.length; j++) buttons[j].disabled = false;
          refreshLogs();
        })
        .catch(function(e) {
          for (var k = 0; k < buttons.length; k++) buttons[k].disabled = false;
          if (placeholder) placeholder.style.display = 'none';
          if (card) {
            if (titleEl) titleEl.textContent = action.charAt(0).toUpperCase() + action.slice(1) + ' result';
            if (summaryEl) summaryEl.textContent = 'Error: ' + e.message;
            if (jsonPre) jsonPre.textContent = 'Error: ' + e.message;
            if (jsonToggle) jsonToggle.textContent = 'View JSON';
            jsonPre.classList.remove('visible');
            card.className = 'card action-result-card err';
            card.style.display = 'block';
          }
          refreshLogs();
        });
    }
  </script>
</body>
</html>
`
