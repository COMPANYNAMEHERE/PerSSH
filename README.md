# PerSSH

CLI SSH & Docker Environment Manager (TunnelBoard).

## Installation

### Prerequisites
- Go 1.21+
- Docker (on the server)

### Build
Run the build script:
```bash
./build.sh
```
This produces `dist/perssh-client` and `dist/perssh-server`.

## Usage

1.  Run the client:
    ```bash
    ./dist/perssh-client
    ```
2.  Enter SSH details (Host IP, User, Password/Key).
3.  The client will automatically deploy the agent to the server.
4.  **Dashboard Controls**:
    - `C`: Create a new environment (Docker Container).
    - `L`: List/Refresh environments.
    - `Q`: Quit.

## Dev Mode
To test locally without a remote server, you can modify the code to mock the SSH connection (implementation details in `internal/ssh/mock.go` - *Note: Mocking currently requires code adjustment in `tui/model.go` to use mock client*).
