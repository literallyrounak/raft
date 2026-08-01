package transfer

import "sync"

type pauseGate struct {
	mu     sync.Mutex
	paused bool
	cond   *sync.Cond
}

func newPauseGate() *pauseGate {
	gate := &pauseGate{}
	gate.cond = sync.NewCond(&gate.mu)
	return gate
}

func (g *pauseGate) Pause() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.paused = true
}

func (g *pauseGate) Resume() {
	g.mu.Lock()
	g.paused = false
	g.mu.Unlock()
	g.cond.Broadcast()
}

func (g *pauseGate) WaitIfPaused() {
	g.mu.Lock()
	for g.paused {
		g.cond.Wait()
	}
	g.mu.Unlock()
}
