package main

import (
	"encoding/json"
	"io"
	"os"

	"github.com/COMPANYNAMEHERE/PerSSH/internal/common"
	"github.com/COMPANYNAMEHERE/PerSSH/internal/docker"
	"github.com/COMPANYNAMEHERE/PerSSH/internal/sysinfo"
)

func main() {
	// Initialize Docker Manager
	dm, err := docker.NewManager()
	if err != nil {
		sendError("GLOBAL", "Failed to connect to Docker: "+err.Error())
		os.Exit(1)
	}
	defer dm.Close()

	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		var req common.Request
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				break
			}
			sendError("DECODE", "Failed to decode request: "+err.Error())
			continue
		}

		resp := handleRequest(req, dm)
		if err := encoder.Encode(resp); err != nil {
			// This is fatal for the communication but we can log it if we had a log file
			continue
		}
	}
}

func handleRequest(req common.Request, dm *docker.Manager) common.Response {
	resp := common.Response{
		ID:      req.ID,
		Success: true,
	}

	switch req.Type {
	case common.CmdPing:
		resp.Data = "PONG"

	case common.CmdGetTelemetry:
		stats, err := sysinfo.GetTelemetry()
		if err != nil {
			resp.Success = false
			resp.Error = err.Error()
		} else {
			stats.DockerRunning = dm.IsRunning()
			resp.Data = stats
		}

	case common.CmdListContainers:
		list, err := dm.ListContainers()
		if err != nil {
			resp.Success = false
			resp.Error = err.Error()
		} else {
			resp.Data = list
		}

	case common.CmdCreateEnv:
		// Convert Payload to CreateEnvPayload
		b, _ := json.Marshal(req.Payload)
		var payload common.CreateEnvPayload
		json.Unmarshal(b, &payload)

		id, err := dm.CreateContainer(payload)
		if err != nil {
			resp.Success = false
			resp.Error = err.Error()
		} else {
			// Auto start
			if err := dm.StartContainer(id); err != nil {
				resp.Error = "Container created but failed to start: " + err.Error()
			}
			resp.Data = id
		}

	case common.CmdStartEnv:
		id, ok := req.Payload.(string)
		if !ok {
			resp.Success = false
			resp.Error = "Payload must be a string (container ID)"
		} else if err := dm.StartContainer(id); err != nil {
			resp.Success = false
			resp.Error = err.Error()
		}

	case common.CmdStopEnv:
		id, ok := req.Payload.(string)
		if !ok {
			resp.Success = false
			resp.Error = "Payload must be a string (container ID)"
		} else if err := dm.StopContainer(id); err != nil {
			resp.Success = false
			resp.Error = err.Error()
		}

	case common.CmdRemoveEnv:
		id, ok := req.Payload.(string)
		if !ok {
			resp.Success = false
			resp.Error = "Payload must be a string (container ID)"
		} else if err := dm.RemoveContainer(id); err != nil {
			resp.Success = false
			resp.Error = err.Error()
		}

	case common.CmdGetLogs:
		id, ok := req.Payload.(string)
		if !ok {
			resp.Success = false
			resp.Error = "Payload must be a string (container ID)"
		} else {
			logs, err := dm.GetLogs(id)
			if err != nil {
				resp.Success = false
				resp.Error = err.Error()
			} else {
				resp.Data = logs
			}
		}

	default:
		resp.Success = false
		resp.Error = "Unknown command: " + string(req.Type)
	}

	return resp
}

func sendError(id, msg string) {
	resp := common.Response{
		ID:      id,
		Success: false,
		Error:   msg,
	}
	json.NewEncoder(os.Stdout).Encode(resp)
}
