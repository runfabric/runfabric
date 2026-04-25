package runtimes

import _ "embed"

// NodeServerJS is the universal HTTP adapter injected into Node.js Kubernetes deployments.
//
//go:embed nodejs/server.js
var NodeServerJS string
