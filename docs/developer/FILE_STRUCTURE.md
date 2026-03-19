# File structure

## Quick navigation

- **Repo tree**: diagram below
- **Package naming**: Package naming conventions

```
runfabric/
├── README.md
├── LICENSE
├── docs/
├── scripts/
├── .github/
│   └── workflows/
│
├── bin/                      # built binaries (e.g. `bin/runfabric`)
│
├── engine/
│   └── ...                     # shared engine source / binary build logic
│
├── packages/
│   ├── node/
│   │   ├── cli/                 # publishes @runfabric/cli
│   │   └── sdk/                 # publishes @runfabric/sdk
│   │
│   ├── python/
│   │   └── runfabric/           # publishes runfabric (CLI + SDK); future: cli/ + sdk/
│   │
│   ├── go/
│   │   ├── sdk/                 # module .../packages/go/sdk
│   │   └── plugin-sdk/          # module .../packages/go/plugin-sdk (external plugin authoring)
│   │
│   ├── java/
│   │   └── sdk/                 # artifact io.runfabric:runfabric-sdk
│   │
│   └── dotnet/
│       └── sdk/                 # package RunFabric.Sdk
│
├── schemas/                  # JSON schemas (runfabric.yml, resources, workflows, protocol)
│
├── registry/                 # extension registry service (API + SPA)
│   ├── internal/             # backend APIs and data services
│   └── web/                  # registry UI (extension docs + marketplace + auth)
│
└── examples/
    ├── node/
    ├── python/
    ├── go/
    ├── java/
    └── dotnet/
```

Notes:

- `docs/` remains the source of truth for long-form docs content.
- `registry/web/` renders extension-dev docs and marketplace UX; it should not duplicate markdown trees from `docs/`.
- `registry/` owns both API and UI deployment; keep backend business rules in `registry/internal/*` and UI consumption in `registry/web/*`.

## Package naming conventions

### Node

| Package       | Install                  | Usage |
|---------------|--------------------------|--------|
| @runfabric/cli | `npm i @runfabric/cli -g` | CLI + programmatic `run`, `deploy`, `inspect`, `build` |
| @runfabric/sdk | `npm i @runfabric/sdk`    | `import { createHandler, UniversalHandler } from "@runfabric/sdk"` |

### Python

| Package        | Install                | Usage |
|----------------|------------------------|--------|
| runfabric      | `pip install runfabric` | CLI + programmatic `run`, `plan`, `deploy`, `build` |
| runfabric-sdk  | *(future)* `pip install runfabric-sdk` | `from runfabric_sdk import Handler` or `from runfabric.sdk import UniversalHandler` |

### Java (Maven / Gradle)

- **GroupId:** `io.runfabric`  
- **ArtifactId:** `runfabric-sdk`  

```gradle
implementation "io.runfabric:runfabric-sdk:1.0.0"
```

### .NET (NuGet)

- **Package:** `RunFabric.Sdk`  

```csharp
using RunFabric.Sdk;
```

### Go modules

- `packages/go/sdk` — in-function Go runtime SDK (`github.com/runfabric/runfabric/sdk/go`)
- `packages/go/plugin-sdk` — external plugin SDK (`github.com/runfabric/runfabric/plugin-sdk/go`)
