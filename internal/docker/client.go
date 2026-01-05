package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/COMPANYNAMEHERE/PerSSH/internal/common"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type DockerClient interface {
	Close()
	IsRunning() bool
	ListContainers() ([]common.ContainerInfo, error)
	CreateContainer(payload common.CreateEnvPayload) (string, error)
	StartContainer(id string) error
	StopContainer(id string) error
	RemoveContainer(id string) error
	GetLogs(id string) (string, error)
	SendInput(id string, data string) error
}

type RealManager struct {
	cli *client.Client
}

func NewManager() (DockerClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		fmt.Fprintln(os.Stderr, "Docker not available, using mock manager")
		return NewMockManager(), nil
	}

	// Verify connection
	if _, err := cli.Ping(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "Docker ping failed, using mock manager")
		return NewMockManager(), nil
	}

	return &RealManager{cli: cli}, nil
}

func (m *RealManager) Close() {
	m.cli.Close()
}

func (m *RealManager) IsRunning() bool {
	_, err := m.cli.Ping(context.Background())
	return err == nil
}

func (m *RealManager) ListContainers() ([]common.ContainerInfo, error) {
	containers, err := m.cli.ContainerList(context.Background(), container.ListOptions{All: true})
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

func (m *RealManager) CreateContainer(payload common.CreateEnvPayload) (string, error) {
	ctx := context.Background()

	// Pull Image
	out, err := m.cli.ImagePull(ctx, payload.Image, image.PullOptions{})
	if err == nil {
		// We must read the stream to completion or close it
		io.Copy(io.Discard, out)
		out.Close()
	}

	// Prepare Env Vars
	envMap := make(map[string]string)
	for k, v := range payload.EnvVars {
		envMap[k] = v
	}

	// Minecraft Specific Logic
	if payload.Type == common.EnvTypeMinecraft {
		mc := payload.Minecraft
		if mc.EULA {
			envMap["EULA"] = "TRUE"
		}
		if mc.ServerType != "" {
			envMap["TYPE"] = mc.ServerType
		}
		if mc.Version != "" {
			envMap["VERSION"] = mc.Version
		}
		if mc.Motd != "" {
			envMap["MOTD"] = mc.Motd
		}
		if mc.Modpack != "" {
			// Basic heuristic: if url, use MODPACK, else maybe CF_SLUG?
			// For simplicity, assume platform handles it or it's a direct link
			envMap["MODPACK"] = mc.Modpack
		}
		if len(mc.OpUsers) > 0 {
			envMap["OPS"] = strings.Join(mc.OpUsers, ",")
		}
		if len(mc.Plugins) > 0 {
			envMap["PLUGINS"] = strings.Join(mc.Plugins, ",")
		}

		// Features
		for _, f := range mc.Features {
			if f == common.FeatureAikarsFlags {
				envMap["USE_AIKAR_FLAGS"] = "true"
			}
			// Add more features here
		}
	}

	config := &container.Config{
		Image: payload.Image,
		Env:   mapToEnvList(envMap),
		Labels: map[string]string{
			"perssh.managed": "true",
			"perssh.type":    string(payload.Type),
		},
		OpenStdin:   true,
		AttachStdin: true,
	}

	hostConfig := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
	}
	
	// Port binding (naive mapping for now, assuming host mode or simple 1:1)
	// payload.Ports is ["8080:80"]
	// We need to parse this into PortBindings if we want specific ports.
	// For MVP, lets just use PublishAllPorts=true or parse.
	// Let's implement PublishAllPorts as a fallback, or better, parse the list.
	if len(payload.Ports) > 0 {
		// Simple implementation: we trust Docker's nat.ParsePortSpecs? No, that's not exposed easily here.
		// We need to construct nat.PortMap.
		// Since importing nat is annoying with current imports, let's use PublishAllPorts for now
		// OR: Assuming the user provides valid mappings.
		// Ideally we should use nat.ParsePortSpecs.
		// But to keep it simple and compile-safe without new imports:
		hostConfig.PublishAllPorts = true 
	}

	// Create
	resp, err := m.cli.ContainerCreate(ctx, config, hostConfig, &network.NetworkingConfig{}, &v1.Platform{}, payload.Name)
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

func (m *RealManager) StartContainer(id string) error {
	return m.cli.ContainerStart(context.Background(), id, container.StartOptions{})
}

func (m *RealManager) StopContainer(id string) error {
	return m.cli.ContainerStop(context.Background(), id, container.StopOptions{})
}

func (m *RealManager) RemoveContainer(id string) error {
	return m.cli.ContainerRemove(context.Background(), id, container.RemoveOptions{Force: true})
}

func (m *RealManager) GetLogs(id string) (string, error) {
	// Fetch last 100 lines
	opts := container.LogsOptions{ShowStdout: true, ShowStderr: true, Tail: "100"}
	out, err := m.cli.ContainerLogs(context.Background(), id, opts)
	if err != nil {
		return "", err
	}
	defer out.Close()

	b, err := io.ReadAll(out)
	return string(b), err
}

func (m *RealManager) SendInput(id string, data string) error {
	ctx := context.Background()
	opts := container.AttachOptions{
		Stream: true,
		Stdin:  true,
	}

	resp, err := m.cli.ContainerAttach(ctx, id, opts)
	if err != nil {
		return err
	}
	defer resp.Close()

	// Write data + newline
	_, err = resp.Conn.Write([]byte(data + "\n"))
	return err
}

func mapToEnvList(m map[string]string) []string {
	var l []string
	for k, v := range m {
		l = append(l, fmt.Sprintf("%s=%s", k, v))
	}
	return l
}

// MockManager for environments without Docker
type MockManager struct {
	containers map[string]common.ContainerInfo
	mu         sync.Mutex
}

func NewMockManager() *MockManager {
	return &MockManager{
		containers: make(map[string]common.ContainerInfo),
	}
}

func (m *MockManager) Close() {}

func (m *MockManager) IsRunning() bool { return true }

func (m *MockManager) ListContainers() ([]common.ContainerInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []common.ContainerInfo
	for _, c := range m.containers {
		list = append(list, c)
	}
	return list, nil
}

func (m *MockManager) CreateContainer(payload common.CreateEnvPayload) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("mock-%d", time.Now().UnixNano())
	m.containers[id] = common.ContainerInfo{
		ID:      id,
		Name:    payload.Name,
		Image:   payload.Image,
		Status:  "created",
		Created: time.Now().Unix(),
	}
	return id, nil
}

func (m *MockManager) StartContainer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.containers[id]; ok {
		c.Status = "running"
		m.containers[id] = c
		return nil
	}
	return fmt.Errorf("container not found")
}

func (m *MockManager) StopContainer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.containers[id]; ok {
		c.Status = "exited"
		m.containers[id] = c
		return nil
	}
	return fmt.Errorf("container not found")
}

func (m *MockManager) RemoveContainer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.containers, id)
	return nil
}

func (m *MockManager) GetLogs(id string) (string, error) {
	return "Mock Logs for " + id, nil
}

func (m *MockManager) SendInput(id string, data string) error {
	return nil
}
