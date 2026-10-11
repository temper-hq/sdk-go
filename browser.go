package temper

import "context"

// BrowserService manages the browser pool: profiles (saved login state), browser leases, and end-user takeover.
type BrowserService struct{ c *Client }

// CreateProfile creates a profile (201). It returns profile_exists if the end user already has a profile with
// that name.
func (s *BrowserService) CreateProfile(ctx context.Context, endUserID, name string) (*BrowserProfile, error) {
	var out BrowserProfile
	body := map[string]string{"end_user_id": endUserID, "name": name}
	return &out, s.c.do(ctx, request{method: "POST", path: "/v1/browser/profiles", json: body}, &out)
}

// ListProfiles returns a page of profiles. An empty endUserID lists all of them.
func (s *BrowserService) ListProfiles(ctx context.Context, endUserID string, limit int, after string) (*BrowserProfileList, error) {
	var out BrowserProfileList
	query := newQuery().str("end_user_id", endUserID).int("limit", limit).str("after", after).values()
	return &out, s.c.do(ctx, request{method: "GET", path: "/v1/browser/profiles", query: query}, &out)
}

// GetProfile returns a profile.
func (s *BrowserService) GetProfile(ctx context.Context, id string) (*BrowserProfile, error) {
	var out BrowserProfile
	return &out, s.c.do(ctx, request{method: "GET", path: "/v1/browser/profiles/" + seg(id)}, &out)
}

// DeleteProfile deletes a profile along with its saved login state. It returns profile_in_use if a lease is still
// using it.
func (s *BrowserService) DeleteProfile(ctx context.Context, id string) error {
	return s.c.do(ctx, request{method: "DELETE", path: "/v1/browser/profiles/" + seg(id)}, nil)
}

// AcquireLease acquires a lease (201), valid for 15 minutes.
func (s *BrowserService) AcquireLease(ctx context.Context, p AcquireLeaseParams) (*BrowserLease, error) {
	var out BrowserLease
	return &out, s.c.do(ctx, request{method: "POST", path: "/v1/browser/leases", json: p}, &out)
}

// ListLeases returns a page of leases.
func (s *BrowserService) ListLeases(ctx context.Context, p ListLeasesParams) (*BrowserLeaseList, error) {
	var out BrowserLeaseList
	query := newQuery().str("environment_id", p.EnvironmentID).str("state", p.State).int("limit", p.Limit).str("after", p.After).values()
	return &out, s.c.do(ctx, request{method: "GET", path: "/v1/browser/leases", query: query}, &out)
}

// IterLeases iterates over leases, fetching pages automatically.
func (s *BrowserService) IterLeases(p ListLeasesParams) *Iterator[BrowserLease] {
	return newIterator(p.After, func(ctx context.Context, after string) ([]BrowserLease, string, error) {
		q := p
		q.After = after
		page, err := s.ListLeases(ctx, q)
		if err != nil {
			return nil, "", err
		}
		return page.Data, deref(page.Next), nil
	})
}

// GetLease returns a lease.
func (s *BrowserService) GetLease(ctx context.Context, id string) (*BrowserLease, error) {
	var out BrowserLease
	return &out, s.c.do(ctx, request{method: "GET", path: "/v1/browser/leases/" + seg(id)}, &out)
}

// RenewLease renews a lease for another 15 minutes. It returns lease_not_active if the lease isn't active.
func (s *BrowserService) RenewLease(ctx context.Context, id string) (*BrowserLease, error) {
	var out BrowserLease
	return &out, s.c.do(ctx, request{method: "POST", path: "/v1/browser/leases/" + seg(id) + "/renew"}, &out)
}

// ReleaseLease releases a lease. With save nil, no request body is sent (a write lease saves its login state by
// default: it moves to releasing, then to released once the host sends the login state back, and settles as
// discarded if nothing arrives within 60 seconds; poll GetLease for the outcome). Ptr(false) skips saving and goes
// straight to released.
func (s *BrowserService) ReleaseLease(ctx context.Context, id string, save *bool) (*BrowserLease, error) {
	var out BrowserLease
	var body any
	if save != nil {
		body = map[string]bool{"save": *save}
	}
	return &out, s.c.do(ctx, request{method: "POST", path: "/v1/browser/leases/" + seg(id) + "/release", json: body}, &out)
}

// Takeover hands the browser to the end user, who logs in or solves a CAPTCHA on the real page; meanwhile all of
// the agent's browser commands are rejected. It returns a single-use view URL for the end user to open; calling it
// again issues a new URL. It returns lease_not_active if the lease isn't active and host_unavailable if the host is
// offline.
func (s *BrowserService) Takeover(ctx context.Context, id string) (*BrowserTakeover, error) {
	var out BrowserTakeover
	return &out, s.c.do(ctx, request{method: "POST", path: "/v1/browser/leases/" + seg(id) + "/takeover"}, &out)
}

// Handback hands control back to the agent (same as the end user clicking "Hand back" on the view page). It
// returns not_taken_over if no takeover is in progress.
func (s *BrowserService) Handback(ctx context.Context, id string) (*BrowserLease, error) {
	var out BrowserLease
	return &out, s.c.do(ctx, request{method: "POST", path: "/v1/browser/leases/" + seg(id) + "/handback"}, &out)
}

// Limits returns the lease concurrency limits and the lease lifetime.
func (s *BrowserService) Limits(ctx context.Context) (*BrowserLimits, error) {
	var out BrowserLimits
	return &out, s.c.do(ctx, request{method: "GET", path: "/v1/browser/limits"}, &out)
}
