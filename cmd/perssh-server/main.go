package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/COMPANYNAMEHERE/PerSSH/internal/common"
	"github.com/COMPANYNAMEHERE/PerSSH/internal/docker"
	"github.com/COMPANYNAMEHERE/PerSSH/internal/sysinfo"
)

func main() {
	listenAddr := flag.String("listen", "", "Address to listen on (e.g. :8080)")
	flag.Parse()

	// Initialize Docker Manager
	dm, err := docker.NewManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to Docker: %v\n", err)
		os.Exit(1)
	}
	defer dm.Close()

	if *listenAddr != "" {
		// Server Mode
		fmt.Printf("PerSSH Server starting...\n")
		ln, err := net.Listen("tcp", *listenAddr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to listen on %s: %v\n", *listenAddr, err)
			os.Exit(1)
		}
		fmt.Printf("✅ Listening on %s\n", *listenAddr)
		fmt.Printf("   (Press Ctrl+C to stop)\n")

		for {
			conn, err := ln.Accept()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Accept error: %v\n", err)
				continue
			}
			fmt.Printf("➕ New connection from %s\n", conn.RemoteAddr())
			go func(c net.Conn) {
				defer c.Close()
				defer fmt.Printf("➖ Connection closed from %s\n", c.RemoteAddr())
				processLoop(c, c, dm)
			}(conn)
		}
	} else {
		// Standard Mode (Stdin/Stdout)
		
		// Check if running in a terminal (interactive mode)
		if stat, _ := os.Stdin.Stat(); (stat.Mode() & os.ModeCharDevice) != 0 {
			fmt.Println("⚠️  PerSSH Agent started in Standard I/O Mode (Interactive).")
			fmt.Println("   This mode is intended for SSH tunneling or internal use.")
			fmt.Println("   If you want to run the SERVER to accept connections, use:")
			fmt.Println("       ./perssh-server -listen :8080")
			fmt.Println("")
			fmt.Println("   Waiting for JSON requests on Stdin...")
		}

		processLoop(os.Stdin, os.Stdout, dm)
	}
}

func processLoop(r io.Reader, w io.Writer, dm docker.DockerClient) {
	decoder := json.NewDecoder(r)
	encoder := json.NewEncoder(w)

	for {
		var req common.Request
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				break
			}
			// If JSON is malformed, we lose synchronization.
			// Log it and stop.
			fmt.Fprintf(os.Stderr, "Decode error: %v\n", err)
			sendErrorTo(w, "DECODE", "Failed to decode request: "+err.Error())
			break
		}

		fmt.Fprintf(os.Stderr, "Received Request: ID=%s Type=%s\n", req.ID, req.Type)

		resp := handleRequest(req, dm)
		
		if resp.Success {
			fmt.Fprintf(os.Stderr, "Sending Success Response: ID=%s\n", resp.ID)
		} else {
			fmt.Fprintf(os.Stderr, "Sending Error Response: ID=%s Error=%s\n", resp.ID, resp.Error)
		}

		if err := encoder.Encode(resp); err != nil {
			// Write failed, stop processing for this connection
			fmt.Fprintf(os.Stderr, "Encode error: %v\n", err)
			break
		}
	}
}

func handleRequest(req common.Request, dm docker.DockerClient) common.Response {
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
		b, err := json.Marshal(req.Payload)
		if err != nil {
			resp.Success = false
			resp.Error = "Invalid payload: " + err.Error()
		} else {
			var payload common.CreateEnvPayload
			if err := json.Unmarshal(b, &payload); err != nil {
				resp.Success = false
				resp.Error = "Failed to parse creation payload: " + err.Error()
			} else {
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
			}
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

	case common.CmdSendInput:
		// Payload: {"id": "...", "data": "..."}
		// But since Payload is generic interface{}, we need to structure it or just expect a map?
		// For simplicity, let's assume Payload is a map or struct.
		// Actually, req.Payload is unmarshaled into map[string]interface{} by default.
		
		if m, ok := req.Payload.(map[string]interface{}); ok {
			id, _ := m["id"].(string)
			data, _ := m["data"].(string)
			
			fmt.Fprintf(os.Stderr, "Sending input to %s: %q\n", id, data)

			if id == "" {
				resp.Success = false
				resp.Error = "Missing container ID"
			} else {
				if err := dm.SendInput(id, data); err != nil {
					resp.Success = false
					resp.Error = err.Error()
				}
			}
		} else {
			resp.Success = false
			resp.Error = "Invalid payload format for SEND_INPUT"
		}

	default:
		resp.Success = false
		resp.Error = "Unknown command: " + string(req.Type)
	}

	return resp
}

func sendError(id, msg string) {
	sendErrorTo(os.Stdout, id, msg)
}

func sendErrorTo(w io.Writer, id, msg string) {
	resp := common.Response{
		ID:      id,
		Success: false,
		Error:   msg,
	}
	json.NewEncoder(w).Encode(resp)
}