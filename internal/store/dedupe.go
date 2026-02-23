package store

import (
	"sync"
	"time"
)

type Dedupe struct {
	mu       sync.Mutex
	seen     map[string]time.Time
	ttl      time.Duration
	nextTrim time.Time
}

func NewDedupe(ttl time.Duration) *Dedupe {
	return &Dedupe{seen: make(map[string]time.Time), ttl: ttl, nextTrim: time.Now().Add(ttl)}
}

func (d *Dedupe) Allow(key string) bool {
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()

	if t, ok := d.seen[key]; ok && now.Sub(t) < d.ttl {
		return false
	}
	d.seen[key] = now
	if now.After(d.nextTrim) {
		for k, t := range d.seen {
			if now.Sub(t) > d.ttl {
				delete(d.seen, k)
			}
		}
		d.nextTrim = now.Add(d.ttl)
	}
	return true
}
