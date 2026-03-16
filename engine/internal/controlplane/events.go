package controlplane

type Event struct {
	Type    string
	Service string
	Stage   string
	Message string
}
