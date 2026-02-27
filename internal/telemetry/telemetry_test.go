package telemetry

import (
	"context"
	"sync"
	"testing"
	"time"
)

// mockPoster records calls for assertions.
type mockPoster struct {
	mu     sync.Mutex
	called [][]event
	delay  time.Duration
}

func (m *mockPoster) post(ctx context.Context, url string, events []event) error {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.called = append(m.called, events)
	return nil
}

func (m *mockPoster) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.called)
}

func (m *mockPoster) firstBatch() []event {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.called) == 0 {
		return nil
	}
	return m.called[0]
}

// ── New ───────────────────────────────────────────────────────────────────────

func TestNew_CI_returnsNoop(t *testing.T) {
	t.Setenv("CI", "1")
	r := New("some-id", "http://example.com", "0.1.0")
	if _, ok := r.(noop); !ok {
		t.Errorf("New() with CI=1 should return noop, got %T", r)
	}
}

func TestNew_emptyInstallID_returnsNoop(t *testing.T) {
	r := New("", "http://example.com", "0.1.0")
	if _, ok := r.(noop); !ok {
		t.Errorf("New() with empty installID should return noop, got %T", r)
	}
}

func TestNew_emptyWorkerURL_returnsNoop(t *testing.T) {
	r := New("some-id", "", "0.1.0")
	if _, ok := r.(noop); !ok {
		t.Errorf("New() with empty workerURL should return noop, got %T", r)
	}
}

// ── noop ─────────────────────────────────────────────────────────────────────

func TestNoop_RecordAndFlush_noPanic(t *testing.T) {
	n := noop{}
	n.Record("cmd_view", map[string]any{"success": true}) // must not panic
	n.Flush()                                             // must not panic
}

// ── Record ────────────────────────────────────────────────────────────────────

func TestRecord_queuesEventWithStandardProperties(t *testing.T) {
	mock := &mockPoster{}
	c := &client{
		id:      "test-uuid",
		url:     "http://unused",
		version: "0.2.0",
		poster:  mock.post,
		mu:      &sync.Mutex{},
	}

	c.Record("cmd_view", map[string]any{"success": true})

	c.mu.Lock()
	pending := c.pending
	c.mu.Unlock()

	if len(pending) != 1 {
		t.Fatalf("expected 1 queued event, got %d", len(pending))
	}
	e := pending[0]
	if e.Name != "cmd_view" {
		t.Errorf("event name = %q, want %q", e.Name, "cmd_view")
	}
	checks := map[string]any{
		"distinct_id": "test-uuid",
		"gcm_version": "0.2.0",
		"success":     true,
	}
	for k, want := range checks {
		if got := e.Props[k]; got != want {
			t.Errorf("props[%q] = %v, want %v", k, got, want)
		}
	}
	// os and arch must be non-empty (populated from runtime)
	if e.Props["os"] == "" {
		t.Error("props[os] should not be empty")
	}
	if e.Props["arch"] == "" {
		t.Error("props[arch] should not be empty")
	}
}

// ── Flush ─────────────────────────────────────────────────────────────────────

func TestFlush_emptyQueue_returnsImmediately(t *testing.T) {
	mock := &mockPoster{}
	c := &client{
		id:     "test-uuid",
		url:    "http://unused",
		poster: mock.post,
		mu:     &sync.Mutex{},
	}
	c.Flush() // must not block or panic
	if mock.callCount() != 0 {
		t.Error("Flush() with empty queue should not call poster")
	}
}

func TestFlush_deliversEventsToMockPoster(t *testing.T) {
	mock := &mockPoster{}
	c := &client{
		id:      "test-uuid",
		url:     "http://unused",
		version: "0.2.0",
		poster:  mock.post,
		mu:      &sync.Mutex{},
	}
	c.Record("cmd_view", map[string]any{"success": true})
	c.Record("cmd_assign", map[string]any{"success": false})
	c.Flush()

	if mock.callCount() != 1 {
		t.Fatalf("expected 1 poster call, got %d", mock.callCount())
	}
	batch := mock.firstBatch()
	if len(batch) != 2 {
		t.Errorf("expected 2 events in batch, got %d", len(batch))
	}
}

func TestFlush_hardCapAt500ms(t *testing.T) {
	// Poster sleeps 2s — Flush must return in under 750ms (500ms cap + buffer).
	slowMock := &mockPoster{delay: 2 * time.Second}
	c := &client{
		id:     "test-uuid",
		url:    "http://unused",
		poster: slowMock.post,
		mu:     &sync.Mutex{},
	}
	c.Record("cmd_view", map[string]any{"success": true})

	start := time.Now()
	c.Flush()
	elapsed := time.Since(start)

	if elapsed > 750*time.Millisecond {
		t.Errorf("Flush() took %v, want ≤ 750ms (hard 500ms cap)", elapsed)
	}
}

func TestFlush_clearsQueueAfterFlush(t *testing.T) {
	mock := &mockPoster{}
	c := &client{
		id:     "test-uuid",
		url:    "http://unused",
		poster: mock.post,
		mu:     &sync.Mutex{},
	}
	c.Record("cmd_view", map[string]any{"success": true})
	c.Flush()
	c.Flush() // second Flush must not resend

	if mock.callCount() != 1 {
		t.Errorf("expected 1 poster call after two Flushes, got %d", mock.callCount())
	}
}
