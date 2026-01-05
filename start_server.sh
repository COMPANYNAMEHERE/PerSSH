#!/bin/bash
# Script to launch perssh-server properly

# Get the directory where the script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

# Path to the server executable
SERVER_BIN="$SCRIPT_DIR/dist/perssh-server"

# Log file
LOG_FILE="/tmp/perssh-server.log"

echo "Starting PerSSH Server..."
echo "  Bin: $SERVER_BIN"
echo "  Log: $LOG_FILE"

# Check if binary exists
if [ ! -f "$SERVER_BIN" ]; then
    echo "Error: Server binary not found at $SERVER_BIN"
    echo "Please run ./build.sh first."
    exit 1
fi

# Stop any existing instance
pkill -f perssh-server
echo "  Stopped previous instances."

# Start the server in the background
nohup "$SERVER_BIN" -listen :8080 > "$LOG_FILE" 2>&1 &
PID=$!

echo "Server started with PID $PID"
echo "Listening on 0.0.0.0:8080"
echo "You can view logs with: tail -f $LOG_FILE"
