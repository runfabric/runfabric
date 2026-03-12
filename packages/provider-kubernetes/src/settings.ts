import type { ProjectConfig } from "@runfabric/core";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function readString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim().length > 0 ? value : undefined;
}

function sanitizeKubernetesName(value: string): string {
  const normalized = value
    .toLowerCase()
    .replace(/[^a-z0-9-]/g, "-")
    .replace(/-{2,}/g, "-")
    .replace(/^-+/, "")
    .replace(/-+$/, "");
  return normalized.length > 0 ? normalized.slice(0, 63) : "runfabric";
}

function extensionConfig(project: ProjectConfig): Record<string, unknown> {
  const extension = project.extensions?.kubernetes;
  return isRecord(extension) ? extension : {};
}

export function resolveKubernetesSettings(project: ProjectConfig): {
  stage: string;
  namespace: string;
  context?: string;
  deploymentName: string;
  serviceName: string;
  defaultEndpoint: string;
} {
  const stage = project.stage || "default";
  const extension = extensionConfig(project);
  const namespace = readString(extension.namespace) || readString(process.env.KUBE_NAMESPACE) || "default";
  const context = readString(extension.context) || readString(process.env.KUBE_CONTEXT);
  const deploymentName = sanitizeKubernetesName(
    readString(extension.deploymentName) || `${project.service}-${stage}`
  );
  const serviceName = sanitizeKubernetesName(readString(extension.serviceName) || deploymentName);
  const ingressHost = readString(extension.ingressHost);
  const defaultEndpoint = ingressHost
    ? ingressHost.startsWith("http://") || ingressHost.startsWith("https://")
      ? ingressHost
      : `https://${ingressHost}`
    : `https://${serviceName}.${namespace}.svc.cluster.local`;

  return {
    stage,
    namespace,
    context,
    deploymentName,
    serviceName,
    defaultEndpoint
  };
}
