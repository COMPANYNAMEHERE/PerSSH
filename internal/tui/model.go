package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/COMPANYNAMEHERE/PerSSH/internal/common"
	"github.com/COMPANYNAMEHERE/PerSSH/internal/config"
	"github.com/COMPANYNAMEHERE/PerSSH/internal/modules"
	"github.com/COMPANYNAMEHERE/PerSSH/internal/ssh"
	"github.com/COMPANYNAMEHERE/PerSSH/internal/utils"
)

// Styles
var (
	styleGreen = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
	styleDim   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleErr   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleBox   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
)

type state int

const (
	stateLogin state = iota
	stateDashboard
	stateCreateEnv
)

type Model struct {
	state        state
	width, height int
	clientConfig *config.ClientConfig
	sshClient    ssh.RemoteInterface
	rpcResp      chan common.Response
	logger       *utils.Logger

	// Login
	inputHost, inputUser, inputPort, inputPassword textinput.Model
	loginErr     string
	loginSpinner spinner.Model
	loggingIn    bool

	// Dashboard
	telemetry     common.TelemetryData
	telemetryErr  string
	containers    []common.ContainerInfo
	containerList string // Pre-rendered list for simplicity

	// Create Env
	inputName  textinput.Model
	inputType  int // Index in modules.Registry
	inputImage textinput.Model // For standard
	// Minecraft specific
	mcEula bool
	mcOp   textinput.Model
	mcRam  textinput.Model
	
	createErr string
	creating  bool
	createSpinner spinner.Model
	decoder      *json.Decoder
	DevMode      bool
	
	// Dashboard Selection
	cursor int
}

func NewModel(logger *utils.Logger) Model {
	cfg, _ := config.LoadClientConfig()
	if cfg == nil {
		cfg = config.DefaultClientConfig()
	}

	h := textinput.New(); h.Placeholder = "Host IP"; h.Focus()
	u := textinput.New(); u.Placeholder = "User"
	p := textinput.New(); p.Placeholder = "22"; p.SetValue("22")
	pw := textinput.New(); pw.Placeholder = "Password"; pw.EchoMode = textinput.EchoPassword

	s := spinner.New(); s.Spinner = spinner.Dot; s.Style = styleGreen

	nm := textinput.New(); nm.Placeholder = "Environment Name"
	img := textinput.New(); img.Placeholder = "Image (e.g. ubuntu:latest)"
	op := textinput.New(); op.Placeholder = "OP User (Minecraft)"
	ram := textinput.New(); ram.Placeholder = "RAM (e.g. 2G)"

	return Model{
		state:        stateLogin,
		clientConfig: cfg,
		logger:       logger,
		inputHost:    h, inputUser: u, inputPort: p, inputPassword: pw,
		loginSpinner: s,
		createSpinner: s,
		inputName: nm, inputImage: img, inputType: 0,
		mcOp: op, mcRam: ram,
		rpcResp:      make(chan common.Response),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.loginSpinner.Tick)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			if m.sshClient != nil {
				m.sshClient.Close()
			}
			return m, tea.Quit
		}
	case common.Response:
		// Handle RPC Responses
		if msg.ID == "telemetry" && msg.Success {
			// Try to convert map to struct (hacky for MVP since JSON unmarshals to map)
			// In production, better JSON handling needed.
			b, _ := json.Marshal(msg.Data)
			json.Unmarshal(b, &m.telemetry)
		}
		if msg.ID == "list" && msg.Success {
			// Same for containers
			b, _ := json.Marshal(msg.Data)
			var list []common.ContainerInfo
			json.Unmarshal(b, &list)
			m.containers = list
			
			// Render list
			var s strings.Builder
			for _, c := range list {
				s.WriteString(fmt.Sprintf("%s - %s [%s]\n", c.Name, c.Image, c.Status))
			}
			m.containerList = s.String()
		}
		if msg.ID == "create" {
			m.creating = false
			if msg.Success {
				m.state = stateDashboard
				m.logger.Audit("Created environment successfully")
				// Refresh list
				m.sshClient.SendRequest(common.Request{ID: "list", Type: common.CmdListContainers})
			} else {
				m.createErr = msg.Error
			}
		}
		return m, m.waitForPacket()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	// State Switch
	switch m.state {
	case stateLogin:
		return m.updateLogin(msg)
	case stateDashboard:
		return m.updateDashboard(msg)
	case stateCreateEnv:
		return m.updateCreateEnv(msg)
	}
	return m, nil
}

func (m Model) View() string {
	var s string
	switch m.state {
	case stateLogin:
		s = m.viewLogin()
	case stateDashboard:
		s = m.viewDashboard()
	case stateCreateEnv:
		s = m.viewCreateEnv()
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, s)
}

// --- Login ---
func (m Model) updateLogin(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if key, ok := msg.(tea.KeyMsg); ok {
		if key.String() == "enter" {
			m.loggingIn = true
			return m, tea.Batch(m.loginSpinner.Tick, m.cmdLogin())
		}
		// Simple tab cycle
		if key.String() == "tab" {
			if m.inputHost.Focused() { m.inputHost.Blur(); m.inputUser.Focus() } else 
			if m.inputUser.Focused() { m.inputUser.Blur(); m.inputPort.Focus() } else 
			if m.inputPort.Focused() { m.inputPort.Blur(); m.inputPassword.Focus() } else 
			{ m.inputPassword.Blur(); m.inputHost.Focus() }
			return m, textinput.Blink
		}
	}

	if m.inputHost.Focused() { m.inputHost, cmd = m.inputHost.Update(msg) }
	if m.inputUser.Focused() { m.inputUser, cmd = m.inputUser.Update(msg) }
	if m.inputPort.Focused() { m.inputPort, cmd = m.inputPort.Update(msg) }
	if m.inputPassword.Focused() { m.inputPassword, cmd = m.inputPassword.Update(msg) }

	if _, ok := msg.(loginSuccessMsg); ok {
		m.loggingIn = false
		m.sshClient = msg.(loginSuccessMsg).client
		m.state = stateDashboard
		m.decoder = json.NewDecoder(m.sshClient.GetStdout())
		
		return m, tea.Batch(m.cmdPollTelemetry(), m.cmdPollList(), m.cmdPollListTick(), m.waitForPacket())
	}
	
	if err, ok := msg.(errMsg); ok {
		m.loggingIn = false
		m.loginErr = err.error.Error()
	}

	if m.loggingIn {
		var sCmd tea.Cmd
		m.loginSpinner, sCmd = m.loginSpinner.Update(msg)
		return m, tea.Batch(cmd, sCmd)
	}

	return m, cmd
}

func (m Model) viewLogin() string {
	b := strings.Builder{}
	b.WriteString(styleGreen.Render("PerSSH Login") + "\n\n")
	b.WriteString(fmt.Sprintf("Host: %s\n", m.inputHost.View()))
	b.WriteString(fmt.Sprintf("User: %s\n", m.inputUser.View()))
	b.WriteString(fmt.Sprintf("Port: %s\n", m.inputPort.View()))
	b.WriteString(fmt.Sprintf("Pass: %s\n", m.inputPassword.View()))
	
	if m.loggingIn {
		b.WriteString(fmt.Sprintf("\n%s Connecting...", m.loginSpinner.View()))
	} else {
		b.WriteString("\n[Enter] Connect")
	}
	
	if m.loginErr != "" {
		b.WriteString(styleErr.Render("\nError: " + m.loginErr))
	}
	return styleBox.Render(b.String())
}

// --- Dashboard ---
func (m Model) updateDashboard(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "c":
			m.state = stateCreateEnv
			m.inputName.Focus()
			return m, textinput.Blink
		case "l":
			// Refresh list
			if m.sshClient != nil {
				m.sshClient.SendRequest(common.Request{ID: "list", Type: common.CmdListContainers})
			}
		case "up":
			if m.cursor > 0 { m.cursor-- }
		case "down":
			if m.cursor < len(m.containers)-1 { m.cursor++ }
		case "s":
			// Toggle Start/Stop
			if len(m.containers) > 0 && m.cursor < len(m.containers) {
				c := m.containers[m.cursor]
				cmdType := common.CmdStartEnv
				if c.Status == "running" { cmdType = common.CmdStopEnv }
				m.sshClient.SendRequest(common.Request{ID: "action", Type: cmdType, Payload: c.ID})
			}
		case "x":
			// Remove
			if len(m.containers) > 0 && m.cursor < len(m.containers) {
				c := m.containers[m.cursor]
				m.sshClient.SendRequest(common.Request{ID: "action", Type: common.CmdRemoveEnv, Payload: c.ID})
			}
		case "q":
			return m, tea.Quit
		}
	}

	// Handle telemetry tick (every 2s)
	// We want frequent updates for CPU/RAM usage.
	if _, ok := msg.(telemetryTickMsg); ok {
		return m, m.cmdPollTelemetry()
	}

	// Handle list tick (every 6s)
	// We reduce the frequency of listing containers to avoid unnecessary load,
	// as container state changes less frequently than system stats.
	if _, ok := msg.(listTickMsg); ok {
		return m, tea.Batch(m.cmdPollList(), m.cmdPollListTick())
	}

	return m, nil
}

func (m Model) viewDashboard() string {
	// Telemetry Header
	stats := fmt.Sprintf("CPU: %s | RAM: %s | Disk: %s | Temp: %.1fC", 
		styleGreen.Render(fmt.Sprintf("%.1f%%", m.telemetry.CPUUsage)),
		styleGreen.Render(fmt.Sprintf("%.1f%%", m.telemetry.RAMUsage)),
		styleGreen.Render(fmt.Sprintf("%dGB", m.telemetry.DiskFree/1024/1024/1024)),
		m.telemetry.CPUTemp,
	)
	
	// Menu
	menu := fmt.Sprintf("%s Create  %s Refresh  %s Start/Stop  %s Remove  %s Quit",
		styleGreen.Render("[C]"),
		styleGreen.Render("[L]"),
		styleGreen.Render("[S]"),
		styleGreen.Render("[X]"),
		styleGreen.Render("[Q]"),
	)
	
	// Content
	var s strings.Builder
	s.WriteString("Active Environments:\n")
	for i, c := range m.containers {
		pref := "  "
		if i == m.cursor { pref = styleGreen.Render("> ") }
		
		statusStyle := styleDim
		if c.Status == "running" { statusStyle = styleGreen }
		
		s.WriteString(fmt.Sprintf("%s%s - %s [%s]\n", pref, c.Name, c.Image, statusStyle.Render(c.Status)))
	}
	
	content := s.String()
	if len(m.containers) == 0 {
		content += "\n" + styleDim.Render("No environments running.") + "\n\n"
		content += styleGreen.Render("Press 'C' to create your first environment")
	}

	return lipgloss.JoinVertical(lipgloss.Left, 
		styleBox.Render(stats),
		styleBox.Render(content),
		menu,
	)
}

// --- Create Env ---
func (m Model) updateCreateEnv(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	
	if key, ok := msg.(tea.KeyMsg); ok {
		if key.String() == "esc" {
			m.state = stateDashboard
			return m, nil
		}
		if key.String() == "enter" {
			m.creating = true
			return m, tea.Batch(m.createSpinner.Tick, m.cmdCreate())
		}
		// Tab cycle
		if key.String() == "tab" {
			// Logic to cycle between Name -> Type -> Fields -> EULA
			if m.inputName.Focused() { m.inputName.Blur(); m.inputImage.Focus() } else // simplified
			{ m.inputImage.Blur(); m.inputName.Focus() }
			return m, textinput.Blink
		}
		// Type selection with arrows
		if key.String() == "right" {
			m.inputType = (m.inputType + 1) % len(modules.Registry)
		}
	}

	if m.inputName.Focused() { m.inputName, cmd = m.inputName.Update(msg) }
	if m.inputImage.Focused() { m.inputImage, cmd = m.inputImage.Update(msg) }
	
	if m.creating {
		var sCmd tea.Cmd
		m.createSpinner, sCmd = m.createSpinner.Update(msg)
		return m, tea.Batch(cmd, sCmd)
	}

	return m, cmd
}

func (m Model) viewCreateEnv() string {
	mod := modules.Registry[m.inputType]
	
	b := strings.Builder{}
	b.WriteString("Create New Environment\n\n")
	b.WriteString(fmt.Sprintf("Name: %s\n", m.inputName.View()))
	b.WriteString(fmt.Sprintf("Type: < %s > (Left/Right to change)\n", mod.Name()))
	
	if mod.Type() == common.EnvTypeStandard {
		b.WriteString(fmt.Sprintf("Image: %s\n", m.inputImage.View()))
	} else if mod.Type() == common.EnvTypeMinecraft {
		b.WriteString(styleDim.Render("Minecraft settings (EULA=True, Port=25565)\n"))
	}
	
	if m.creating {
		b.WriteString(fmt.Sprintf("\n%s Creating...", m.createSpinner.View()))
	} else {
		b.WriteString("\n[Enter] Create  [Esc] Cancel")
	}
	
	if m.createErr != "" {
		b.WriteString(styleErr.Render("\nError: " + m.createErr))
	}

	return styleBox.Render(b.String())
}

// --- Helpers ---

// Split polling messages to allow different frequencies
type telemetryTickMsg time.Time
type listTickMsg time.Time

type loginSuccessMsg struct { client ssh.RemoteInterface }
type errMsg struct { error }

func (m Model) cmdLogin() tea.Cmd {
	return func() tea.Msg {
		var c ssh.RemoteInterface
		var err error

		if m.DevMode {
			c = ssh.NewLocalMockClient()
		} else {
			port := 22
			fmt.Sscanf(m.inputPort.Value(), "%d", &port)
			c, err = ssh.NewClient(m.inputHost.Value(), m.inputUser.Value(), port, m.inputPassword.Value(), "")
			if err != nil {
				return errMsg{err}
			}
		}

		if err := c.Connect(); err != nil {
			return errMsg{err}
		}

		if !m.DevMode {
			// Auto Deploy
			exe, _ := os.Executable()
			binPath := filepath.Join(filepath.Dir(exe), "perssh-server")
			if _, err := os.Stat(binPath); err == nil {
				c.DeployAgent(binPath)
			}
		}

		c.StartAgent()
		return loginSuccessMsg{client: c}
	}
}

func (m Model) cmdPollTelemetry() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		if m.sshClient != nil {
			m.sshClient.SendRequest(common.Request{ID: "telemetry", Type: common.CmdGetTelemetry})
		}
		return telemetryTickMsg(t)
	})
}

// cmdPollListTick schedules the next list refresh.
// Poll every 6 seconds to reduce load (3x less frequent than telemetry).
func (m Model) cmdPollListTick() tea.Cmd {
	return tea.Tick(6*time.Second, func(t time.Time) tea.Msg {
		return listTickMsg(t)
	})
}

func (m Model) cmdPollList() tea.Cmd {
	return func() tea.Msg {
		if m.sshClient != nil {
			m.sshClient.SendRequest(common.Request{ID: "list", Type: common.CmdListContainers})
		}
		return nil
	}
}

func (m Model) cmdCreate() tea.Cmd {
	return func() tea.Msg {
		mod := modules.Registry[m.inputType]
		payload := mod.GetDefaults()
		payload.Name = m.inputName.Value()
		
		if mod.Type() == common.EnvTypeStandard {
			payload.Image = m.inputImage.Value()
		}
		
		m.sshClient.SendRequest(common.Request{
			ID: "create",
			Type: common.CmdCreateEnv,
			Payload: payload,
		})
		return nil
	}
}

func (m Model) waitForPacket() tea.Cmd {
	return func() tea.Msg {
		var resp common.Response
		if err := m.decoder.Decode(&resp); err != nil {
			return errMsg{err}
		}
		return resp
	}
}
