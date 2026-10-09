package app

import "sync"

// requestLifetime fences admission before waiting. Add and stop share the
// mutex, so no request can increment a zero WaitGroup after drain begins.
type requestLifetime struct {
	mu      sync.Mutex
	stopped bool
	active  sync.WaitGroup
}

func (l *requestLifetime) enter() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stopped {
		return false
	}
	l.active.Add(1)
	return true
}
func (l *requestLifetime) leave() { l.active.Done() }
func (l *requestLifetime) stop() {
	l.mu.Lock()
	l.stopped = true
	l.mu.Unlock()
}
func (l *requestLifetime) wait() { l.active.Wait() }
