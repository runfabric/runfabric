package locking

type LockRecord struct {
	Service         string `json:"service"`
	Stage           string `json:"stage"`
	Operation       string `json:"operation"`
	OwnerToken      string `json:"ownerToken"`
	PID             int    `json:"pid"`
	CreatedAt       string `json:"createdAt"`
	ExpiresAt       string `json:"expiresAt"`
	LastHeartbeatAt string `json:"lastHeartbeatAt,omitempty"`
}
