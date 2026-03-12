# @runfabric/provider-kubernetes

Kubernetes provider adapter for `runfabric`.

## Install

```bash
npm install @runfabric/provider-kubernetes @runfabric/core
```

## Usage

```ts
import { createKubernetesProvider } from "@runfabric/provider-kubernetes";

const provider = createKubernetesProvider({ projectDir: process.cwd() });
```

## Credentials

Required environment variable:

- `KUBECONFIG`

Optional:

- `KUBE_CONTEXT`
- `KUBE_NAMESPACE`
