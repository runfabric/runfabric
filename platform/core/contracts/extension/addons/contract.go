// Package addons defines the addon contract (AddonResult, AddonApplyInput) for RunFabric Addons.
// See docs/ADDON_CONTRACT.md for the full TypeScript Addon interface.
package addons

// AddonResult is the return shape of Addon.apply() (TypeScript AddonResult).
type AddonResult struct {
	Env             map[string]string  `json:"env,omitempty"`
	Files           []AddonResultFile  `json:"files,omitempty"`
	Patches         []AddonResultPatch `json:"patches,omitempty"`
	HandlerWrappers []string           `json:"handlerWrappers,omitempty"`
	BuildSteps      []string           `json:"buildSteps,omitempty"`
	Warnings        []string           `json:"warnings,omitempty"`
}

// AddonResultFile is one generated file (path + content).
type AddonResultFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// AddonResultPatch is one text patch (path, find, replace).
type AddonResultPatch struct {
	Path    string `json:"path"`
	Find    string `json:"find"`
	Replace string `json:"replace"`
}

// AddonApplyInput is the input to Addon.apply() for validation or serialization.
type AddonApplyInput struct {
	FunctionName   string `json:"functionName"`
	FunctionConfig any    `json:"functionConfig,omitempty"`
	AddonConfig    any    `json:"addonConfig,omitempty"`
	ProjectRoot    string `json:"projectRoot"`
	BuildDir       string `json:"buildDir"`
	GeneratedDir   string `json:"generatedDir"`
}
