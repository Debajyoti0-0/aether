package validate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HECClient streams telemetry into Splunk HTTP Event Collector
// ( Splunk Enterprise / Splunk Cloud), which is also compatible with
// many SIEM ingest endpoints (Elastic HEC bridges, custom sinks).
type HECClient struct {
	// Endpoint is the HEC collector URL, e.g.
	// https://splunk.corp.com:8088/services/collector/event
	Endpoint string
	Token    string
	// Source / Sourcetype / Index metadata applied to each event.
	Source     string
	Sourcetype string
	Index      string

	HTTP *http.Client
}

// NewHECClient builds an HEC streaming client.
func NewHECClient(endpoint, token, source, sourcetype, index string) *HECClient {
	return &HECClient{
		Endpoint:   strings.TrimRight(endpoint, "/"),
		Token:      token,
		Source:     source,
		Sourcetype: sourcetype,
		Index:      index,
		HTTP:       &http.Client{Timeout: 15 * time.Second},
	}
}

// StreamEvents posts events (any JSON-serializable record) to HEC in
// batch form and returns the count accepted.
func (c *HECClient) StreamEvents(ctx context.Context, events []map[string]any) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}
	if c.Endpoint == "" || c.Token == "" {
		return 0, fmt.Errorf("hec endpoint and token are required")
	}

	// NDJSON batch: one event object per line.
	var buf bytes.Buffer
	for _, ev := range events {
		wrapper := map[string]any{"event": ev, "time": time.Now().Unix()}
		if c.Source != "" {
			wrapper["source"] = c.Source
		}
		if c.Sourcetype != "" {
			wrapper["sourcetype"] = c.Sourcetype
		}
		if c.Index != "" {
			wrapper["index"] = c.Index
		}
		data, err := json.Marshal(wrapper)
		if err != nil {
			return 0, err
		}
		buf.Write(data)
		buf.WriteByte('\n')
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, &buf)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Splunk "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, fmt.Errorf("hec request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("hec http %d: %s", resp.StatusCode, string(body[:min(len(body), 256)]))
	}

	var ack struct {
		Text  string `json:"text"`
		Code  int    `json:"code"`
	}
	if err := json.Unmarshal(body, &ack); err != nil {
		return len(events), nil
	}
	if ack.Code != 0 {
		return 0, fmt.Errorf("hec rejected batch: %s (code %d)", ack.Text, ack.Code)
	}
	return len(events), nil
}

// StreamFuzzBatch streams fuzzed telemetry variants to HEC.
func (c *HECClient) StreamFuzzBatch(ctx context.Context, variants []FuzzVariant) (int, error) {
	events := make([]map[string]any, 0, len(variants))
	for i, v := range variants {
		rec := copyMap(v.Record)
		rec["aether_variant"] = i + 1
		rec["aether_mutations"] = strings.Join(v.Mutations, ",")
		events = append(events, rec)
	}
	return c.StreamEvents(ctx, events)
}
