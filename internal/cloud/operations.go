package cloud

import (
	"context"
	"encoding/json"
)

// operationsPathPrefix is where long-running operations are polled. The
// assets API returns relative paths like "operations/{uuid}" and documents
// GET https://apis.roblox.com/assets/v1/operations/{id}.
const operationsPathPrefix = "/assets/v1/"

// operation is the Open Cloud long-running-operation envelope.
type operation struct {
	Path  string `json:"path"`
	Done  bool   `json:"done"`
	Error *struct {
		Code    json.RawMessage `json:"code"`
		Message string          `json:"message"`
	} `json:"error"`
	Response json.RawMessage `json:"response"`
}

// PollOperation polls GET {baseURL}/assets/v1/{path} (path as returned by
// CreateAsset et al., e.g. "operations/abc-123") with exponential backoff
// until the operation reports done, then decodes its `response` field into
// `into` (skipped when into is nil). A done operation carrying an error is
// surfaced as an *APIError with StatusCode 0 — the HTTP exchange succeeded;
// the operation itself failed.
func (c *Client) PollOperation(ctx context.Context, path string, into any) error {
	delay := c.pollBase
	for {
		var op operation
		if err := c.do(ctx, "GET", operationsPathPrefix+path, nil, "", nil, &op); err != nil {
			return err
		}
		if op.Done {
			if op.Error != nil {
				return &APIError{Code: rawToString(op.Error.Code), Message: op.Error.Message}
			}
			if into != nil && len(op.Response) > 0 {
				return json.Unmarshal(op.Response, into)
			}
			return nil
		}
		if err := sleep(ctx, delay); err != nil {
			return err
		}
		if delay *= 2; delay > c.pollCap {
			delay = c.pollCap
		}
	}
}
