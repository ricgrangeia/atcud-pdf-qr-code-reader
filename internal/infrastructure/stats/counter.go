// Package stats provides a simple file-backed counter for tracking document analyses.
package stats

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// monthData holds per-month counts broken down by source.
type monthData struct {
	Total   int64            `json:"total"`
	Sources map[string]int64 `json:"sources"` // e.g. "web", "android", "api"
}

type counterData struct {
	Total   int64                `json:"total"`
	Monthly map[string]monthData `json:"monthly"` // key: "2026-04"
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
		data:     counterData{Monthly: make(map[string]monthData)},
	}

	raw, err := os.ReadFile(filePath)
	if err == nil {
		_ = json.Unmarshal(raw, &c.data)
		if c.data.Monthly == nil {
			c.data.Monthly = make(map[string]monthData)
		}
		// Ensure each month entry has an initialized Sources map.
		for k, v := range c.data.Monthly {
			if v.Sources == nil {
				v.Sources = make(map[string]int64)
				c.data.Monthly[k] = v
			}
		}
	}

	return c, nil
}

// Increment adds 1 to the total and to the current month's counter for the given source.
// source should be one of "web", "android", or "api".
func (c *Counter) Increment(source string) {
	c.mu.Lock()
	c.data.Total++
	key := time.Now().Format("2006-01")

	m := c.data.Monthly[key]
	if m.Sources == nil {
		m.Sources = make(map[string]int64)
	}
	m.Total++
	m.Sources[source]++
	c.data.Monthly[key] = m

	// Snapshot for the async write — taken while still holding the lock.
	raw, _ := json.Marshal(c.data)
	c.mu.Unlock()

	// Write to disk asynchronously so the request is never blocked by I/O.
	go func() { _ = os.WriteFile(c.filePath, raw, 0644) }()
}

// StatsResult carries the full breakdown returned by Stats.
type StatsResult struct {
	Total          int64            `json:"total"`
	ThisMonth      int64            `json:"this_month"`
	ThisMonthWeb   int64            `json:"this_month_web"`
	ThisMonthApp   int64            `json:"this_month_app"`
	ThisMonthOther int64            `json:"this_month_other"`
	Month          string           `json:"month"`
	Sources        map[string]int64 `json:"sources"` // all-time per-source totals
}

// Stats returns the full statistics breakdown.
func (c *Counter) Stats() StatsResult {
	c.mu.Lock()
	defer c.mu.Unlock()

	monthKey := time.Now().Format("2006-01")
	m := c.data.Monthly[monthKey]

	// Aggregate all-time per-source totals.
	allSources := make(map[string]int64)
	for _, md := range c.data.Monthly {
		for src, n := range md.Sources {
			allSources[src] += n
		}
	}

	return StatsResult{
		Total:          c.data.Total,
		ThisMonth:      m.Total,
		ThisMonthWeb:   m.Sources["web"],
		ThisMonthApp:   m.Sources["android"],
		ThisMonthOther: m.Sources["api"],
		Month:          monthKey,
		Sources:        allSources,
	}
}
