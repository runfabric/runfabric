// Package common provides shared runtime types and helpers for runtimes/node and runtimes/python.
package common

// Runtime identifies a language runtime (node, python, etc.).
type Runtime string

const (
	RuntimeNode   Runtime = "node"
	RuntimePython Runtime = "python"
)
