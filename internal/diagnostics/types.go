package diagnostics

type CheckResult struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Backend string `json:"backend,omitempty"`
	Message string `json:"message,omitempty"`
}

type HealthReport struct {
	Service string        `json:"service"`
	Stage   string        `json:"stage"`
	Checks  []CheckResult `json:"checks"`
}
