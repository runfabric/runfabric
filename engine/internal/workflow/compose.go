package workflow

import "strings"

// ServiceBindingEnv returns environment variables for service binding: for each key "api" with value "https://..."
// it sets SERVICE_API_URL=https://... (uppercase key with _URL suffix). Used when deploying compose so that
// service B can call service A via env SERVICE_A_URL.
func ServiceBindingEnv(serviceURLs map[string]string) map[string]string {
	out := make(map[string]string)
	for name, url := range serviceURLs {
		if name == "" || url == "" {
			continue
		}
		key := "SERVICE_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_")) + "_URL"
		out[key] = url
	}
	return out
}
