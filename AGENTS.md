# PerSSH (TunnelBoard) Agent Guidelines

## Project Overview
PerSSH is a CLI SSH & Docker Environment Manager (Client-Server architecture).
- **Client**: Runs on Windows/Linux/macOS. Connects to server via SSH.
- **Server**: Runs on Linux (headless). Manages Docker containers.

## Design & Aesthetics
- **TUI**: Use `bubbletea` and `bubbles`.
- **Theme**: Minimalist Retro. Black background, Green text.
- **UX**: Hotkeys (C, L, S, Q) + Arrow navigation. Split screen (Telemetry Top, Menu Bottom).

## Architecture Principles
1.  **Isolation**: Prevent host pollution. Use Docker for everything.
2.  **Telemetry**: Only active when Client is connected.
3.  **Persistence**: 
    - Client stores connection settings in `client.ini` and passwords in OS keyring.
    - Server stores environment state in Docker Labels and local JSON metadata.
4.  **No Hard Crashes**: Log errors to `system.log` and recover gracefully.
5.  **Extensibility**: Use Interface pattern for Environment Modules (Standard, Minecraft, etc.).

## Coding Standards
- **Go Version**: 1.21+
- **Comments**: Heavy comments explaining logic for future expandability.
- **Error Handling**: Return informative errors, do not just log and exit (unless fatal startup).
- **Logging**:
    - `system.log`: Internal errors/stack traces.
    - `audit.log`: User actions (e.g., "Created Environment X").
    - `test_audit.log`: Automated test results.

## Testing
- Use `--dev` flag for local mock testing (no real SSH/Docker).
- Use `--auto-test` for scripted verification scenarios.
