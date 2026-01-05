package main

import (
	"encoding/json"
	"testing"

	"github.com/COMPANYNAMEHERE/PerSSH/internal/common"
	"github.com/COMPANYNAMEHERE/PerSSH/internal/docker"
)

func TestHandlePing(t *testing.T) {
	// We don't need a real docker manager for PING
	dm := docker.NewMockManager() 

	req := common.Request{
		ID:   "test-1",
		Type: common.CmdPing,
	}

	resp := handleRequest(req, dm)

	if resp.ID != "test-1" {
		t.Errorf("Expected ID test-1, got %s", resp.ID)
	}

	if !resp.Success {
		t.Errorf("Expected success, got error: %s", resp.Error)
	}

	if resp.Data != "PONG" {
		t.Errorf("Expected PONG, got %v", resp.Data)
	}
}

func TestHandleUnknownCommand(t *testing.T) {
	dm := docker.NewMockManager() 

	req := common.Request{
		ID:   "test-2",
		Type: "GHOST_CMD",
	}

	resp := handleRequest(req, dm)

	if resp.Success {
		t.Error("Expected failure for unknown command, but got success")
	}

	if resp.Error == "" {
		t.Error("Expected error message, but got empty string")
	}
}

func TestJSONEncoding(t *testing.T) {
	// Test that our structures encode/decode as expected for the protocol
	req := common.Request{
		ID:   "123",
		Type: common.CmdPing,
	}

	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	var req2 common.Request
	err = json.Unmarshal(b, &req2)
	if err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}

	if req2.ID != req.ID || req2.Type != req.Type {
		t.Errorf("Mismatch after roundtrip: %+v vs %+v", req, req2)
	}
}
