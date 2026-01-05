package modules

import (
	"github.com/COMPANYNAMEHERE/PerSSH/internal/common"
)

// Module defines the behavior of a specific environment type.
type Module interface {
	// Name returns the display name (e.g., "Minecraft Server").
	Name() string
	
	// Type returns the enum type.
	Type() common.EnvironmentType
	
	// GetDefaults returns default creation payload.
	GetDefaults() common.CreateEnvPayload
	
	// ParseLogs allows custom parsing of logs (e.g., for telemetry).
	ParseLogs(logs string) map[string]interface{}
}

// StandardModule implementation.
type StandardModule struct{}

func (m *StandardModule) Name() string { return "Standard Docker Container" }
func (m *StandardModule) Type() common.EnvironmentType { return common.EnvTypeStandard }
func (m *StandardModule) GetDefaults() common.CreateEnvPayload {
	return common.CreateEnvPayload{
		Type: common.EnvTypeStandard,
		Image: "ubuntu:latest",
	}
}
func (m *StandardModule) ParseLogs(logs string) map[string]interface{} {
	return nil // No specific parsing
}

// MinecraftModule implementation.
type MinecraftModule struct{}

func (m *MinecraftModule) Name() string { return "Minecraft Server (Java)" }
func (m *MinecraftModule) Type() common.EnvironmentType { return common.EnvTypeMinecraft }
func (m *MinecraftModule) GetDefaults() common.CreateEnvPayload {
	return common.CreateEnvPayload{
		Type: common.EnvTypeMinecraft,
		Image: "itzg/minecraft-server", // Popular image
		Ports: []string{"25565:25565"},
		Minecraft: common.MinecraftConfig{
			EULA:       true,
			ServerType: "VANILLA",
			Version:    "latest",
			Motd:       "A PerSSH Managed Server",
			Features:   []string{common.FeatureAikarsFlags},
		},
	}
}
func (m *MinecraftModule) ParseLogs(logs string) map[string]interface{} {
	// Here we would regex for "joined the game" etc.
	return map[string]interface{}{
		"status": "Running",
	}
}

// Registry to hold modules
var Registry = []Module{
	&StandardModule{},
	&MinecraftModule{},
}
