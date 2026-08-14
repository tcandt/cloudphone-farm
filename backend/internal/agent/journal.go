package agent

import (
	"sync"
	"time"
)

type JournalEntry struct {
	CommandID    string    `json:"command_id"`
	FencingToken int64     `json:"fencing_token"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"error_message,omitempty"`
	ExecutedAt   time.Time `json:"executed_at"`
}

type CommandJournal struct {
	mu      sync.RWMutex
	entries map[string]*JournalEntry
}

func NewCommandJournal() *CommandJournal {
	return &CommandJournal{
		entries: make(map[string]*JournalEntry),
	}
}

func (j *CommandJournal) Get(commandID string) (*JournalEntry, bool) {
	j.mu.RLock()
	defer j.mu.RUnlock()
	entry, exists := j.entries[commandID]
	return entry, exists
}

func (j *CommandJournal) Record(commandID string, fencingToken int64, status string, errStr string) *JournalEntry {
	j.mu.Lock()
	defer j.mu.Unlock()

	entry := &JournalEntry{
		CommandID:    commandID,
		FencingToken: fencingToken,
		Status:       status,
		ErrorMessage: errStr,
		ExecutedAt:   time.Now().UTC(),
	}
	j.entries[commandID] = entry
	return entry
}
