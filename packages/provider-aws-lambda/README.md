# @runfabric/provider-aws-lambda

AWS Lambda provider adapter for RunFabric.

## Install

```bash
npm install @runfabric/provider-aws-lambda @runfabric/core
```

## Usage

```ts
import { createAwsLambdaProvider } from "@runfabric/provider-aws-lambda";

const provider = createAwsLambdaProvider({ projectDir: process.cwd() });
```

In normal usage this package is loaded dynamically by `@runfabric/cli`.
