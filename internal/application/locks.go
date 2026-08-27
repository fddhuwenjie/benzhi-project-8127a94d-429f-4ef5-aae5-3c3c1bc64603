package application

import "sync"

type lockEntry struct {
	mu   sync.Mutex
	refs int
}

type caseLocks struct {
	mu      sync.Mutex
	entries map[string]*lockEntry
}

func newCaseLocks() *caseLocks {
	return &caseLocks{entries: make(map[string]*lockEntry)}
}

func (l *caseLocks) lock(caseID string) func() {
	l.mu.Lock()
	entry := l.entries[caseID]
	if entry == nil {
		entry = &lockEntry{}
		l.entries[caseID] = entry
	}
	entry.refs++
	l.mu.Unlock()
	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		l.mu.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(l.entries, caseID)
		}
		l.mu.Unlock()
	}
}

func (l *caseLocks) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}
