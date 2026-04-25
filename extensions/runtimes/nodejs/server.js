'use strict';
const http = require('http');
const url  = require('url');

const allRoutes = JSON.parse(process.env.RUNFABRIC_ROUTES || '[]');
const fnFilter  = process.env.RUNFABRIC_FN || '';
const routes    = fnFilter ? allRoutes.filter(r => r.handler === fnFilter) : allRoutes;
const port      = parseInt(process.env.PORT || '80', 10);

// Pre-load all handler modules at startup so the first request isn't slow.
const handlers = {};
for (const r of routes) {
  try {
    const mod = require('./dist/' + r.handler + '.js');
    handlers[r.handler] = mod.handler || mod.default || mod;
  } catch (e) {
    console.error('failed to load handler ' + r.handler + ':', e.message);
  }
}

function matchPath(pattern, pathname) {
  const names = [];
  const re = new RegExp(
    '^' + pattern.replace(/\{([^}]+)\}/g, (_, n) => { names.push(n); return '([^/]+)'; }) + '$'
  );
  const m = pathname.match(re);
  if (!m) return null;
  const params = {};
  names.forEach((n, i) => { params[n] = m[i + 1]; });
  return params;
}

http.createServer((req, res) => {
  const parsed   = url.parse(req.url || '/', true);
  const pathname = parsed.pathname || '/';

  for (const r of routes) {
    if (req.method !== r.method) continue;
    const pathParams = matchPath(r.path, pathname);
    if (pathParams === null) continue;

    const fn = handlers[r.handler];
    if (typeof fn !== 'function') {
      res.writeHead(503, { 'content-type': 'application/json' });
      res.end(JSON.stringify({ error: 'handler not loaded', handler: r.handler }));
      return;
    }

    let raw = '';
    req.on('data', chunk => { raw += chunk; });
    req.on('end', () => {
      const rfReq = {
        method:      req.method,
        path:        pathname,
        pathParams,
        query:       parsed.query,
        headers:     req.headers,
        body:        raw || null,
      };
      Promise.resolve()
        .then(() => fn(rfReq))
        .then(result => {
          const status  = result.status || 200;
          const headers = Object.assign({ 'content-type': 'application/json' }, result.headers || {});
          res.writeHead(status, headers);
          const body = typeof result.body === 'string' ? result.body : JSON.stringify(result.body ?? null);
          res.end(body);
        })
        .catch(err => {
          res.writeHead(500, { 'content-type': 'application/json' });
          res.end(JSON.stringify({ error: String(err && err.message ? err.message : err) }));
        });
    });
    return;
  }

  res.writeHead(404, { 'content-type': 'application/json' });
  res.end(JSON.stringify({ error: 'not found', path: pathname, method: req.method }));
}).listen(port, () => console.log('runfabric node server listening on :' + port));
