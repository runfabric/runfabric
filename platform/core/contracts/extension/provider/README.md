# Providers Module

This module is intended to hold the canonical provider-related contracts and resolution boundaries.

## Current mapping
- Provider contract + request/result DTOs:
  - `platform/engine/internal/extensions/providers/*`
- Provider plugins / implementations (AWS, GCP, ...):
  - `platform/engine/internal/extensions/provider/*`
- Resolution boundary code:
  - `platform/engine/internal/extensions/resolution/*`

## Suggested incremental move order
1. Move `platform/engine/internal/extensions/providers` types (interfaces + DTOs) into this module
2. Add compatibility wrappers at the old paths so current imports keep working
3. Update internal call sites gradually
4. Move resolution glue last

