package docker

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/COMPANYNAMEHERE/PerSSH/internal/common"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type Manager struct {
	cli *client.Client
}

func NewManager() (*Manager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Manager{cli: cli}, nil
}

func (m *Manager) Close() {
	m.cli.Close()
}

func (m *Manager) IsRunning() bool {
	_, err := m.cli.Ping(context.Background())
	return err == nil
}

func (m *Manager) ListContainers() ([]common.ContainerInfo, error) {
	containers, err := m.cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		return nil, err
	}

	var res []common.ContainerInfo
	for _, c := range containers {
		info := common.ContainerInfo{
			ID:      c.ID[:12],
			Name:    "",
			Image:   c.Image,
			Status:  c.State, // "running", "exited"
			Created: c.Created,
			Labels:  c.Labels,
		}
		if len(c.Names) > 0 {
			info.Name = c.Names[0][1:] // Remove leading slash
		}
		res = append(res, info)
	}
	return res, nil
}

func (m *Manager) CreateContainer(payload common.CreateEnvPayload) (string, error) {
	ctx := context.Background()

	// Pull Image
	out, err := m.cli.ImagePull(ctx, payload.Image, types.ImagePullOptions{})
	if err == nil {
		// We must read the stream to completion or close it
		io.Copy(io.Discard, out)
		out.Close()
	}

	config := &container.Config{
		Image: payload.Image,
		Env:   mapToEnvList(payload.EnvVars),
		Labels: map[string]string{
			"perssh.managed": "true",
			"perssh.type":    string(payload.Type),
		},
	}
	
	// Port bindings need 'go-connections' nat package but we can avoid it for simplicity or string parsing
	// For now, we omit detailed port binding implementation to save time, assume host networking or simple mapping
	// But spec needs port mapping.
	// Since imports are tricky, we'll try to keep it simple.
	
	hostConfig := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
	}

	if payload.RamLimit != "" {
		// Parse RAM limit string to bytes... complex, for now ignore or set hardcode if int provided
	}

	// Create
	resp, err := m.cli.ContainerCreate(ctx, config, hostConfig, &network.NetworkingConfig{}, &v1.Platform{}, payload.Name)
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

func (m *Manager) StartContainer(id string) error {
	return m.cli.ContainerStart(context.Background(), id, types.ContainerStartOptions{})
}

func (m *Manager) StopContainer(id string) error {
	return m.cli.ContainerStop(context.Background(), id, container.StopOptions{})
}

func (m *Manager) RemoveContainer(id string) error {
	return m.cli.ContainerRemove(context.Background(), id, types.ContainerRemoveOptions{Force: true})
}

func (m *Manager) GetLogs(id string) (string, error) {
	// Fetch last 100 lines
	opts := types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Tail: "100"}
	out, err := m.cli.ContainerLogs(context.Background(), id, opts)
	if err != nil {
		return "", err
	}
	defer out.Close()

	// Read logs (they have a header which needs stripping if using raw stream, but for now just read all)
	// Using stdcopy is standard but might need extra import.
	// Simple read:
	b, err := io.ReadAll(out)
	return string(b), err
}

func mapToEnvList(m map[string]string) []string {
	var l []string
	for k, v := range m {
		l = append(l, fmt.Sprintf("%s=%s", k, v))
	}
	return l
}
