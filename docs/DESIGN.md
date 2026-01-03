# Design Document

## Goals
- **Isolation**: Every environment is a Docker container.
- **Portability**: Single binary client, auto-deploying agent.
- **Aesthetics**: Retro Terminal UI.

## Components

### Client
- **TUI**: Bubbletea framework.
- **SSH**: `x/crypto/ssh`.
- **Config**: `client.ini` for preferences.

### Agent
- **Docker**: `moby/client`.
- **SysInfo**: `gopsutil`.

## Data Flow
User Input (TUI) -> SSH Channel (JSON) -> Agent -> Docker Socket -> Container.
Telemetry -> Agent -> SSH Channel (JSON) -> TUI -> View.
