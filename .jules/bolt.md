## 2025-05-22 - [Optimized TUI Polling Frequency]
**Learning:** In Bubble Tea applications, using a single `tickMsg` for multiple data sources forces them to sync to the fastest frequency. Decoupling them into separate message types (`telemetryTickMsg`, `listTickMsg`) allows for independent and appropriate polling intervals (e.g., 2s vs 6s).
**Action:** When adding periodic tasks to a TUI, always create a distinct message type for each task to allow independent frequency control.
