package ssh

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/COMPANYNAMEHERE/PerSSH/internal/common"
)

// LocalMockClient runs the agent locally for testing purposes.
type LocalMockClient struct {
	Stdin  io.WriteCloser
	Stdout io.Reader
	Cmd    *exec.Cmd
}

func NewLocalMockClient() *LocalMockClient {
	return &LocalMockClient{}
}

func (c *LocalMockClient) Connect() error {
	return nil // Local dev mode doesn't need SSH connection
}

func (c *LocalMockClient) Close() {
	if c.Cmd != nil && c.Cmd.Process != nil {
		c.Cmd.Process.Kill()
	}
}

func (c *LocalMockClient) DeployAgent(localBinaryPath string) error {
	return nil // Already local
}

func (c *LocalMockClient) StartAgent() error {
	exe, _ := os.Executable()
	binPath := filepath.Join(filepath.Dir(exe), "perssh-server")
	
	// If running via 'go run', perssh-server might be in 'dist' or current dir
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		binPath = "./dist/perssh-server"
	}
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		binPath = "./perssh-server"
	}

	cmd := exec.Command(binPath)
	
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	c.Stdin = stdin

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	c.Stdout = stdout

	// For local debugging, pipe stderr to client's stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start local server %s: %w", binPath, err)
	}
	c.Cmd = cmd
	
	return nil
}

func (c *LocalMockClient) SendRequest(req common.Request) error {
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = c.Stdin.Write(b)
	return err
}

func (c *LocalMockClient) GetStdout() io.Reader {
	return c.Stdout
}
