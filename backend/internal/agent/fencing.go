package agent

import "sync"

type FencingManager struct {
	mu                  sync.RWMutex
	highestFencingToken int64
}

func NewFencingManager(initialToken int64) *FencingManager {
	return &FencingManager{
		highestFencingToken: initialToken,
	}
}

func (f *FencingManager) ValidateAndUpdate(token int64) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	if token < f.highestFencingToken {
		return false // Mismatch / Stale fencing token from old lease
	}

	f.highestFencingToken = token
	return true
}

func (f *FencingManager) GetHighest() int64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.highestFencingToken
}
