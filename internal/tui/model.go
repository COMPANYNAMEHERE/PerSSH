package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/COMPANYNAMEHERE/PerSSH/internal/common"
	"github.com/COMPANYNAMEHERE/PerSSH/internal/config"
	"github.com/COMPANYNAMEHERE/PerSSH/internal/discovery"
	"github.com/COMPANYNAMEHERE/PerSSH/internal/modules"
	"github.com/COMPANYNAMEHERE/PerSSH/internal/ssh"
	"github.com/COMPANYNAMEHERE/PerSSH/internal/utils"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	stateFinder
	stateDashboard
	stateEnvDetails
	stateCreateEnv
)

type Model struct {
	state         state
	width, height int
	clientConfig  *config.ClientConfig
	sshClient     ssh.RemoteInterface
	rpcResp       chan common.Response
	logger        *utils.Logger

	// Login
	inputHost, inputUser, inputPort, inputPassword textinput.Model
	loginErr                                       string
	loginSpinner                                   spinner.Model
	loggingIn                                      bool

	// Finder
	finderSpinner  spinner.Model
	finderList     []string
	finderMsg      string
	finderScanning bool
	finderCursor   int

	// Dashboard
	telemetry     common.TelemetryData
	telemetryErr  string
	containers    []common.ContainerInfo
	containerList string // Pre-rendered list for simplicity

	// Env Details
	selectedEnvID string
	logsViewport  viewport.Model
	logsLoading   bool
	consoleInput  textinput.Model
	detailedView  bool
	cpuHistory    []float64
	ramHistory    []float64
	tempHistory   []float64

	// Create Env
	inputName  textinput.Model
	inputType  int             // Index in modules.Registry
	inputImage textinput.Model // For standard
	// Minecraft specific
	mcEula            bool
	mcOp              textinput.Model
	mcRam             textinput.Model
	mcServerTypeIndex int
	mcVersion         textinput.Model
	mcModpack         textinput.Model
	mcAikar           bool

	createErr     string
	creating      bool
	createSpinner spinner.Model
	decoder       *json.Decoder
	DevMode       bool

	// Dashboard Selection
	cursor int
}

func NewModel(logger *utils.Logger) Model {
	cfg, _ := config.LoadClientConfig()
	if cfg == nil {
		cfg = config.DefaultClientConfig()
	}

	h := textinput.New()
	h.Placeholder = "Host IP"
	h.Focus()
	u := textinput.New()
	u.Placeholder = "User"
	p := textinput.New()
	p.Placeholder = "22"
	p.SetValue("22")
	pw := textinput.New()
	pw.Placeholder = "Password"
	pw.EchoMode = textinput.EchoPassword

	// Auto-fill from config
	if cfg.Session.LastHost != "" {
		h.SetValue(cfg.Session.LastHost)
	}
	if cfg.Session.LastUser != "" {
		u.SetValue(cfg.Session.LastUser)
	}
	if cfg.Session.LastPort != 0 {
		p.SetValue(fmt.Sprintf("%d", cfg.Session.LastPort))
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styleGreen
	fs := spinner.New()
	fs.Spinner = spinner.MiniDot
	fs.Style = styleGreen

	nm := textinput.New()
	nm.Placeholder = "Environment Name"
	img := textinput.New()
	img.Placeholder = "Image (e.g. ubuntu:latest)"
	op := textinput.New()
	op.Placeholder = "OP User (Minecraft)"
	ram := textinput.New()
	ram.Placeholder = "RAM (e.g. 2G)"
	
mcVer := textinput.New()
	mcVer.Placeholder = "Version (default: latest)"
	mcMod := textinput.New()
	mcMod.Placeholder = "Modpack URL (optional)"

	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())

	ci := textinput.New()
	ci.Placeholder = "Type command..."
	ci.CharLimit = 200
	ci.Width = 80

	return Model{
		state:        stateLogin,
		clientConfig: cfg,
		logger:       logger,
		inputHost:    h, inputUser: u, inputPort: p, inputPassword: pw,
		loginSpinner:  s,
		finderSpinner: fs,
		createSpinner: s,
		logsViewport:  vp,
		consoleInput:  ci,
		inputName:     nm, inputImage: img, inputType: 0,
		mcOp: op, mcRam: ram, mcVersion: mcVer, mcModpack: mcMod, mcAikar: true,
		rpcResp:     make(chan common.Response),
		cpuHistory:  make([]float64, 0, 300),
		ramHistory:  make([]float64, 0, 300),
		tempHistory: make([]float64, 0, 300),
	}
}

type refreshTickMsg time.Time

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		textinput.Blink, 
		m.loginSpinner.Tick, 
		m.finderSpinner.Tick,
		m.cmdRefreshTick(),
	}
	if m.DevMode {
		return tea.Batch(append(cmds, m.cmdLogin())...)
	}

	// Load credentials async
	cmds = append(cmds, m.cmdLoadCredentials())

	return tea.Batch(cmds...)
}

type autoLoginMsg struct{}
type credentialsLoadedMsg struct{ password string }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case refreshTickMsg:
		// Total refresh to fix glitches
		return m, tea.Batch(tea.ClearScreen, m.cmdRefreshTick())

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			if m.sshClient != nil {
				m.sshClient.Close()
			}
			return m, tea.Quit
		}
	case credentialsLoadedMsg:
		if msg.password != "" {
			m.inputPassword.SetValue(msg.password)
			// Auto-Login if we have everything
			if m.inputHost.Value() != "" && m.inputUser.Value() != "" {
				return m, func() tea.Msg { return autoLoginMsg{} }
			}
		}

	case autoLoginMsg:
		m.loggingIn = true
		return m, m.cmdLogin()

	case loginSuccessMsg:
		m.logger.System("Login successful, switching to dashboard")

		// Save Credentials
		m.clientConfig.Session.LastHost = m.inputHost.Value()
		m.clientConfig.Session.LastUser = m.inputUser.Value()
		fmt.Sscanf(m.inputPort.Value(), "%d", &m.clientConfig.Session.LastPort)

		config.SaveClientConfig(m.clientConfig)
		// Async save to avoid blocking
		go utils.StorePassword(m.inputHost.Value(), m.inputUser.Value(), m.inputPassword.Value())

	case errMsg:
		m.logger.Error("TUI Error Msg: %v", msg.error)
	case common.Response:
		// Handle RPC Responses
		if msg.ID == "telemetry" && msg.Success {
			// Try to convert map to struct (hacky for MVP since JSON unmarshals to map)
			// In production, better JSON handling needed.
			b, _ := json.Marshal(msg.Data)
			json.Unmarshal(b, &m.telemetry)
			
			// Append history
			m.cpuHistory = append(m.cpuHistory, m.telemetry.CPUUsage)
			if len(m.cpuHistory) > 300 {
				m.cpuHistory = m.cpuHistory[1:]
			}
			m.ramHistory = append(m.ramHistory, m.telemetry.RAMUsage)
			if len(m.ramHistory) > 300 {
				m.ramHistory = m.ramHistory[1:]
			}
			m.tempHistory = append(m.tempHistory, m.telemetry.CPUTemp)
			if len(m.tempHistory) > 300 {
				m.tempHistory = m.tempHistory[1:]
			}

			m.logger.System("Received telemetry: CPU %.2f%%", m.telemetry.CPUUsage)
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
		if msg.ID == "logs" {
			m.logsLoading = false
			if msg.Success {
				if logs, ok := msg.Data.(string); ok {
					m.logsViewport.SetContent(logs)
					m.logsViewport.GotoBottom()
				}
			} else {
				m.logsViewport.SetContent("Error fetching logs: " + msg.Error)
			}
		}
		return m, m.waitForPacket()

	case finderResultMsg:
		m.finderScanning = false
		m.finderList = msg.ips
		if len(m.finderList) == 0 {
			m.finderMsg = "No devices found."
		} else {
			m.finderMsg = fmt.Sprintf("Found %d devices.", len(m.finderList))
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Use more conservative sizing to prevent overflow/scroll issues
		m.logsViewport.Width = msg.Width - 6
		m.logsViewport.Height = msg.Height - 14
		m.consoleInput.Width = msg.Width - 8
	}

	// State Switch
	switch m.state {
	case stateLogin:
		return m.updateLogin(msg)
	case stateFinder:
		return m.updateFinder(msg)
	case stateDashboard:
		return m.updateDashboard(msg)
	case stateEnvDetails:
		return m.updateEnvDetails(msg)
	case stateCreateEnv:
		return m.updateCreateEnv(msg)
	}
	return m, nil
}

func (m Model) View() string {
	start := time.Now()
	var s string
	switch m.state {
	case stateLogin:
		s = m.viewLogin()
	case stateFinder:
		s = m.viewFinder()
	case stateDashboard:
		s = m.viewDashboard()
	case stateEnvDetails:
		s = m.viewEnvDetails()
	case stateCreateEnv:
		s = m.viewCreateEnv()
	}
	res := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, s)
	
	if m.clientConfig.General.Debug {
		dur := time.Since(start)
		m.logger.System("View refresh took: %v", dur)
	}
	return res
}

// --- Login ---
func (m Model) updateLogin(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if key, ok := msg.(tea.KeyMsg); ok {
		if key.String() == "enter" {
			m.loggingIn = true
			return m, tea.Batch(m.loginSpinner.Tick, m.cmdLogin())
		}
		if key.String() == "ctrl+f" {
			m.state = stateFinder
			m.finderScanning = true
			m.finderList = []string{}
			m.finderMsg = "Scanning local subnet (Port 22)..."
			return m, tea.Batch(m.finderSpinner.Tick, m.cmdScanNetwork())
		}
		// Simple tab cycle
		if key.String() == "tab" {
			if m.inputHost.Focused() {
				m.inputHost.Blur()
				m.inputUser.Focus()
			} else if m.inputUser.Focused() {
				m.inputUser.Blur()
				m.inputPort.Focus()
			} else if m.inputPort.Focused() {
				m.inputPort.Blur()
				m.inputPassword.Focus()
			} else {
				m.inputPassword.Blur()
				m.inputHost.Focus()
			}
			return m, textinput.Blink
		}
	}

	if m.inputHost.Focused() {
		m.inputHost, cmd = m.inputHost.Update(msg)
	}
	if m.inputUser.Focused() {
		m.inputUser, cmd = m.inputUser.Update(msg)
	}
	if m.inputPort.Focused() {
		m.inputPort, cmd = m.inputPort.Update(msg)
	}
	if m.inputPassword.Focused() {
		m.inputPassword, cmd = m.inputPassword.Update(msg)
	}

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
		b.WriteString("\n[Enter] Connect   [Ctrl+F] Find Servers")
	}

	if m.loginErr != "" {
		b.WriteString(styleErr.Render("\nError: " + m.loginErr))
	}
	return styleBox.Render(b.String())
}

// --- Finder ---
func (m Model) updateFinder(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if key.String() == "esc" {
			m.state = stateLogin
			return m, nil
		}
		if key.String() == "up" {
			if m.finderCursor > 0 {
				m.finderCursor--
			}
		}
		if key.String() == "down" {
			if m.finderCursor < len(m.finderList)-1 {
				m.finderCursor++
			}
		}
		if key.String() == "enter" {
			if len(m.finderList) > 0 {
				selectedIP := m.finderList[m.finderCursor]
				m.inputHost.SetValue(selectedIP)
				m.state = stateLogin
				return m, nil
			}
		}
	}

	if m.finderScanning {
		var sCmd tea.Cmd
		m.finderSpinner, sCmd = m.finderSpinner.Update(msg)
		return m, sCmd
	}

	return m, nil
}

func (m Model) viewFinder() string {
	b := strings.Builder{}
	b.WriteString(styleGreen.Render("Network Scanner") + "\n\n")

	if m.finderScanning {
		b.WriteString(fmt.Sprintf("%s %s\n", m.finderSpinner.View(), m.finderMsg))
	} else {
		b.WriteString(m.finderMsg + "\n\n")
		for i, ip := range m.finderList {
			cursor := "  "
			if i == m.finderCursor {
				cursor = "> "
			}
			b.WriteString(fmt.Sprintf("%s%s\n", cursor, ip))
		}
		b.WriteString("\n[Enter] Select   [Esc] Cancel")
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
		case "enter":
			if len(m.containers) > 0 && m.cursor < len(m.containers) {
				m.state = stateEnvDetails
				m.selectedEnvID = m.containers[m.cursor].ID
				m.logsLoading = true
				m.logsViewport.SetContent("Loading logs...")
				// m.consoleInput.Focus() // Removed to allow shortcuts first
				return m, tea.Batch(m.cmdGetLogs(m.selectedEnvID), m.cmdPollLogsTick(), textinput.Blink)
			}
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			if m.cursor < len(m.containers)-1 {
				m.cursor++
			}
		case "s":
			// Toggle Start/Stop
			if len(m.containers) > 0 && m.cursor < len(m.containers) {
				c := m.containers[m.cursor]
				cmdType := common.CmdStartEnv
				if c.Status == "running" {
					cmdType = common.CmdStopEnv
				}
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
	menu := styleDim.Render("[Enter] Details  [C] Create  [L] Refresh  [S] Start/Stop  [X] Remove  [Q] Quit")

	// Content
	var s strings.Builder
	s.WriteString("Active Environments:\n")
	for i, c := range m.containers {
		pref := "  "
		if i == m.cursor {
			pref = styleGreen.Render("> ")
		}

		statusStyle := styleDim
		if c.Status == "running" {
			statusStyle = styleGreen
		}

		s.WriteString(fmt.Sprintf("%s%s - %s [%s]\n", pref, c.Name, c.Image, statusStyle.Render(c.Status)))
	}

	content := s.String()
	if len(m.containers) == 0 {
		content += styleDim.Render("(No environments running)")
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		styleBox.Render(stats),
		styleBox.Render(content),
		menu,
	)
}

// --- Env Details ---
func (m Model) updateEnvDetails(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if key, ok := msg.(tea.KeyMsg); ok {
		// Always allow Esc to handle focus/exit
		if key.String() == "esc" {
			if m.consoleInput.Focused() {
				m.consoleInput.Blur()
				return m, nil
			}
			m.state = stateDashboard
			// Reset view settings when leaving
			m.detailedView = false
			return m, nil
		}

		// Mode-specific handling
		if m.consoleInput.Focused() {
			// Typing Mode
			if key.String() == "enter" {
				if m.consoleInput.Value() != "" {
					cmdStr := m.consoleInput.Value()
					m.consoleInput.SetValue("")
					return m, tea.Batch(
						m.cmdSendInput(m.selectedEnvID, cmdStr),
						// Force an immediate log refresh
						m.cmdGetLogs(m.selectedEnvID),
					)
				}
			}
		} else {
			// Navigation Mode
			if key.String() == "enter" {
				m.consoleInput.Focus()
				return m, textinput.Blink
			}
			if key.String() == "r" {
				m.logsLoading = true
				m.logsViewport.SetContent("Refreshing...")
				return m, m.cmdGetLogs(m.selectedEnvID)
			}
			if key.String() == "d" || key.String() == "tab" {
				m.detailedView = !m.detailedView
				// Adjust viewport height based on view mode
				if m.detailedView {
					// Logs get 50%
					m.logsViewport.Height = (m.height / 2) - 8
				} else {
					// Logs get full space minus headers
					m.logsViewport.Height = m.height - 14
				}
				return m, nil
			}
		}
	}

	// Handle telemetry tick in background to keep graph/stats alive if we return
	if _, ok := msg.(telemetryTickMsg); ok {
		return m, m.cmdPollTelemetry()
	}

	// Handle log tick
	if _, ok := msg.(logTickMsg); ok {
		return m, tea.Batch(m.cmdGetLogs(m.selectedEnvID), m.cmdPollLogsTick())
	}

	m.logsViewport, cmd = m.logsViewport.Update(msg)
	var ciCmd tea.Cmd
	m.consoleInput, ciCmd = m.consoleInput.Update(msg)

	return m, tea.Batch(cmd, ciCmd)
}

func (m Model) viewEnvDetails() string {
	// Truncate ID if too long to prevent wrapping
	id := m.selectedEnvID
	if len(id) > 40 {
		id = id[:37] + "..."
	}
	title := styleGreen.Render(fmt.Sprintf("Environment Details: %s", id))

	// Small Stats Line
	smallStats := fmt.Sprintf("CPU: %s | RAM: %s | Disk: %s | Temp: %.1fC",
		styleGreen.Render(fmt.Sprintf("%.1f%%", m.telemetry.CPUUsage)),
		styleGreen.Render(fmt.Sprintf("%.1f%%", m.telemetry.RAMUsage)),
		styleGreen.Render(fmt.Sprintf("%dGB", m.telemetry.DiskFree/1024/1024/1024)),
		m.telemetry.CPUTemp,
	)

	var help string
	if m.consoleInput.Focused() {
		help = styleDim.Render("[Esc] Stop Typing   [Enter] Send Command")
	} else {
		help = styleDim.Render("[Esc] Back   [Enter] Type Command   [R] Refresh Logs   [D/Tab] Toggle Graphs")
	}

	var topView string
	if m.detailedView {
		// Render Graphs
		// 50% height
		graphHeight := (m.height / 2) - 4
		if graphHeight < 5 {
			graphHeight = 5
		}
		graphWidth := m.width - 6

		series := map[string]struct {
			Data    []float64
			ColorFn func(float64) lipgloss.Style
		}{
			"CPU": {m.cpuHistory, func(_ float64) lipgloss.Style { return lipgloss.NewStyle().Foreground(lipgloss.Color("39")) }}, // Blue
			"RAM": {m.ramHistory, func(_ float64) lipgloss.Style { return lipgloss.NewStyle().Foreground(lipgloss.Color("255")) }}, // White
			"Temp": {m.tempHistory, func(val float64) lipgloss.Style {
				if val > 80 {
					return lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red
				}
				if val > 60 {
					return lipgloss.NewStyle().Foreground(lipgloss.Color("220")) // Yellow
				}
				return lipgloss.NewStyle().Foreground(lipgloss.Color("46")) // Green
			}},
		}
		
		topView = lipgloss.JoinVertical(lipgloss.Left, 
			title, 
			renderCombinedGraph(series, graphWidth, graphHeight),
		)
	} else {
		topView = lipgloss.JoinVertical(lipgloss.Left, title, smallStats)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		topView,
		m.logsViewport.View(),
		m.consoleInput.View(),
		help,
	)
}

func renderCombinedGraph(series map[string]struct{Data []float64; ColorFn func(float64) lipgloss.Style}, width, height int) string {
	if width < 1 || height < 1 {
		return ""
	}

	// 1. Prepare Grid
	// grid[y][x]
	type cell struct {
		char  string
		style lipgloss.Style
	}
	grid := make([][]cell, height)
	for y := 0; y < height; y++ {
		grid[y] = make([]cell, width)
		for x := 0; x < width; x++ {
			grid[y][x] = cell{char: " ", style: styleDim}
		}
	}

	// 2. Plot Data
	minVal, maxVal := 0.0, 100.0
	
	// Order of drawing determines z-index (later overwrites earlier)
	order := []string{"CPU", "RAM", "Temp"}
	
	for _, label := range order {
		s, ok := series[label]
		if !ok { continue }
		
		data := s.Data
		// Take last `width` points
		start := 0
		if len(data) > width {
			start = len(data) - width
		}
		viewData := data[start:]
		
		for x, val := range viewData {
			if x >= width { break }
			
			// Normalize y
			// Y=0 is top in array, but we want Y=0 at bottom visually
			norm := (val - minVal) / (maxVal - minVal)
			if norm < 0 { norm = 0 }
			if norm > 1 { norm = 1 }
			
			y := int((1.0 - norm) * float64(height-1))
			
			// Plot
			// If collision, maybe use a blended char? For now, straight overwrite.
			style := s.ColorFn(val)
			grid[y][x] = cell{char: "•", style: style}
		}
	}

	// 3. Render to String
	var sb strings.Builder
	
	// Top Border / Title
	sb.WriteString(styleDim.Render("┌" + strings.Repeat("─", width) + "┐") + "\n")
	
	for y := 0; y < height; y++ {
		sb.WriteString(styleDim.Render("│"))
		for x := 0; x < width; x++ {
			c := grid[y][x]
			sb.WriteString(c.style.Render(c.char))
		}
		sb.WriteString(styleDim.Render("│"))
		// Axis labels could go here
		if y == 0 { sb.WriteString(styleDim.Render(" 100%")) }
		if y == height-1 { sb.WriteString(styleDim.Render(" 0%")) }
		sb.WriteString("\n")
	}
	sb.WriteString(styleDim.Render("└" + strings.Repeat("─", width) + "┘") + "\n")

	// 4. Legend
	var legend []string
	for _, label := range order {
		s := series[label]
		curr := 0.0
		if len(s.Data) > 0 {
			curr = s.Data[len(s.Data)-1]
		}
		
		displayLabel := label
		if label == "CPU" || label == "RAM" {
			displayLabel += " (%)"
		}
		
		style := s.ColorFn(curr)
		legend = append(legend, style.Render(fmt.Sprintf("● %s: %.1f", displayLabel, curr)))
	}
	sb.WriteString(" " + strings.Join(legend, "   "))
	
	return sb.String()
}

// --- Create Env ---
var mcTypes = []string{"VANILLA", "FORGE", "FABRIC", "ARCLIGHT", "NEOFORGE"}

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
		
		// Type selection for Module (Standard vs Minecraft)
		if key.String() == "left" {
			m.inputType--
			if m.inputType < 0 { m.inputType = len(modules.Registry) - 1 }
		}
		if key.String() == "right" {
			m.inputType = (m.inputType + 1) % len(modules.Registry)
		}

		mod := modules.Registry[m.inputType]
		if mod.Type() == common.EnvTypeMinecraft {
			if key.String() == "t" {
				m.mcServerTypeIndex = (m.mcServerTypeIndex + 1) % len(mcTypes)
			}
			if key.String() == "f" {
				m.mcAikar = !m.mcAikar
			}
		}

		// Tab cycle
		if key.String() == "tab" {
			if mod.Type() == common.EnvTypeStandard {
				if m.inputName.Focused() {
					m.inputName.Blur()
					m.inputImage.Focus()
				} else {
					m.inputImage.Blur()
					m.inputName.Focus()
				}
			} else {
				// Minecraft Cycle
				// Name -> Version -> Modpack -> Op -> Ram -> Name
				if m.inputName.Focused() {
					m.inputName.Blur()
					m.mcVersion.Focus()
				} else if m.mcVersion.Focused() {
					m.mcVersion.Blur()
					m.mcModpack.Focus()
				} else if m.mcModpack.Focused() {
					m.mcModpack.Blur()
					m.mcOp.Focus()
				} else if m.mcOp.Focused() {
					m.mcOp.Blur()
					m.mcRam.Focus()
				} else if m.mcRam.Focused() {
					m.mcRam.Blur()
					m.inputName.Focus()
				} else {
					m.inputName.Focus()
				}
			}
			return m, textinput.Blink
		}
	}

	if m.inputName.Focused() {
		m.inputName, cmd = m.inputName.Update(msg)
	}
	if m.inputImage.Focused() {
		m.inputImage, cmd = m.inputImage.Update(msg)
	}
	if m.mcVersion.Focused() {
		m.mcVersion, cmd = m.mcVersion.Update(msg)
	}
	if m.mcModpack.Focused() {
		m.mcModpack, cmd = m.mcModpack.Update(msg)
	}
	if m.mcOp.Focused() {
		m.mcOp, cmd = m.mcOp.Update(msg)
	}
	if m.mcRam.Focused() {
		m.mcRam, cmd = m.mcRam.Update(msg)
	}

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
	b.WriteString(fmt.Sprintf("Module: < %s > (Left/Right)\n", mod.Name()))

	if mod.Type() == common.EnvTypeStandard {
		b.WriteString(fmt.Sprintf("Image: %s\n", m.inputImage.View()))
	} else if mod.Type() == common.EnvTypeMinecraft {
		b.WriteString("\nMinecraft Settings:\n")
		b.WriteString(fmt.Sprintf("  Server Type: %s (Press 't' to cycle)\n", styleGreen.Render(mcTypes[m.mcServerTypeIndex])))
		
		aikarState := "[ ]"
		if m.mcAikar { aikarState = styleGreen.Render("[x]") }
		b.WriteString(fmt.Sprintf("  Performance: %s Aikar's Flags (Press 'f')\n", aikarState))
		
		b.WriteString(fmt.Sprintf("  Version: %s\n", m.mcVersion.View()))
		b.WriteString(fmt.Sprintf("  Modpack: %s\n", m.mcModpack.View()))
		b.WriteString(fmt.Sprintf("  OP Users: %s\n", m.mcOp.View()))
		b.WriteString(fmt.Sprintf("  RAM Limit: %s\n", m.mcRam.View()))
	}

	if m.creating {
		b.WriteString(fmt.Sprintf("\n%s Creating...", m.createSpinner.View()))
	} else {
		b.WriteString("\n[Enter] Create  [Tab] Next Field  [Esc] Cancel")
	}

	if m.createErr != "" {
		b.WriteString(styleErr.Render("\nError: " + m.createErr))
	}

	return styleBox.Render(b.String())
}

func (m Model) cmdCreate() tea.Cmd {
	return func() tea.Msg {
		mod := modules.Registry[m.inputType]
		payload := mod.GetDefaults()
		payload.Name = m.inputName.Value()

		if mod.Type() == common.EnvTypeStandard {
			payload.Image = m.inputImage.Value()
		} else if mod.Type() == common.EnvTypeMinecraft {
			payload.RamLimit = m.mcRam.Value()
			
			// Config
			mc := payload.Minecraft
			mc.ServerType = mcTypes[m.mcServerTypeIndex]
			
			ver := m.mcVersion.Value()
			if ver == "" { ver = "latest" }
			mc.Version = ver
			
			mc.Modpack = m.mcModpack.Value()
			
			if m.mcOp.Value() != "" {
				mc.OpUsers = strings.Split(m.mcOp.Value(), ",")
			}
			
			mc.Features = []string{}
			if m.mcAikar {
				mc.Features = append(mc.Features, common.FeatureAikarsFlags)
			}
			// Future: Plugins
			
			payload.Minecraft = mc
		}

		m.sshClient.SendRequest(common.Request{
			ID:      "create",
			Type:    common.CmdCreateEnv,
			Payload: payload,
		})
		return nil
	}
}

// --- Helpers ---

// Split polling messages to allow different frequencies
type telemetryTickMsg time.Time
type listTickMsg time.Time
type logTickMsg time.Time

type loginSuccessMsg struct{ client ssh.RemoteInterface }
type errMsg struct{ error }

func (m Model) cmdLogin() tea.Cmd {
	// Capture values to ensure closure uses correct data
	host := m.inputHost.Value()
	user := m.inputUser.Value()
	pass := m.inputPassword.Value()
	portStr := m.inputPort.Value()

	return func() tea.Msg {
		var c ssh.RemoteInterface
		var err error

		if m.DevMode {
			c = ssh.NewLocalMockClient()
		} else {
			if pass == "" {
				return errMsg{fmt.Errorf("password required")}
			}
			port := 22
			fmt.Sscanf(portStr, "%d", &port)

			// Log connection attempt
			m.logger.System("Attempting SSH connection to %s@%s:%d", user, host, port)

			c, err = ssh.NewClient(host, user, port, pass, "")
			if err != nil {
				return errMsg{err}
			}
		}

		if err := c.Connect(); err != nil {
			return errMsg{fmt.Errorf("connect failed: %w", err)}
		}

		if !m.DevMode {
			// Auto Deploy
			exe, _ := os.Executable()
			binPath := filepath.Join(filepath.Dir(exe), "perssh-server")
			if _, err := os.Stat(binPath); err == nil {
				if err := c.DeployAgent(binPath); err != nil {
					return errMsg{fmt.Errorf("deploy failed: %w", err)}
				}
			}
		}

		if err := c.StartAgent(); err != nil {
			return errMsg{fmt.Errorf("failed to start agent: %w", err)}
		}
		return loginSuccessMsg{client: c}
	}
}

func (m Model) cmdPollTelemetry() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		if m.sshClient != nil {
			m.sshClient.SendRequest(common.Request{ID: "telemetry", Type: common.CmdGetTelemetry})
		}
		return telemetryTickMsg(t)
	})
}

func (m Model) cmdPollLogsTick() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg {
		return logTickMsg(t)
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

func (m Model) cmdRefreshTick() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return refreshTickMsg(t)
	})
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

type finderResultMsg struct {
	ips []string
}

func (m Model) cmdScanNetwork() tea.Cmd {
	return func() tea.Msg {
		subnet, err := discovery.GetLocalSubnet()
		if err != nil {
			// Fallback to 192.168.1 if detection fails
			subnet = "192.168.1"
		}

		// Scan for SSH (Port 22)
		ips := discovery.ScanSubnet(subnet, 22, 500*time.Millisecond)

		// If none found on 22, maybe check 8080 (Agent)?
		// For now, let's stick to SSH as that's what the client uses.

		return finderResultMsg{ips: ips}
	}
}

func (m Model) cmdGetLogs(id string) tea.Cmd {
	return func() tea.Msg {
		if m.sshClient != nil {
			m.sshClient.SendRequest(common.Request{
				ID:      "logs",
				Type:    common.CmdGetLogs,
				Payload: id,
			})
		}
		return nil
	}
}

func (m Model) cmdSendInput(id, data string) tea.Cmd {
	return func() tea.Msg {
		if m.sshClient != nil {
			payload := map[string]string{
				"id":   id,
				"data": data,
			}
			m.sshClient.SendRequest(common.Request{
				ID:      "input",
				Type:    common.CmdSendInput,
				Payload: payload,
			})
		}
		return nil
	}
}

func (m Model) cmdLoadCredentials() tea.Cmd {
	return func() tea.Msg {
		host := m.inputHost.Value()
		user := m.inputUser.Value()
		if host != "" && user != "" {
			// Use a channel to implement timeout
			c := make(chan string, 1)
			go func() {
				if pass, err := utils.GetPassword(host, user); err == nil {
					c <- pass
				} else {
					c <- ""
				}
			}()

			select {
			case pass := <-c:
				return credentialsLoadedMsg{password: pass}
			case <-time.After(500 * time.Millisecond):
				// Timeout if keyring is slow/hanging
				return credentialsLoadedMsg{password: ""}
			}
		}
		return nil
	}
}
