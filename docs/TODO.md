# Project TODO

Only pending work is listed here. Completed items are removed.

## Active
- No active items.

## Backlog

### P9 - Optional IaC Resource Provisioning (Terraform / Pulumi)

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
