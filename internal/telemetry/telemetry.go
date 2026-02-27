package telemetry

import (
	"context"
	"os"
	"runtime"
	"sync"
	"time"
)

// Recorder is the only interface the rest of gcm uses for telemetry.
// Record is fire-and-forget — it returns no error.
// Flush drains the queue with a 500ms hard cap, then returns.
type Recorder interface {
	Record(event string, props map[string]any)
	Flush()
}

// Noop returns a Recorder that discards all events. Used as the safe default
// in cmd/root.go before Execute(tel) is called.
func Noop() Recorder { return noop{} }

type noop struct{}

func (noop) Record(string, map[string]any) {}
func (noop) Flush()                        {}

type client struct {
	id      string
	url     string
	version string
	poster  poster
	mu      *sync.Mutex
	pending []event
}

// New returns a Recorder. Never returns nil. Returns noop when disabled or
// when installID / workerURL are empty. version should be cmd.Version().
func New(installID, workerURL, version string) Recorder {
	if isDisabled() || installID == "" || workerURL == "" {
		return noop{}
	}
	return &client{
		id:      installID,
		url:     workerURL,
		version: version,
		poster:  defaultPoster,
		mu:      &sync.Mutex{},
	}
}

// isDisabled reports whether telemetry is suppressed by environment variables.
// Intentionally duplicated from internal/config to avoid a circular import.
func isDisabled() bool {
	return os.Getenv("CI") != "" ||
		os.Getenv("GITHUB_ACTIONS") != ""
}

// Record queues an event for delivery. Merges standard properties (distinct_id,
// os, arch, gcm_version) with the caller-supplied props. Pure in-memory — no
// network I/O on the hot path.
func (c *client) Record(name string, props map[string]any) {
	merged := map[string]any{
		"distinct_id": c.id,
		"os":          runtime.GOOS,
		"arch":        runtime.GOARCH,
		"gcm_version": c.version,
	}
	for k, v := range props {
		merged[k] = v
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending = append(c.pending, event{Name: name, Props: merged})
}

// Flush sends all queued events to the Cloudflare Worker in a goroutine with
// a 400ms context deadline. A hard 500ms time.After cap ensures the process
// never blocks more than 500ms regardless of network conditions.
func (c *client) Flush() {
	c.mu.Lock()
	events := c.pending
	c.pending = nil
	c.mu.Unlock()

	if len(events) == 0 {
		return
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
		defer cancel()
		_ = c.poster(ctx, c.url, events) // silent on failure
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond): // hard cap — drop events, never hang
	}
}
