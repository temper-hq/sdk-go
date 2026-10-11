package temper

import "context"

// MeteringService manages usage and monthly limits. An environment over a limit is suspended
// (Environment.OverLimit), and resuming, waking or running exec on it returns limit_exceeded.
type MeteringService struct{ c *Client }

// Usage returns per-environment usage and totals for a time range (by default from the start of the current UTC
// month to now). It returns bad_request if From is not earlier than To.
func (s *MeteringService) Usage(ctx context.Context, p UsageParams) (*Usage, error) {
	var out Usage
	query := newQuery().time("from", p.From).time("to", p.To).str("environment_id", p.EnvironmentID).values()
	return &out, s.c.do(ctx, request{method: "GET", path: "/v1/usage", query: query}, &out)
}

// ListLimits returns all monthly limits (developer-level ones first), with the usage so far this month.
func (s *MeteringService) ListLimits(ctx context.Context) ([]UsageLimit, error) {
	var out struct {
		Data []UsageLimit `json:"data"`
	}
	return out.Data, s.c.do(ctx, request{method: "GET", path: "/v1/limits"}, &out)
}

// SetLimit sets (or replaces) a monthly limit; an empty EnvironmentID sets a developer-level limit. Limits are
// rechecked immediately.
func (s *MeteringService) SetLimit(ctx context.Context, p SetLimitParams) (*UsageLimit, error) {
	var out UsageLimit
	return &out, s.c.do(ctx, request{method: "PUT", path: "/v1/limits", json: p}, &out)
}

// DeleteLimit deletes a monthly limit (an empty environmentID deletes the developer-level one). It returns
// not_found if there is no such limit.
func (s *MeteringService) DeleteLimit(ctx context.Context, metric, environmentID string) error {
	query := newQuery().str("metric", metric).str("environment_id", environmentID).values()
	return s.c.do(ctx, request{method: "DELETE", path: "/v1/limits", query: query}, nil)
}
