package recovery

type Request struct {
	Root    string
	Service string
	Stage   string
	Region  string
	Mode    Mode
}

type Result struct {
	Recovered bool              `json:"recovered"`
	Mode      string            `json:"mode"`
	Status    string            `json:"status"`
	Message   string            `json:"message,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Errors    []string          `json:"errors,omitempty"`
}
