package aws

import "strings"

type stepFunctionDecl struct {
	Name       string
	Role       string
	Bindings   map[string]string
	Definition map[string]any
}

func applyStepFunctionBindings(definition string, decl stepFunctionDecl, lambdaARNByFunction map[string]string) string {
	if strings.TrimSpace(definition) == "" || len(decl.Bindings) == 0 {
		return definition
	}
	replacerArgs := make([]string, 0, len(decl.Bindings)*8)
	for key, ref := range decl.Bindings {
		resolved := resolveBindingValue(ref, lambdaARNByFunction)
		if resolved == "" {
			continue
		}
		replacerArgs = append(replacerArgs,
			"${bindings."+key+"}", resolved,
			"${binding."+key+"}", resolved,
			"{{bindings."+key+"}}", resolved,
			"{{binding."+key+"}}", resolved,
		)
	}
	if len(replacerArgs) == 0 {
		return definition
	}
	return strings.NewReplacer(replacerArgs...).Replace(definition)
}

func resolveBindingValue(ref string, lambdaARNByFunction map[string]string) string {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "arn:") {
		return trimmed
	}
	if arn := strings.TrimSpace(lambdaARNByFunction[trimmed]); arn != "" {
		return arn
	}
	return trimmed
}
