/*__REGISTRY_CLIENT__*/

const TOKEN_STORAGE_KEY = "runfabric.registry.token";
const THEME_STORAGE_KEY = "runfabric.registry.theme";
const DEFAULT_UI_CONFIG = {
  authLoginURL: "https://auth.runfabric.cloud/device",
  cliDocsURL: "https://runfabric.cloud/docs",
  oidcIssuer: "https://auth.runfabric.cloud",
};

const app = document.getElementById("app");
const client = createRegistryClient("", () => getSavedToken());
let docsIndexPromise;
let uiConfig = { ...DEFAULT_UI_CONFIG };
let uiConfigPromise;

function getSavedToken() {
  try {
    return localStorage.getItem(TOKEN_STORAGE_KEY) || "";
  } catch {
    return "";
  }
}

function setSavedToken(value) {
  try {
    if (value) {
      localStorage.setItem(TOKEN_STORAGE_KEY, value);
    } else {
      localStorage.removeItem(TOKEN_STORAGE_KEY);
    }
  } catch {
    // no-op in restricted browser contexts
  }
}

function getSavedTheme() {
  try {
    const explicit = localStorage.getItem(THEME_STORAGE_KEY);
    if (explicit === "light" || explicit === "dark") {
      return explicit;
    }
  } catch {
    // ignore
  }
  return window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function setSavedTheme(theme) {
  try {
    localStorage.setItem(THEME_STORAGE_KEY, theme);
  } catch {
    // ignore
  }
}

function applyTheme(theme) {
  const safeTheme = theme === "dark" ? "dark" : "light";
  document.body.dataset.theme = safeTheme;
  setSavedTheme(safeTheme);
}

function toggleTheme() {
  const current = document.body.dataset.theme === "dark" ? "dark" : "light";
  applyTheme(current === "dark" ? "light" : "dark");
  const button = document.getElementById("theme-toggle");
  if (button) {
    button.textContent = document.body.dataset.theme === "dark" ? "Light Mode" : "Dark Mode";
  }
}

function escapeHTML(value) {
  return String(value || "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function safe(value, fallback = "") {
  const text = String(value || "").trim();
  return text === "" ? fallback : text;
}

function formatDate(value) {
  const raw = String(value || "").trim();
  if (!raw) {
    return "-";
  }
  const date = new Date(raw);
  if (Number.isNaN(date.getTime())) {
    return raw;
  }
  return date.toLocaleString();
}

function normalizePath(pathname) {
  if (!pathname || pathname === "") {
    return "/";
  }
  if (pathname.length > 1 && pathname.endsWith("/")) {
    return pathname.slice(0, -1);
  }
  return pathname;
}

async function getUIConfig() {
  if (!uiConfigPromise) {
    uiConfigPromise = fetch("/v1/ui/config", { headers: { Accept: "application/json" } })
      .then((response) => {
        if (!response.ok) {
          throw new Error(`ui config request failed: ${response.status}`);
        }
        return response.json();
      })
      .then((payload) => ({
        authLoginURL: safe(payload.authLoginURL, DEFAULT_UI_CONFIG.authLoginURL),
        cliDocsURL: safe(payload.cliDocsURL, DEFAULT_UI_CONFIG.cliDocsURL),
        oidcIssuer: safe(payload.oidcIssuer, DEFAULT_UI_CONFIG.oidcIssuer),
      }))
      .catch(() => ({ ...DEFAULT_UI_CONFIG }));
  }
  uiConfig = await uiConfigPromise;
  return uiConfig;
}

function navLinks(pathname) {
  const links = [
    ["/", "Home"],
    ["/docs", "Docs"],
    ["/extensions", "Extensions"],
    ["/search", "Search"],
    ["/account", "Account"],
  ];
  return links
    .map(([href, label]) => {
      const active = pathname === href || (href !== "/" && pathname.startsWith(href + "/"));
      return `<a class="nav-link${active ? " active" : ""}" href="${href}" data-nav="1">${label}</a>`;
    })
    .join("");
}

function layout(title, body, pathname) {
  const themeLabel = document.body.dataset.theme === "dark" ? "Light Mode" : "Dark Mode";
  return `
    <div class="layout">
      <header>
        <nav class="nav">
          <a href="/" data-nav="1" class="brand">RunFabric Registry</a>
          <div class="nav-right">
            <div class="nav-links">${navLinks(pathname)}</div>
            <button id="theme-toggle" type="button" class="ghost">${themeLabel}</button>
          </div>
        </nav>
      </header>
      <main>
        <section class="surface">
          <h1>${escapeHTML(title)}</h1>
          ${body}
        </section>
      </main>
      <footer>
        Registry UI serves extension-development docs + marketplace. CLI/general docs: <a href="${escapeHTML(uiConfig.cliDocsURL)}" target="_blank" rel="noreferrer">${escapeHTML(uiConfig.cliDocsURL)}</a>
      </footer>
    </div>
  `;
}

function showError(error) {
  const message = error instanceof Error ? error.message : String(error || "Unknown error");
  return `<div class="error">${escapeHTML(message)}</div>`;
}

function installCommand(id, version) {
  const parts = ["runfabric", "extension", "install", id];
  if (version) {
    parts.push("--version", version);
  }
  return parts.join(" ");
}

async function loadDocsIndex() {
  if (!docsIndexPromise) {
    docsIndexPromise = fetch("/docs-index.json", { headers: { Accept: "application/json" } })
      .then((res) => {
        if (!res.ok) {
          throw new Error(`Failed to load docs index (${res.status})`);
        }
        return res.json();
      })
      .then((payload) => {
        const docs = Array.isArray(payload.docs) ? payload.docs : [];
        return {
          generatedAt: payload.generatedAt || "",
          docs,
        };
      });
  }
  return docsIndexPromise;
}

function renderMarkdown(markdown) {
  const lines = String(markdown || "").split(/\r?\n/);
  const out = [];
  let inList = false;
  let inCode = false;
  let codeBuffer = [];

  function closeList() {
    if (inList) {
      out.push("</ul>");
      inList = false;
    }
  }

  function closeCode() {
    if (inCode) {
      out.push(`<pre><code>${escapeHTML(codeBuffer.join("\n"))}</code></pre>`);
      inCode = false;
      codeBuffer = [];
    }
  }

  for (const rawLine of lines) {
    const line = rawLine || "";
    if (line.startsWith("```")) {
      if (inCode) {
        closeCode();
      } else {
        closeList();
        inCode = true;
      }
      continue;
    }
    if (inCode) {
      codeBuffer.push(line);
      continue;
    }
    if (/^###\s+/.test(line)) {
      closeList();
      out.push(`<h3>${escapeHTML(line.replace(/^###\s+/, ""))}</h3>`);
      continue;
    }
    if (/^##\s+/.test(line)) {
      closeList();
      out.push(`<h2>${escapeHTML(line.replace(/^##\s+/, ""))}</h2>`);
      continue;
    }
    if (/^#\s+/.test(line)) {
      closeList();
      out.push(`<h1>${escapeHTML(line.replace(/^#\s+/, ""))}</h1>`);
      continue;
    }
    if (/^[-*]\s+/.test(line)) {
      if (!inList) {
        out.push("<ul>");
        inList = true;
      }
      out.push(`<li>${escapeHTML(line.replace(/^[-*]\s+/, ""))}</li>`);
      continue;
    }
    if (line.trim() === "") {
      closeList();
      continue;
    }
    closeList();
    out.push(`<p>${escapeHTML(line)}</p>`);
  }

  closeList();
  closeCode();
  return `<div class="markdown">${out.join("\n")}</div>`;
}

function normalizeSlug(slugPath) {
  return String(slugPath || "")
    .split("/")
    .filter(Boolean)
    .map((part) => decodeURIComponent(part))
    .join("/");
}

function parseRoute(pathname) {
  const path = normalizePath(pathname);
  if (path === "/") return { name: "home" };
  if (path === "/docs") return { name: "docs-index" };
  if (path.startsWith("/docs/")) return { name: "docs-detail", slug: normalizeSlug(path.slice("/docs/".length)) };
  if (path === "/extensions") return { name: "extensions" };
  if (path.startsWith("/extensions/")) {
    const parts = path.split("/").filter(Boolean);
    if (parts.length === 2) {
      return { name: "extension-detail", id: decodeURIComponent(parts[1]) };
    }
    if (parts.length === 4 && parts[2] === "versions") {
      return {
        name: "extension-version",
        id: decodeURIComponent(parts[1]),
        version: decodeURIComponent(parts[3]),
      };
    }
  }
  if (path.startsWith("/publishers/")) {
    return { name: "publisher", publisher: decodeURIComponent(path.slice("/publishers/".length)) };
  }
  if (path === "/search") return { name: "search" };
  if (path === "/auth/callback") return { name: "auth-callback" };
  if (path === "/auth") return { name: "auth" };
  if (path === "/account") return { name: "account" };
  if (path.startsWith("/orgs/")) return { name: "org-detail", slug: decodeURIComponent(path.slice("/orgs/".length)) };
  if (path === "/orgs") return { name: "account" };
  return { name: "not-found" };
}

function queryParam(name) {
  const params = new URLSearchParams(window.location.search);
  return params.get(name) || "";
}

function setQuery(params) {
  const next = new URL(window.location.href);
  Object.entries(params).forEach(([k, v]) => {
    if (v == null || String(v).trim() === "") {
      next.searchParams.delete(k);
    } else {
      next.searchParams.set(k, String(v));
    }
  });
  history.replaceState({}, "", `${next.pathname}${next.search}`);
}

function navigate(href) {
  history.pushState({}, "", href);
  void render();
}

function bindNavigation() {
  document.querySelectorAll("a[data-nav='1']").forEach((el) => {
    el.addEventListener("click", (event) => {
      const href = el.getAttribute("href") || "";
      if (!href.startsWith("/")) {
        return;
      }
      event.preventDefault();
      navigate(href);
    });
  });
}

function bindThemeToggle() {
  const toggle = document.getElementById("theme-toggle");
  if (!toggle) {
    return;
  }
  toggle.addEventListener("click", () => {
    toggleTheme();
  });
}

function bindSearchForms() {
  const docsSearch = document.getElementById("docs-search-form");
  if (docsSearch) {
    docsSearch.addEventListener("submit", (event) => {
      event.preventDefault();
      const value = document.getElementById("docs-search-input")?.value || "";
      navigate(`/search?q=${encodeURIComponent(value)}`);
    });
  }

  const catalogSearch = document.getElementById("catalog-search-form");
  if (catalogSearch) {
    catalogSearch.addEventListener("submit", (event) => {
      event.preventDefault();
      const value = document.getElementById("catalog-search-input")?.value || "";
      setQuery({ q: value });
      void render();
    });
  }
}

function bindTokenForm() {
  const authForm = document.getElementById("token-form");
  if (authForm) {
    authForm.addEventListener("submit", (event) => {
      event.preventDefault();
      const token = document.getElementById("token-input")?.value || "";
      setSavedToken(token.trim());
      void render();
    });
  }

  const clearToken = document.getElementById("clear-token");
  if (clearToken) {
    clearToken.addEventListener("click", () => {
      setSavedToken("");
      void render();
    });
  }
}

function resolveLoginURL(mode, customInput) {
  if (mode === "issuer") {
    return safe(uiConfig.oidcIssuer);
  }
  if (mode === "custom") {
    return safe(customInput);
  }
  return safe(uiConfig.authLoginURL);
}

function bindLoginRedirect() {
  const form = document.getElementById("sso-login-form");
  const modeSelect = document.getElementById("login-mode");
  const customInput = document.getElementById("custom-login-url");
  const message = document.getElementById("login-message");
  if (!form || !modeSelect || !customInput || !message) {
    return;
  }

  const syncInputState = () => {
    const mode = modeSelect.value;
    customInput.disabled = mode !== "custom";
  };
  syncInputState();

  modeSelect.addEventListener("change", syncInputState);

  form.addEventListener("submit", (event) => {
    event.preventDefault();
    const target = resolveLoginURL(modeSelect.value, customInput.value);
    if (!/^https?:\/\//.test(target)) {
      message.textContent = "Login URL must start with http:// or https://";
      message.className = "error";
      return;
    }
    message.textContent = `Redirecting to ${target}`;
    message.className = "copy";
    window.location.assign(target);
  });
}

function bindPageEvents() {
  bindNavigation();
  bindThemeToggle();
  bindSearchForms();
  bindIdentityAuth();
  bindAccount();
}

async function renderHome(pathname) {
  try {
    const [docsIndex, searchData] = await Promise.all([
      loadDocsIndex(),
      client.searchExtensions({ pageSize: 6 }).catch(() => ({ items: [] })),
    ]);
    const docsCount = docsIndex.docs.length;
    const extensionItems = Array.isArray(searchData.items) ? searchData.items : [];

    const cards = extensionItems
      .map((item) => {
        const publisher = item.publisher && item.publisher.name ? item.publisher.name : "Unknown";
        const trust = item.publisher && item.publisher.trust ? item.publisher.trust : "unknown";
        return `
          <article class="panel">
            <h3><a href="/extensions/${encodeURIComponent(item.id)}" data-nav="1">${escapeHTML(item.name || item.id)}</a></h3>
            <p class="copy">${escapeHTML(item.description || "No description yet.")}</p>
            <p><span class="badge">${escapeHTML(item.type || "plugin")}</span><span class="badge">${escapeHTML(item.pluginKind || "-")}</span>${trustBadge(trust)}</p>
            <p class="copy">Publisher: <a href="/publishers/${encodeURIComponent(publisher)}" data-nav="1">${escapeHTML(publisher)}</a></p>
          </article>
        `;
      })
      .join("");

    const body = `
      <p class="copy">A single registry UI for extension-development docs, marketplace data, and auth/SSO flows.</p>
      <div class="grid">
        <section class="panel">
          <h2>Docs</h2>
          <p>${docsCount} extension-dev docs are indexed from <code>docs/developer</code>.</p>
          <p><a href="/docs" data-nav="1">Browse docs</a></p>
        </section>
        <section class="panel">
          <h2>Marketplace</h2>
          <p>Catalog, extension detail, versions, advisories, trust, and install commands from live registry APIs.</p>
          <p><a href="/extensions" data-nav="1">Explore extensions</a></p>
        </section>
        <section class="panel">
          <h2>Auth</h2>
          <p>Login redirect, token save/clear, and CLI auth guidance live in one place.</p>
          <p><a href="/auth" data-nav="1">Open auth page</a></p>
        </section>
      </div>
      <h2>Featured Extensions</h2>
      <div class="grid">
        ${cards || "<p class='copy'>No extensions available yet.</p>"}
      </div>
    `;
    return layout("Registry Overview", body, pathname);
  } catch (error) {
    return layout("Registry Overview", showError(error), pathname);
  }
}

async function renderDocsIndex(pathname) {
  try {
    const docsIndex = await loadDocsIndex();
    const rows = docsIndex.docs
      .map(
        (doc) => `
          <article class="panel">
            <h3><a href="/docs/${encodeURIComponent(doc.slug)}" data-nav="1">${escapeHTML(doc.title)}</a></h3>
            <p class="copy">${escapeHTML(doc.excerpt || "")}</p>
            <p class="copy">Source: <code>${escapeHTML(doc.source)}</code></p>
          </article>
        `,
      )
      .join("");

    const body = `
      <p class="copy">Extension-development docs only. CLI/general docs stay outside this app and are linked in the footer.</p>
      <form id="docs-search-form" class="input-row" autocomplete="off">
        <input id="docs-search-input" type="search" placeholder="Search docs + marketplace" />
        <button type="submit">Search</button>
      </form>
      <div class="grid">${rows}</div>
    `;
    return layout("Extension Docs", body, pathname);
  } catch (error) {
    return layout("Extension Docs", showError(error), pathname);
  }
}

async function renderDocDetail(pathname, slug) {
  try {
    const docsIndex = await loadDocsIndex();
    const doc = docsIndex.docs.find((item) => item.slug === slug);
    if (!doc) {
      return layout("Doc Not Found", `<p class="copy">No doc found for slug <code>${escapeHTML(slug)}</code>.</p>`, pathname);
    }
    const body = `
      <p class="copy">Source: <code>${escapeHTML(doc.source)}</code></p>
      ${renderMarkdown(doc.markdown)}
    `;
    return layout(doc.title, body, pathname);
  } catch (error) {
    return layout("Doc", showError(error), pathname);
  }
}

function trustBadge(trust) {
  const normalized = String(trust || "").toLowerCase();
  if (normalized === "official" || normalized === "verified") {
    return `<span class="badge ok">${escapeHTML(trust)}</span>`;
  }
  if (normalized === "community") {
    return `<span class="badge warn">${escapeHTML(trust)}</span>`;
  }
  return `<span class="badge">${escapeHTML(trust || "unknown")}</span>`;
}

async function renderExtensions(pathname) {
  const query = queryParam("q");
  const pluginKind = queryParam("kind");
  try {
    const data = await client.searchExtensions({ q: query, pluginKind, pageSize: 60 });
    const items = Array.isArray(data.items) ? data.items : [];
    const cards = items
      .map((item) => {
        const publisher = item.publisher && item.publisher.name ? item.publisher.name : "unknown";
        const trust = item.publisher && item.publisher.trust ? item.publisher.trust : "unknown";
        return `
          <article class="panel">
            <h3><a href="/extensions/${encodeURIComponent(item.id)}" data-nav="1">${escapeHTML(item.name || item.id)}</a></h3>
            <p class="copy">${escapeHTML(item.description || "No description")}</p>
            <p><span class="badge">${escapeHTML(item.type || "plugin")}</span><span class="badge">${escapeHTML(item.pluginKind || "-")}</span>${trustBadge(trust)}</p>
            <p class="copy">Publisher: <a href="/publishers/${encodeURIComponent(publisher)}" data-nav="1">${escapeHTML(publisher)}</a></p>
            <p class="copy">Latest: ${escapeHTML(item.latestVersion || "-")}</p>
          </article>
        `;
      })
      .join("");

    const body = `
      <form id="catalog-search-form" class="input-row" autocomplete="off">
        <input id="catalog-search-input" type="search" value="${escapeHTML(query)}" placeholder="Search extensions" />
        <button type="submit">Search</button>
      </form>
      <p class="copy">Showing ${items.length} of ${Number(data.total || items.length)} results.</p>
      <div class="grid">${cards || "<p class='copy'>No extensions found.</p>"}</div>
    `;
    return layout("Extension Marketplace", body, pathname);
  } catch (error) {
    return layout("Extension Marketplace", showError(error), pathname);
  }
}

async function renderExtensionDetail(pathname, id) {
  try {
    const [detail, versionsPayload, advisoriesPayload] = await Promise.all([
      client.extensionDetail(id),
      client.extensionVersions(id),
      client.extensionAdvisories(id),
    ]);

    const versions = Array.isArray(versionsPayload.versions) ? versionsPayload.versions : [];
    const advisories = Array.isArray(advisoriesPayload.advisories) ? advisoriesPayload.advisories : [];
    const publisher = detail.publisher || {};

    const versionRows = versions
      .slice(0, 20)
      .map(
        (version) => `
          <tr>
            <td><a href="/extensions/${encodeURIComponent(id)}/versions/${encodeURIComponent(version.version)}" data-nav="1">${escapeHTML(version.version)}</a></td>
            <td>${escapeHTML(version.releaseStatus || "-")}</td>
            <td>${escapeHTML(version.coreConstraint || (version.compatibility && version.compatibility.core) || "-")}</td>
            <td>${formatDate(version.publishedAt)}</td>
          </tr>
        `,
      )
      .join("");

    const advisoryRows = advisories
      .map(
        (advisory) => `
          <tr>
            <td>${escapeHTML(advisory.id)}</td>
            <td><span class="badge ${advisory.severity === "critical" ? "danger" : "warn"}">${escapeHTML(advisory.severity || "unknown")}</span></td>
            <td>${escapeHTML(advisory.summary || "")}</td>
            <td>${formatDate(advisory.publishedAt)}</td>
          </tr>
        `,
      )
      .join("");

    const latest = safe(detail.latestVersion);
    const body = `
      <p class="copy">${escapeHTML(detail.description || "No description")}</p>
      <p>
        <span class="badge">${escapeHTML(detail.type || "plugin")}</span>
        <span class="badge">${escapeHTML(detail.pluginKind || "-")}</span>
        ${trustBadge(publisher.trust || "unknown")}
      </p>
      <p class="copy">Publisher: <a href="/publishers/${encodeURIComponent(publisher.name || publisher.id || "unknown")}" data-nav="1">${escapeHTML(publisher.name || publisher.id || "unknown")}</a></p>
      <h2>Install</h2>
      <pre><code>${escapeHTML(installCommand(id, latest))}</code></pre>
      <h2>Versions</h2>
      <div class="table-wrap">
        <table>
          <thead><tr><th>Version</th><th>Status</th><th>Core</th><th>Published</th></tr></thead>
          <tbody>${versionRows || "<tr><td colspan='4'>No versions found.</td></tr>"}</tbody>
        </table>
      </div>
      <h2>Advisories</h2>
      <div class="table-wrap">
        <table>
          <thead><tr><th>ID</th><th>Severity</th><th>Summary</th><th>Published</th></tr></thead>
          <tbody>${advisoryRows || "<tr><td colspan='4'>No advisories.</td></tr>"}</tbody>
        </table>
      </div>
    `;

    return layout(`Extension: ${detail.name || id}`, body, pathname);
  } catch (error) {
    return layout(`Extension: ${id}`, showError(error), pathname);
  }
}

async function renderExtensionVersion(pathname, id, version) {
  try {
    const detail = await client.extensionVersionDetail(id, version);
    const artifacts = Array.isArray(detail.artifact) ? detail.artifact : [];
    const permissions = Array.isArray(detail.permissions) ? detail.permissions : [];
    const capabilities = Array.isArray(detail.capabilities) ? detail.capabilities : [];

    const artifactRows = artifacts
      .map(
        (artifact) => `
          <tr>
            <td>${escapeHTML(artifact.os || "any")}/${escapeHTML(artifact.arch || "any")}</td>
            <td>${escapeHTML(artifact.format || "-")}</td>
            <td>${escapeHTML(String(artifact.sizeBytes || "-"))}</td>
            <td><a href="${escapeHTML(artifact.url || "#")}" target="_blank" rel="noreferrer">download</a></td>
          </tr>
        `,
      )
      .join("");

    const body = `
      <p class="copy">${escapeHTML(detail.description || "No description")}</p>
      <p>${trustBadge(detail.publisher && detail.publisher.trust ? detail.publisher.trust : "unknown")}</p>
      <h2>Install</h2>
      <pre><code>${escapeHTML(installCommand(id, version))}</code></pre>
      <h2>Compatibility</h2>
      <pre><code>${escapeHTML(JSON.stringify(detail.compatibility || {}, null, 2))}</code></pre>
      <h2>Capabilities</h2>
      <p>${capabilities.map((item) => `<span class="badge">${escapeHTML(item)}</span>`).join("") || "<span class='copy'>None</span>"}</p>
      <h2>Permissions</h2>
      <p>${permissions.map((item) => `<span class="badge">${escapeHTML(item)}</span>`).join("") || "<span class='copy'>None</span>"}</p>
      <h2>Artifacts</h2>
      <div class="table-wrap">
        <table>
          <thead><tr><th>Target</th><th>Format</th><th>Size</th><th>URL</th></tr></thead>
          <tbody>${artifactRows || "<tr><td colspan='4'>No artifacts published.</td></tr>"}</tbody>
        </table>
      </div>
    `;

    return layout(`Extension ${id} ${version}`, body, pathname);
  } catch (error) {
    return layout(`Extension ${id} ${version}`, showError(error), pathname);
  }
}

async function renderPublisher(pathname, publisherQuery) {
  try {
    const data = await client.searchExtensions({ q: publisherQuery, pageSize: 100 });
    const items = Array.isArray(data.items) ? data.items : [];
    const filtered = items.filter((item) => {
      const publisher = item.publisher && (item.publisher.name || item.publisher.id) ? String(item.publisher.name || item.publisher.id) : "";
      return publisher.toLowerCase() === publisherQuery.toLowerCase() || publisher.toLowerCase().includes(publisherQuery.toLowerCase());
    });
    const list = (filtered.length > 0 ? filtered : items)
      .map(
        (item) => `
          <tr>
            <td><a href="/extensions/${encodeURIComponent(item.id)}" data-nav="1">${escapeHTML(item.name || item.id)}</a></td>
            <td>${escapeHTML(item.latestVersion || "-")}</td>
            <td>${escapeHTML(item.type || "-")}</td>
            <td>${escapeHTML(item.pluginKind || "-")}</td>
          </tr>
        `,
      )
      .join("");

    const body = `
      <p class="copy">Publisher filter: <strong>${escapeHTML(publisherQuery)}</strong></p>
      <div class="table-wrap">
        <table>
          <thead><tr><th>Extension</th><th>Latest</th><th>Type</th><th>Kind</th></tr></thead>
          <tbody>${list || "<tr><td colspan='4'>No extensions found for this publisher.</td></tr>"}</tbody>
        </table>
      </div>
    `;
    return layout(`Publisher: ${publisherQuery}`, body, pathname);
  } catch (error) {
    return layout(`Publisher: ${publisherQuery}`, showError(error), pathname);
  }
}

async function renderUnifiedSearch(pathname) {
  const q = queryParam("q").trim();
  if (!q) {
    const body = `
      <p class="copy">Unified search spans extension-dev docs and live marketplace entities.</p>
      <form id="docs-search-form" class="input-row" autocomplete="off">
        <input id="docs-search-input" type="search" placeholder="Search docs + extensions + packages" />
        <button type="submit">Search</button>
      </form>
    `;
    return layout("Search", body, pathname);
  }

  try {
    const [docsIndex, extensions, packages] = await Promise.all([
      loadDocsIndex(),
      client.searchExtensions({ q, pageSize: 30 }).catch(() => ({ items: [] })),
      client.listPackages({ q }).catch(() => ({ items: [] })),
    ]);

    const docsMatches = docsIndex.docs.filter((doc) => {
      const hay = `${doc.title} ${doc.excerpt} ${doc.searchText}`.toLowerCase();
      return hay.includes(q.toLowerCase());
    });
    const extensionItems = Array.isArray(extensions.items) ? extensions.items : [];
    const packageItems = Array.isArray(packages.items) ? packages.items : [];

    const docsResults = docsMatches
      .map(
        (doc) => `<li><a href="/docs/${encodeURIComponent(doc.slug)}" data-nav="1">${escapeHTML(doc.title)}</a> <span class="copy">(${escapeHTML(doc.source)})</span></li>`,
      )
      .join("");

    const extensionResults = extensionItems
      .map(
        (item) => `<li><a href="/extensions/${encodeURIComponent(item.id)}" data-nav="1">${escapeHTML(item.name || item.id)}</a> <span class="copy">(${escapeHTML(item.latestVersion || "-")})</span></li>`,
      )
      .join("");

    const packageResults = packageItems
      .map(
        (pkg) => `<li><code>${escapeHTML(`${pkg.namespace}/${pkg.name}`)}</code> <span class="copy">(${escapeHTML(pkg.visibility || "-")})</span></li>`,
      )
      .join("");

    const body = `
      <form id="docs-search-form" class="input-row" autocomplete="off">
        <input id="docs-search-input" type="search" value="${escapeHTML(q)}" placeholder="Search docs + marketplace" />
        <button type="submit">Search</button>
      </form>
      <div class="grid">
        <section class="panel">
          <h2>Docs (${docsMatches.length})</h2>
          <ul>${docsResults || "<li>No docs match.</li>"}</ul>
        </section>
        <section class="panel">
          <h2>Extensions (${extensionItems.length})</h2>
          <ul>${extensionResults || "<li>No extensions match.</li>"}</ul>
        </section>
        <section class="panel">
          <h2>Packages (${packageItems.length})</h2>
          <ul>${packageResults || "<li>No packages match.</li>"}</ul>
        </section>
      </div>
    `;
    return layout(`Search: ${q}`, body, pathname);
  } catch (error) {
    return layout("Search", showError(error), pathname);
  }
}

function identityBaseURL() {
  const issuer = safe(uiConfig.oidcIssuer, DEFAULT_UI_CONFIG.oidcIssuer);
  return issuer.replace(/\/$/, "");
}

async function renderAuth(pathname) {
  const token = getSavedToken();
  const issuerURL = identityBaseURL();

  if (token) {
    const body = `
      <p class="copy">You are signed in to the registry.</p>
      <section class="panel" style="max-width:560px">
        <h2>Session</h2>
        <p class="copy">A RunFabric identity token is stored and sent with registry API requests.</p>
        <div class="input-row" style="margin-top:.6rem">
          <a href="/" data-nav="1"><button type="button">Go to registry</button></a>
          <button id="signout-btn" type="button" class="ghost">Sign out</button>
        </div>
      </section>
    `;
    return layout("Account", body, pathname);
  }

  // Bounce to the identity service; it authenticates (its own login page) and
  // redirects back to /auth/callback with a token in the URL fragment.
  const redirectUri = window.location.origin + "/auth/callback";
  const authorizeUrl =
    identityBaseURL() + "/sso/authorize?redirect_uri=" + encodeURIComponent(redirectUri);

  const body = `
    <p class="copy">Sign in with your RunFabric identity to publish and manage extensions.</p>
    <section class="panel" style="max-width:520px">
      <h2>Authenticate</h2>
      <p class="copy">You will be redirected to the RunFabric identity service and returned here once signed in.</p>
      <div class="input-row" style="margin-top:.6rem">
        <a href="${escapeHTML(authorizeUrl)}"><button type="button">Sign in with RunFabric</button></a>
      </div>
    </section>
    <p class="copy" style="margin-top:12px">Identity provider: <code>${escapeHTML(issuerURL)}</code></p>
  `;
  return layout("Sign In", body, pathname);
}

// Callback landing after the identity service redirects back with the token in
// the URL fragment (#access_token=...). Store it, clean the URL, go home.
async function renderAuthCallback(pathname) {
  const hash = (window.location.hash || "").replace(/^#/, "");
  const params = new URLSearchParams(hash);
  const token = params.get("access_token");
  if (token) {
    setSavedToken(token);
    setTimeout(() => {
      history.replaceState({}, "", "/");
      void render();
    }, 0);
    return layout("Signing in…", '<p class="copy">Signed in. Returning to the registry…</p>', pathname);
  }
  const error = params.get("error") || "No token was returned by the identity service.";
  const body = `
    <div class="error">${escapeHTML(error)}</div>
    <p class="copy" style="margin-top:10px"><a href="/auth" data-nav="1">Back to sign in</a></p>
  `;
  return layout("Sign in failed", body, pathname);
}

function bindIdentityAuth() {
  const signout = document.getElementById("signout-btn");
  if (signout) {
    signout.addEventListener("click", () => {
      setSavedToken("");
      void render();
    });
  }
}

// --- Account dashboard + organizations (npm-style) ---

function decodeTokenClaims(token) {
  try {
    const part = String(token || "").split(".")[1];
    if (!part) return {};
    const pad = part + "=".repeat((4 - (part.length % 4)) % 4);
    const json = atob(pad.replace(/-/g, "+").replace(/_/g, "/"));
    return JSON.parse(json);
  } catch {
    return {};
  }
}

function currentUserId() {
  const claims = decodeTokenClaims(getSavedToken());
  return String(claims.sub || claims.user_id || claims.subject || "");
}

const ACCOUNT_SECTIONS = [
  ["profile", "Profile"],
  ["packages", "Packages"],
  ["access-tokens", "Access Tokens"],
  ["staged", "Staged Packages"],
  ["billing", "Billing Info"],
];

async function renderAccount(pathname) {
  const token = getSavedToken();
  if (!token) {
    setTimeout(() => navigate("/auth"), 0);
    return layout("Account", '<p class="copy">Redirecting to sign in…</p>', pathname);
  }
  const claims = decodeTokenClaims(token);
  const section = queryParam("section") || "profile";

  let orgs = [];
  let orgError = "";
  try {
    const res = await client.listOrganizations();
    orgs = Array.isArray(res.items) ? res.items : [];
  } catch (error) {
    orgError = error instanceof Error ? error.message : String(error);
  }

  const navItem = (id, label) =>
    `<a class="acct-nav-link${section === id ? " active" : ""}" href="/account?section=${id}" data-nav="1">${escapeHTML(label)}</a>`;
  const orgLinks = orgs
    .map(
      (o) =>
        `<a class="acct-nav-sub" href="/orgs/${encodeURIComponent(o.slug)}" data-nav="1">${escapeHTML(o.name || o.slug)}</a>`,
    )
    .join("");

  const nav = `
    <aside class="acct-nav">
      ${ACCOUNT_SECTIONS.map(([id, label]) => navItem(id, label)).join("")}
      <div class="acct-nav-group">Organizations</div>
      <a class="acct-nav-sub create${section === "new-org" ? " active" : ""}" href="/account?section=new-org" data-nav="1">+ Create New Organization</a>
      ${orgLinks || '<span class="copy" style="padding:4px 12px;display:block">No organizations yet</span>'}
    </aside>
  `;

  const content = accountSectionContent(section, claims, token, orgs, orgError);
  const body = `<div class="acct-layout">${nav}<section class="acct-main">${content}</section></div>`;
  return layout("Account", body, pathname);
}

function accountSectionContent(section, claims, token, orgs, orgError) {
  const email = safe(claims.email, "");
  const sub = safe(claims.sub, "");
  switch (section) {
    case "packages":
      return `
        <h2>Packages</h2>
        <p class="copy">Packages you publish appear here. Browse everything in the <a href="/extensions" data-nav="1">marketplace</a>.</p>
      `;
    case "access-tokens":
      return `
        <h2>Access Tokens</h2>
        <p class="copy">This browser holds a bearer token used for registry API calls.</p>
        <div class="panel" style="max-width:640px">
          <p class="copy" style="margin:0 0 8px">Current token</p>
          <pre style="white-space:pre-wrap;word-break:break-all">${escapeHTML(token.slice(0, 24))}…${escapeHTML(token.slice(-12))}</pre>
          <div class="input-row" style="margin-top:10px">
            <button id="copy-token" type="button" class="ghost">Copy token</button>
            <button id="signout-btn" type="button" class="ghost">Sign out</button>
          </div>
        </div>
      `;
    case "staged":
      return `<h2>Staged Packages</h2><p class="copy">No staged packages. Packages awaiting finalize will show here.</p>`;
    case "billing":
      return `<h2>Billing Info</h2><p class="copy">Billing is not configured for this registry.</p>`;
    case "new-org":
      return `
        <h2>Create New Organization</h2>
        <p class="copy">Organizations own a package scope (<code>@your-org/pkg</code>) and have members with roles.</p>
        <section class="panel" style="max-width:560px">
          <form id="create-org-form" autocomplete="off">
            <div style="margin-bottom:12px">
              <label style="display:block;font-weight:600;font-size:13px;margin-bottom:5px">Organization name (scope)</label>
              <input id="org-slug" type="text" required placeholder="my-org" style="width:100%" />
              <p class="copy" style="margin-top:4px">Lowercase letters, numbers and dashes. Used as <code>@my-org</code>.</p>
            </div>
            <div style="margin-bottom:12px">
              <label style="display:block;font-weight:600;font-size:13px;margin-bottom:5px">Display name (optional)</label>
              <input id="org-name" type="text" placeholder="My Organization" style="width:100%" />
            </div>
            <div style="margin-bottom:12px">
              <label style="display:block;font-weight:600;font-size:13px;margin-bottom:5px">Default package visibility</label>
              <select id="org-visibility" style="width:100%">
                <option value="public">Public</option>
                <option value="tenant">Private (tenant)</option>
              </select>
            </div>
            <div class="input-row"><button type="submit">Create organization</button></div>
            <p id="create-org-message" class="copy" style="margin-top:10px"></p>
          </form>
        </section>
      `;
    case "profile":
    default:
      return `
        <h2>Profile</h2>
        <div class="panel" style="max-width:640px">
          <table><tbody>
            <tr><th style="width:140px">Email</th><td>${escapeHTML(email || "—")}</td></tr>
            <tr><th>User ID</th><td><code>${escapeHTML(sub || "—")}</code></td></tr>
            <tr><th>Organizations</th><td>${orgError ? `<span class="copy">${escapeHTML(orgError)}</span>` : String(orgs.length)}</td></tr>
          </tbody></table>
        </div>
      `;
  }
}

async function renderOrgDetail(pathname, slug) {
  const token = getSavedToken();
  if (!token) {
    setTimeout(() => navigate("/auth"), 0);
    return layout("Organization", '<p class="copy">Redirecting to sign in…</p>', pathname);
  }
  let org;
  try {
    org = await client.getOrganization(slug);
  } catch (error) {
    return layout("Organization", showError(error), pathname);
  }
  const me = currentUserId();
  const myRole = (org.members || []).find((m) => m.userId === me)?.role || "";
  const canManage = myRole === "owner" || myRole === "admin";

  const rows = (org.members || [])
    .map(
      (m) => `
        <tr>
          <td>${escapeHTML(m.email || m.userId)}</td>
          <td><span class="badge${m.role === "owner" ? " ok" : ""}">${escapeHTML(m.role)}</span></td>
          <td>${canManage && m.userId !== me ? `<button class="ghost remove-member-btn" data-user="${escapeHTML(m.userId)}" type="button">Remove</button>` : ""}</td>
        </tr>
      `,
    )
    .join("");

  const manageForm = canManage
    ? `
      <section class="panel" style="max-width:640px;margin-top:1rem">
        <h3>Add member</h3>
        <form id="add-member-form" autocomplete="off" data-slug="${escapeHTML(org.slug)}">
          <div class="input-row">
            <input id="member-user" type="text" placeholder="user id" required />
            <input id="member-email" type="email" placeholder="email (optional)" />
            <select id="member-role">
              <option value="developer">Developer</option>
              <option value="admin">Admin</option>
              <option value="owner">Owner</option>
            </select>
            <button type="submit">Add</button>
          </div>
          <p id="add-member-message" class="copy" style="margin-top:8px"></p>
        </form>
      </section>`
    : "";

  const body = `
    <p class="copy"><a href="/account?section=new-org" data-nav="1">← Account</a></p>
    <p>
      <span class="badge">@${escapeHTML(org.slug)}</span>
      <span class="badge${org.defaultVisibility === "public" ? " ok" : ""}">${escapeHTML(org.defaultVisibility || "public")}</span>
      ${myRole ? `<span class="badge">you: ${escapeHTML(myRole)}</span>` : ""}
    </p>
    ${org.description ? `<p class="copy">${escapeHTML(org.description)}</p>` : ""}
    <h2>Members (${(org.members || []).length})</h2>
    <div class="table-wrap" data-slug="${escapeHTML(org.slug)}">
      <table>
        <thead><tr><th>Member</th><th>Role</th><th></th></tr></thead>
        <tbody>${rows}</tbody>
      </table>
    </div>
    ${manageForm}
  `;
  return layout(org.name || org.slug, body, pathname);
}

function bindAccount() {
  const copyToken = document.getElementById("copy-token");
  if (copyToken) {
    copyToken.addEventListener("click", () => {
      navigator.clipboard?.writeText(getSavedToken());
      copyToken.textContent = "Copied";
      setTimeout(() => (copyToken.textContent = "Copy token"), 1200);
    });
  }

  const createForm = document.getElementById("create-org-form");
  if (createForm) {
    createForm.addEventListener("submit", async (event) => {
      event.preventDefault();
      const message = document.getElementById("create-org-message");
      const slug = (document.getElementById("org-slug")?.value || "").trim();
      const name = (document.getElementById("org-name")?.value || "").trim();
      const visibility = document.getElementById("org-visibility")?.value || "public";
      if (!slug) {
        message.textContent = "Organization name is required.";
        message.className = "error";
        return;
      }
      message.textContent = "Creating…";
      message.className = "copy";
      try {
        const org = await client.createOrganization({ slug, name, visibility });
        navigate("/orgs/" + encodeURIComponent(org.slug));
      } catch (error) {
        message.textContent = error instanceof Error ? error.message : String(error);
        message.className = "error";
      }
    });
  }

  const addForm = document.getElementById("add-member-form");
  if (addForm) {
    addForm.addEventListener("submit", async (event) => {
      event.preventDefault();
      const slug = addForm.getAttribute("data-slug");
      const message = document.getElementById("add-member-message");
      const userId = (document.getElementById("member-user")?.value || "").trim();
      const email = (document.getElementById("member-email")?.value || "").trim();
      const role = document.getElementById("member-role")?.value || "developer";
      if (!userId) {
        message.textContent = "User id is required.";
        message.className = "error";
        return;
      }
      try {
        await client.addOrganizationMember(slug, { userId, email, role });
        void render();
      } catch (error) {
        message.textContent = error instanceof Error ? error.message : String(error);
        message.className = "error";
      }
    });
  }

  document.querySelectorAll(".remove-member-btn").forEach((btn) => {
    btn.addEventListener("click", async () => {
      const wrap = btn.closest("[data-slug]");
      const slug = wrap ? wrap.getAttribute("data-slug") : "";
      const userId = btn.getAttribute("data-user");
      try {
        await client.removeOrganizationMember(slug, userId);
        void render();
      } catch (error) {
        alert(error instanceof Error ? error.message : String(error));
      }
    });
  });
}

async function renderNotFound(pathname) {
  const body = `<p class="copy">No page matches <code>${escapeHTML(pathname)}</code>.</p>`;
  return layout("Not Found", body, pathname);
}

async function render() {
  await getUIConfig();
  const pathname = normalizePath(window.location.pathname);
  const route = parseRoute(pathname);
  let html = "";
  switch (route.name) {
    case "home":
      html = await renderHome(pathname);
      break;
    case "docs-index":
      html = await renderDocsIndex(pathname);
      break;
    case "docs-detail":
      html = await renderDocDetail(pathname, route.slug);
      break;
    case "extensions":
      html = await renderExtensions(pathname);
      break;
    case "extension-detail":
      html = await renderExtensionDetail(pathname, route.id);
      break;
    case "extension-version":
      html = await renderExtensionVersion(pathname, route.id, route.version);
      break;
    case "publisher":
      html = await renderPublisher(pathname, route.publisher);
      break;
    case "search":
      html = await renderUnifiedSearch(pathname);
      break;
    case "auth":
      html = await renderAuth(pathname);
      break;
    case "auth-callback":
      html = await renderAuthCallback(pathname);
      break;
    case "account":
      html = await renderAccount(pathname);
      break;
    case "org-detail":
      html = await renderOrgDetail(pathname, route.slug);
      break;
    default:
      html = await renderNotFound(pathname);
      break;
  }

  app.innerHTML = html;
  bindPageEvents();
}

window.addEventListener("popstate", () => {
  void render();
});

applyTheme(getSavedTheme());
void render();
