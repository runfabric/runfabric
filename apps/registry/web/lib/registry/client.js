const DEFAULT_TIMEOUT_MS = 12000;

function normalizeBaseUrl(baseUrl) {
  const raw = String(baseUrl || "").trim();
  if (raw === "") {
    return "";
  }
  return raw.endsWith("/") ? raw.slice(0, -1) : raw;
}

export function createRegistryClient(baseUrl = "", tokenProvider = () => "") {
  const root = normalizeBaseUrl(baseUrl);

  async function request(path, options = {}) {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), DEFAULT_TIMEOUT_MS);
    try {
      const headers = {
        Accept: "application/json",
        ...(options.body ? { "Content-Type": "application/json" } : {}),
        ...(options.headers || {})
      };
      const token = tokenProvider();
      if (token) {
        headers.Authorization = `Bearer ${token}`;
      }
      const response = await fetch(`${root}${path}`, {
        ...options,
        headers,
        signal: controller.signal
      });
      const payload = await response.json().catch(() => ({}));
      if (!response.ok) {
        const apiError = payload && payload.error ? payload.error : { message: response.statusText };
        const message = apiError.message || "request failed";
        const code = apiError.code || "REQUEST_FAILED";
        throw new Error(`${code}: ${message}`);
      }
      return payload;
    } finally {
      clearTimeout(timeout);
    }
  }

  return {
    listPackages(params = {}) {
      const q = new URLSearchParams();
      if (params.q) q.set("q", params.q);
      if (params.namespace) q.set("namespace", params.namespace);
      const query = q.toString();
      return request(`/packages${query ? `?${query}` : ""}`);
    },
    packageDetail(namespace, name) {
      return request(`/packages/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`);
    },
    searchExtensions(params = {}) {
      const q = new URLSearchParams();
      if (params.q) q.set("q", params.q);
      if (params.type) q.set("type", params.type);
      if (params.pluginKind) q.set("pluginKind", params.pluginKind);
      q.set("page", String(params.page || 1));
      q.set("pageSize", String(params.pageSize || 24));
      return request(`/v1/extensions/search?${q.toString()}`);
    },
    extensionDetail(id) {
      return request(`/v1/extensions/${encodeURIComponent(id)}`);
    },
    extensionVersions(id) {
      return request(`/v1/extensions/${encodeURIComponent(id)}/versions`);
    },
    extensionVersionDetail(id, version) {
      return request(`/v1/extensions/${encodeURIComponent(id)}/versions/${encodeURIComponent(version)}`);
    },
    extensionAdvisories(id) {
      return request(`/v1/extensions/${encodeURIComponent(id)}/advisories`);
    }
  };
}
