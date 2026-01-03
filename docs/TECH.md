# PerSSH Technical Documentation

## Architecture

PerSSH operates on a Client-Server model where the "Server" is an ephemeral agent running on the target machine, tunneling commands via SSH.

### 1. Connection Flow
1.  **Authentication**: The Client (`perssh-client`) uses standard SSH keys or passwords to authenticate with the target Linux host.
2.  **Deployment**: Upon connection, the Client checks if `perssh-server` exists on the remote host. If not, it uploads the binary via SFTP.
3.  **Execution**: The Client executes `./perssh-server` on the remote host. It captures `Stdin` and `Stdout` of this process.

### 2. RPC Protocol (JSON over Stdin/Stdout)
The communication is strictly JSON-based.
- **Request**: `{ "id": "uuid", "type": "COMMAND_TYPE", "payload": { ... } }`
- **Response**: `{ "id": "uuid", "success": true, "data": { ... } }`

This allows the agent to be stateless and simple. The agent runs a loop reading JSON lines from Stdin and writing JSON lines to Stdout.

### 3. Docker Management
The Agent uses the official Docker SDK to talk to the local Docker socket (`/var/run/docker.sock`).
- **Persistence**: Environment metadata (Name, Type) is stored in Docker Labels (`perssh.managed=true`, `perssh.type=MINECRAFT`). This ensures that even if the Agent is killed, the state is recovered from Docker itself upon reconnection.

### 4. Modules
The Client uses an Interface pattern for Modules.
- **Standard**: Generic image runner.
- **Minecraft**: Preset configuration (Ports 25565, EULA=TRUE env var).

### 5. Telemetry
The Agent reads `/proc` and `/sys` (via `gopsutil`) only when requested (`CMD_GET_TELEMETRY`). This minimizes resource usage when the dashboard is not active.
