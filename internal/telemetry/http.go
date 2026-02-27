package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

// event is the unit of telemetry — a named event with merged properties.
type event struct {
	Name  string         `json:"name"`
	Props map[string]any `json:"props"`
}

// poster is a function that sends a batch of events to the Cloudflare Worker.
// The client struct holds a poster field; tests override it with a mock.
type poster func(ctx context.Context, url string, events []event) error

// defaultPoster marshals events and POSTs them to the Cloudflare Worker.
func defaultPoster(ctx context.Context, url string, events []event) error {
	body, err := json.Marshal(struct {
		Events []event `json:"events"`
	}{Events: events})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req) //nolint:gosec // url is a hardcoded constant, not user input
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	return nil
}
