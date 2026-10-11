package temper

import (
	"context"
	"io"
	"time"
)

// AuditService reads the security audit log. Each VM has its own append-only hash chain; records contain only
// metadata, never tokens or request bodies.
type AuditService struct{ c *Client }

// List returns a page of audit records in chronological order (From inclusive, To exclusive). Destroyed
// environments can be queried too (records are kept for 90 days).
func (s *AuditService) List(ctx context.Context, environmentID string, p ListAuditParams) (*AuditRecordList, error) {
	var out AuditRecordList
	query := newQuery().str("source", p.Source).str("chain", p.Chain).time("from", p.From).time("to", p.To).
		int("limit", p.Limit).str("after", p.After).values()
	return &out, s.c.do(ctx, request{method: "GET", path: envPath(environmentID, "/audit"), query: query}, &out)
}

// Iter iterates over audit records, fetching pages automatically.
func (s *AuditService) Iter(environmentID string, p ListAuditParams) *Iterator[AuditRecord] {
	return newIterator(p.After, func(ctx context.Context, after string) ([]AuditRecord, string, error) {
		q := p
		q.After = after
		page, err := s.List(ctx, environmentID, q)
		if err != nil {
			return nil, "", err
		}
		return page.Data, deref(page.Next), nil
	})
}

// Export exports the raw rows as NDJSON (ordered by chain and sequence number) and returns a stream (the caller
// must Close it), so the hash chains can be recomputed offline. One export returns at most 100,000 rows: if the
// range holds more, it returns export_too_large; narrow the range with from and to. No timeout by default.
func (s *AuditService) Export(ctx context.Context, environmentID string, from, to time.Time) (io.ReadCloser, error) {
	res, cancel, err := s.c.send(ctx, request{
		method: "GET", path: envPath(environmentID, "/audit/export"), query: newQuery().time("from", from).time("to", to).values(),
		accept: "application/x-ndjson", timeout: -1,
	})
	if err != nil {
		return nil, err
	}
	return cancelOnClose{res.Body, cancel}, nil
}

// Chains reports, for each chain, the latest sequence number received, the head hash, and the sequence number
// from which the chain no longer links up. Intact is false if any chain is broken.
func (s *AuditService) Chains(ctx context.Context, environmentID string) (*AuditChains, error) {
	var out AuditChains
	return &out, s.c.do(ctx, request{method: "GET", path: envPath(environmentID, "/audit/chains")}, &out)
}
