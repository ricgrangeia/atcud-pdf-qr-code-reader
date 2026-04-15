// Package stats provides a simple file-backed counter for tracking document analyses.
package stats

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type counterData struct {
	Total   int64            `json:"total"`
	Monthly map[string]int64 `json:"monthly"` // key: "2026-04"
}

// Counter is a thread-safe, file-backed document analysis counter.
type Counter struct {
	mu       sync.Mutex
	filePath string
	data     counterData
}

// New loads (or creates) the stats file at filePath.
func New(filePath string) (*Counter, error) {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, err
	}

	c := &Counter{
		filePath: filePath,
		data:     counterData{Monthly: make(map[string]int64)},
	}

	raw, err := os.ReadFile(filePath)
	if err == nil {
		_ = json.Unmarshal(raw, &c.data)
		if c.data.Monthly == nil {
			c.data.Monthly = make(map[string]int64)
		}
	}

	return c, nil
}

// Increment adds 1 to the total and to the current month's counter.
func (c *Counter) Increment() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data.Total++
	key := time.Now().Format("2006-01")
	c.data.Monthly[key]++

	// Best-effort write — never block the request on I/O failure.
	raw, _ := json.Marshal(c.data)
	_ = os.WriteFile(c.filePath, raw, 0644)
}

// Stats returns the total count and the count for the current calendar month.
func (c *Counter) Stats() (total int64, thisMonth int64, monthKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	monthKey = time.Now().Format("2006-01")
	return c.data.Total, c.data.Monthly[monthKey], monthKey
}
