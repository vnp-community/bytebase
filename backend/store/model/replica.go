package model

import "time"

type ReplicaNode struct {
	ReplicaID     string    `json:"replica_id"`
	EndpointURL   string    `json:"endpoint_url"`
	Version       string    `json:"version"`
	Status        string    `json:"status"`       // STARTING, READY, DRAINING, STOPPED, UNHEALTHY
	Capabilities  []string  `json:"capabilities"` // API, RUNNER, LSP, MCP
	Metadata      string    `json:"metadata"`     // JSONB string
	StartedAt     time.Time `json:"started_at"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
}
