package transfer

import (
	"sync"

	"raft/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

type pauseGate struct {
	mu     sync.Mutex
	paused bool
	cond   *sync.Cond
	msgs   chan<- tea.Msg
}

func newPauseGate(msgs chan<- tea.Msg) *pauseGate {
	g := &pauseGate{msgs: msgs}
	g.cond = sync.NewCond(&g.mu)
	return g
}

func (g *pauseGate) Pause() {
	g.mu.Lock()
	g.paused = true
	g.mu.Unlock()
	g.msgs <- ui.StatusMsg{Status: ui.StatusPaused}
}

func (g *pauseGate) Resume() {
	g.mu.Lock()
	g.paused = false
	g.mu.Unlock()
	g.cond.Broadcast()
	g.msgs <- ui.StatusMsg{Status: ui.StatusTransferring}
}

func (g *pauseGate) WaitIfPaused() {
	g.mu.Lock()
	for g.paused {
		g.cond.Wait()
	}
	g.mu.Unlock()
}
