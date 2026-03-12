export {};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function readString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim().length > 0 ? value : undefined;
}

function toHttpEndpoint(value: string): string {
  return value.startsWith("http://") || value.startsWith("https://") ? value : `https://${value}`;
}

export function endpointFromKubernetesResponse(response: unknown): string | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const directEndpoint = readString(response.endpoint) || readString(response.url) || readString(response.host);
  if (directEndpoint) {
    return toHttpEndpoint(directEndpoint);
  }

  const status = response.status;
  if (isRecord(status) && isRecord(status.loadBalancer) && Array.isArray(status.loadBalancer.ingress)) {
    for (const entry of status.loadBalancer.ingress) {
      if (!isRecord(entry)) {
        continue;
      }
      const host = readString(entry.hostname) || readString(entry.ip);
      if (host) {
        return toHttpEndpoint(host);
      }
    }
  }

  if (isRecord(response.ingress)) {
    const ingressHost = readString(response.ingress.host);
    if (ingressHost) {
      return toHttpEndpoint(ingressHost);
    }
  }

  return undefined;
}

export function kubernetesResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  const sourceMetadata = isRecord(response.metadata) ? response.metadata : {};
  const name = readString(sourceMetadata.name);
  const namespace = readString(sourceMetadata.namespace);
  const uid = readString(sourceMetadata.uid);
  const kind = readString(response.kind);
  const apiVersion = readString(response.apiVersion);

  if (name) {
    metadata.name = name;
  }
  if (namespace) {
    metadata.namespace = namespace;
  }
  if (uid) {
    metadata.uid = uid;
  }
  if (kind) {
    metadata.kind = kind;
  }
  if (apiVersion) {
    metadata.apiVersion = apiVersion;
  }

  return Object.keys(metadata).length > 0 ? metadata : undefined;
}
