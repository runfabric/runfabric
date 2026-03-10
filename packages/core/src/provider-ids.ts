export const PROVIDER_IDS = [
  "aws-lambda",
  "gcp-functions",
  "azure-functions",
  "cloudflare-workers",
  "vercel",
  "netlify",
  "alibaba-fc",
  "digitalocean-functions",
  "fly-machines",
  "ibm-openwhisk"
] as const;

export type ProviderId = (typeof PROVIDER_IDS)[number];
