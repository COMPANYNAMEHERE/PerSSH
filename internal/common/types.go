package common

import "time"

// CommandType defines the type of RPC command.
type CommandType string

const (
	CmdPing           CommandType = "PING"
	CmdGetTelemetry   CommandType = "GET_TELEMETRY"
	CmdListContainers CommandType = "LIST_CONTAINERS"
	CmdCreateEnv      CommandType = "CREATE_ENV"
	CmdStartEnv       CommandType = "START_ENV"
	CmdStopEnv        CommandType = "STOP_ENV"
	CmdRemoveEnv      CommandType = "REMOVE_ENV"
	CmdGetLogs        CommandType = "GET_LOGS"
	CmdSendInput      CommandType = "SEND_INPUT"
)

// Request is the generic RPC request structure sent from Client to Server.
type Request struct {
	ID      string          `json:"id"`
	Type    CommandType     `json:"type"`
	Payload interface{}     `json:"payload,omitempty"`
}

// Response is the generic RPC response structure sent from Server to Client.
type Response struct {
	ID      string      `json:"id"` // Matches Request ID
	Success bool        `json:"success"`
	Error   string      `json:"error,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// TelemetryData holds system stats.
type TelemetryData struct {
	Timestamp     time.Time `json:"timestamp"`
	CPUUsage      float64   `json:"cpu_usage"`      // Percentage
	CPUTemp       float64   `json:"cpu_temp"`       // Celsius
	RAMUsage      float64   `json:"ram_usage"`      // Percentage
	RAMTotal      uint64    `json:"ram_total"`      // Bytes
	RAMUsed       uint64    `json:"ram_used"`       // Bytes
	DiskFree      uint64    `json:"disk_free"`      // Bytes
	DiskTotal     uint64    `json:"disk_total"`     // Bytes
	DockerRunning bool      `json:"docker_running"` // Is daemon active?
}

// EnvironmentType defines the template used.
type EnvironmentType string

const (
	EnvTypeStandard  EnvironmentType = "STANDARD"
	EnvTypeMinecraft EnvironmentType = "MINECRAFT"
)

// MinecraftConfig holds specific settings for Minecraft environments.
type MinecraftConfig struct {
	EULA       bool     `json:"eula"`
	ServerType string   `json:"server_type"` // VANILLA, FORGE, FABRIC, ARCLIGHT
	Version    string   `json:"version"`     // "latest", "1.20.4"
	Modpack    string   `json:"modpack"`     // URL or ID (CurseForge/Modrinth)
	Motd       string   `json:"motd"`
	Features   []string `json:"features"` // e.g., "AIKAR_FLAGS", "AUTO_UPDATE"
	Plugins    []string `json:"plugins"`  // List of URLs
	OpUsers    []string `json:"op_users"` // List of usernames
}

const (
	FeatureAikarsFlags = "AIKAR_FLAGS"
	FeatureAutoUpdate  = "AUTO_UPDATE"
)

// CreateEnvPayload defines parameters for creating a new environment.
type CreateEnvPayload struct {
	Name        string            `json:"name"`
	Type        EnvironmentType   `json:"type"`
	Image       string            `json:"image"` // For Standard
	Ports       []string          `json:"ports"` // "8080:80"
	EnvVars     map[string]string `json:"env_vars"`
	RamLimit    string            `json:"ram_limit,omitempty"` // e.g., "2g"
	
	// Configuration
	Minecraft MinecraftConfig `json:"minecraft,omitempty"`
}

// ContainerInfo describes a running environment.
type ContainerInfo struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	Status  string            `json:"status"` // "running", "exited", etc.
	Created int64             `json:"created"`
	Labels  map[string]string `json:"labels"`
}
