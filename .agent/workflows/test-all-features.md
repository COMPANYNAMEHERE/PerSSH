---
description: Comprehensive testing workflow for PerSSH
---

# PerSSH Testing Workflow

This workflow ensures that all components of PerSSH are functioning correctly using a combination of automated unit tests and manual TUI verification in Dev mode.

## 1. Automated Unit Tests
Run the Go test suite to verify internal logic.
// turbo
```bash
go test ./...
```

## 2. Build Binaries
Ensure the project compiles correctly.
// turbo
```bash
./build.sh
```

## 3. Verify Local Dev Mode
The Dev mode allows testing the TUI without a remote server by running a local instance of the agent via pipes.

1. Launch the client in Dev mode:
```bash
./dist/perssh-client --dev
```

2. **Login Verification**:
   - In Dev mode, any credentials will work.
   - Press **Enter** to "Connect".
   - Verify it transitions to the **Dashboard**.

3. **Dashboard Verification**:
   - Check if **Telemetry stats** (CPU/RAM) are updating at the top.
   - Press **L** to refresh the container list.
   - Verify that local Docker containers (if any) are displayed.

4. **Environment Creation Verification**:
   - Press **C** to enter the creation screen.
   - Toggle between **Standard** and **Minecraft** types using **Right Arrow**.
   - Input a name and press **Enter**.
   - Verify it returns to the Dashboard and shows the new container (if Docker is running locally).

5. **Cleanup**:
   - Press **Q** or **Ctrl+C** to exit.
   - Check `system.log` and `audit.log` for correct entries.
