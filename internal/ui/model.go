package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Mode string

const (
	ModeSend    Mode = "sending"
	ModeReceive Mode = "receiving"
)

type Status string

const (
	StatusWaiting      Status = "waiting for peer"
	StatusHashing      Status = "hashing file"
	StatusTransferring Status = "transferring"
	StatusPaused       Status = "paused"
	StatusDone         Status = "done"
	StatusError        Status = "error"
)

const progressBarWidth = 32

type ControlCmd int

const (
	CmdPause  ControlCmd = iota
	CmdResume ControlCmd = iota
)

type PeerConnectedMsg struct{ Addr string }
type FileInfoMsg struct {
	Name  string
	Size  int64
	Total int
}
type ProgressMsg struct{ Transferred int64 }
type StatusMsg struct{ Status Status }
type ErrorMsg struct{ Err error }
type DoneMsg struct{ OutPath string }
type tickMsg time.Time

type Model struct {
	mode        Mode
	addr        string
	peerAddr    string
	fileName    string
	fileSize    int64
	totalChunks int
	transferred int64
	startTime   time.Time
	status      Status
	errMsg      string
	outPath     string
	msgs        <-chan tea.Msg
	ctrl        chan<- ControlCmd
}

func New(mode Mode, addr string, msgs <-chan tea.Msg, ctrl chan<- ControlCmd) Model {
	return Model{
		mode:   mode,
		addr:   addr,
		status: StatusWaiting,
		msgs:   msgs,
		ctrl:   ctrl,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		waitForMsg(m.msgs),
		tick(),
	)
}

func waitForMsg(msgs <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-msgs
	}
}

func tick() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "p":
			if m.status == StatusTransferring && m.ctrl != nil {
				m.ctrl <- CmdPause
				m.status = StatusPaused
				return m, tea.Batch(waitForMsg(m.msgs), tick())
			}
		case "r":
			if m.status == StatusPaused && m.ctrl != nil {
				m.ctrl <- CmdResume
				m.status = StatusTransferring
				return m, tea.Batch(waitForMsg(m.msgs), tick())
			}
		}

	case PeerConnectedMsg:
		m.peerAddr = msg.Addr
		return m, waitForMsg(m.msgs)

	case FileInfoMsg:
		m.fileName = msg.Name
		m.fileSize = msg.Size
		m.totalChunks = msg.Total
		m.status = StatusTransferring
		m.startTime = time.Now()
		return m, waitForMsg(m.msgs)

	case ProgressMsg:
		m.transferred = msg.Transferred
		return m, waitForMsg(m.msgs)

	case StatusMsg:
		m.status = msg.Status
		return m, waitForMsg(m.msgs)

	case DoneMsg:
		m.transferred = m.fileSize
		m.status = StatusDone
		m.outPath = msg.OutPath
		return m, tea.Batch(waitForMsg(m.msgs), tick())

	case ErrorMsg:
		m.status = StatusError
		m.errMsg = msg.Err.Error()
		return m, tea.Batch(waitForMsg(m.msgs), tick())

	case tickMsg:
		return m, tick()
	}

	return m, nil
}

var (
	subtle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	highlight = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	label     = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Width(12)
	value     = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	errStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	doneStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true)
	barFilled = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	barEmpty  = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	keyHint   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

func (m Model) View() string {
	var b strings.Builder

	b.WriteString("\n")

	title := fmt.Sprintf("raft  —  %s", string(m.mode))
	if m.peerAddr != "" {
		if m.mode == ModeSend {
			title += fmt.Sprintf("  →  %s", m.peerAddr)
		} else {
			title += fmt.Sprintf("  ←  %s", m.peerAddr)
		}
	} else {
		title += fmt.Sprintf("  —  listening on %s", m.addr)
	}
	b.WriteString(highlight.Render(title))
	b.WriteString("\n\n")

	if m.fileName != "" {
		b.WriteString(row("file", m.fileName))
		b.WriteString(row("size", formatBytes(m.fileSize)))
		b.WriteString("\n")
		b.WriteString(row("progress", renderBar(m.transferred, m.fileSize)))
		b.WriteString(row("", fmt.Sprintf("%s / %s  (%.1f%%)",
			formatBytes(m.transferred), formatBytes(m.fileSize), percent(m.transferred, m.fileSize))))
		b.WriteString("\n")
		b.WriteString(row("speed", formatBytes(speed(m.transferred, m.startTime))+"/s"))
		b.WriteString(row("eta", eta(m.transferred, m.fileSize, m.startTime)))
		b.WriteString("\n")
	}

	b.WriteString(row("status", statusLine(m)))

	if m.outPath != "" {
		b.WriteString(row("saved to", m.outPath))
	}
	if m.errMsg != "" {
		b.WriteString(row("error", errStyle.Render(m.errMsg)))
	}

	b.WriteString("\n")
	b.WriteString(renderHints(m))

	return b.String()
}

func row(lbl, val string) string {
	return fmt.Sprintf("  %s%s\n", label.Render(lbl), value.Render(val))
}

func renderBar(transferred, total int64) string {
	pct := 0.0
	if total > 0 {
		pct = float64(transferred) / float64(total)
	}
	if pct > 1 {
		pct = 1
	}
	filled := int(pct * float64(progressBarWidth))
	empty := progressBarWidth - filled
	bar := barFilled.Render(strings.Repeat("█", filled)) +
		barEmpty.Render(strings.Repeat("░", empty))
	return "[" + bar + "]"
}

func statusLine(m Model) string {
	switch m.status {
	case StatusDone:
		return doneStyle.Render("✓ complete")
	case StatusError:
		return errStyle.Render("✗ error")
	case StatusPaused:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true).Render("⏸ paused")
	case StatusWaiting:
		return subtle.Render("waiting for peer…")
	case StatusHashing:
		return subtle.Render("hashing file…")
	default:
		return subtle.Render("transferring…")
	}
}

func renderHints(m Model) string {
	hints := []string{}
	if m.status == StatusTransferring {
		hints = append(hints, "p  pause")
	}
	if m.status == StatusPaused {
		hints = append(hints, "r  resume")
	}
	hints = append(hints, "q  quit")
	return keyHint.Render("  "+strings.Join(hints, "    ")) + "\n"
}

func percent(transferred, total int64) float64 {
	if total == 0 {
		return 0
	}
	p := float64(transferred) / float64(total) * 100
	if p > 100 {
		return 100
	}
	return p
}

func speed(transferred int64, start time.Time) int64 {
	elapsed := time.Since(start).Seconds()
	if elapsed <= 0 {
		return 0
	}
	return int64(float64(transferred) / elapsed)
}

func eta(transferred, total int64, start time.Time) string {
	if transferred == 0 || transferred >= total {
		return "--:--"
	}
	s := speed(transferred, start)
	if s == 0 {
		return "--:--"
	}
	remaining := time.Duration(float64(total-transferred)/float64(s)) * time.Second
	remaining = remaining.Round(time.Second)
	m := remaining / time.Minute
	remaining -= m * time.Minute
	return fmt.Sprintf("%02d:%02d", m, remaining/time.Second)
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGT"[exp])
}
