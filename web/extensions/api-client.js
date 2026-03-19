export function createRegistryClient(baseUrl, token = "") {
  const root = String(baseUrl || "").replace(/\/$/, "");

  async function request(path, options = {}) {
    const headers = {
      "Content-Type": "application/json",
      ...(options.headers || {}),
    };
    if (token) {
      headers.Authorization = `Bearer ${token}`;
    }
    const res = await fetch(`${root}${path}`, { ...options, headers });
    const payload = await res.json().catch(() => ({}));
    if (!res.ok) {
      const e = payload?.error || { message: res.statusText };
      throw new Error(`${e.code || "REQUEST_FAILED"}: ${e.message || "request failed"}`);
    }
    return payload;
  }

  return {
    resolve: ({ id, core, os, arch, version }) => {
      const q = new URLSearchParams({ id, core, os, arch });
      if (version) q.set("version", version);
      return request(`/v1/extensions/resolve?${q.toString()}`);
    },
    search: ({ q = "", type = "", pluginKind = "", page = 1, pageSize = 20 } = {}) => {
      const params = new URLSearchParams();
      if (q) params.set("q", q);
      if (type) params.set("type", type);
      if (pluginKind) params.set("pluginKind", pluginKind);
      params.set("page", String(page));
      params.set("pageSize", String(pageSize));
      return request(`/v1/extensions/search?${params.toString()}`);
    },
    extension: (id) => request(`/v1/extensions/${encodeURIComponent(id)}`),
    versions: (id) => request(`/v1/extensions/${encodeURIComponent(id)}/versions`),
    advisories: (id) => request(`/v1/extensions/${encodeURIComponent(id)}/advisories`),
    publishStatus: (publishId) => request(`/v1/publish/${encodeURIComponent(publishId)}`),
  };
}
