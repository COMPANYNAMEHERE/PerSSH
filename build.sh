#!/bin/bash
export PATH=$PATH:/usr/local/go/bin:/usr/lib/go/bin:$HOME/go/bin
export GOAMD64=v1 # Ensure compatibility with older CPUs (e.g. Haswell)

# Build Script for PerSSH

# Ensure output dir
mkdir -p dist

echo "Building Agent (Linux amd64)..."
GOOS=linux GOARCH=amd64 go build -o dist/perssh-server ./cmd/perssh-server

echo "Building Client (Current OS)..."
go build -o dist/perssh-client ./cmd/perssh-client

echo "Done. Binaries in ./dist"
