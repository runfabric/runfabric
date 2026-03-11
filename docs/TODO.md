# Project TODO

Only pending work is listed here. Completed items are removed.

## P8 - Multi-Runtime Support (Post Node-First GA)

- Add typed runtime support matrix in core/planner:
  - `runtime: nodejs | python | go | java | rust | dotnet`
  - provider runtime mapping validation errors (clear per-provider unsupported runtime message).
- Extend build adapters beyond Node.js:
  - python packaging adapter (`pip`/venv artifact flow)
  - go build adapter
  - java/jar packaging adapter
  - rust binary packaging adapter
  - dotnet publish adapter

## P9 - Optional IaC Resource Provisioning (Terraform / Pulumi)

- Add optional Terraform-backed provisioning mode for resources:
  - `resources.provisioner: native | terraform | pulumi`
  - `resources.terraform.dir`
  - `resources.terraform.workspace`
  - `resources.terraform.vars`
  - `resources.terraform.autoApprove` (default `false`)
- Add optional Pulumi-backed provisioning mode for resources:
  - `resources.pulumi.project`
  - `resources.pulumi.stack`
  - `resources.pulumi.workDir`
  - `resources.pulumi.config`
  - `resources.pulumi.refresh` (default `true`)
- Add IaC lifecycle CLI wrappers with provisioner-scoped commands to avoid overlap:
  - `runfabric resources plan --provisioner terraform`
  - `runfabric resources apply --provisioner terraform`
  - `runfabric resources preview --provisioner pulumi`
  - `runfabric resources up --provisioner pulumi`
  - `runfabric resources destroy --provisioner <terraform|pulumi>`
- Define state ownership boundaries to avoid dual source-of-truth:
  - Terraform state is canonical for infra resources when `resources.provisioner=terraform`
  - Pulumi state is canonical for infra resources when `resources.provisioner=pulumi`
  - runfabric state remains canonical for function deploy metadata/endpoints
  - persist Terraform/Pulumi outputs/imported references into runfabric state without copying full backend state files.

## P10 - Release Operations (Post-Implementation Validation)

- Execute first real (non-dry-run) release with production npm credentials and verify:
  - package publish sequence succeeds on npm,
  - git tag push is successful,
  - GitHub release body is sourced from `release-notes/<version>.md`.

## P11 - Production Remote State Backends

- Replace dev/test simulated remote state storage paths with production backend drivers for:
  - `postgres`
  - `s3`
  - `gcs`
  - `azblob`
