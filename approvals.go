package temper

import (
	"context"
	"time"
)

// PoliciesService manages action policies (the developer default policy and per-environment policies). Rules
// resolve as deny > ask > allow; high-risk categories can't be allowed outright.
type PoliciesService struct{ c *Client }

type rulesBody struct {
	Rules []PolicyRule `json:"rules"`
}

func nonNil(r []PolicyRule) []PolicyRule {
	if r == nil {
		return []PolicyRule{}
	}
	return r
}

// GetDefault returns the developer default policy. If none has been set, Rules is empty and UpdatedAt is nil.
func (s *PoliciesService) GetDefault(ctx context.Context) (*DefaultPolicy, error) {
	var out DefaultPolicy
	return &out, s.c.do(ctx, request{method: "GET", path: "/v1/policy"}, &out)
}

// SetDefault replaces the whole developer default policy. It returns bad_request for invalid rules, such as allow
// on a high-risk category or MaxScope on an effect other than ask.
func (s *PoliciesService) SetDefault(ctx context.Context, rules []PolicyRule) (*DefaultPolicy, error) {
	var out DefaultPolicy
	return &out, s.c.do(ctx, request{method: "PUT", path: "/v1/policy", json: rulesBody{nonNil(rules)}}, &out)
}

// DeleteDefault deletes the developer default policy, going back to the built-in behavior. Idempotent.
func (s *PoliciesService) DeleteDefault(ctx context.Context) error {
	return s.c.do(ctx, request{method: "DELETE", path: "/v1/policy"}, nil)
}

// GetForEnvironment returns the policy an environment actually uses and where it comes from.
func (s *PoliciesService) GetForEnvironment(ctx context.Context, environmentID string) (*EnvironmentPolicy, error) {
	var out EnvironmentPolicy
	return &out, s.c.do(ctx, request{method: "GET", path: envPath(environmentID, "/policy")}, &out)
}

// SetForEnvironment sets the environment's own policy (replacing it entirely; the developer default policy is no
// longer consulted afterwards).
func (s *PoliciesService) SetForEnvironment(ctx context.Context, environmentID string, rules []PolicyRule) (*EnvironmentPolicy, error) {
	var out EnvironmentPolicy
	return &out, s.c.do(ctx, request{method: "PUT", path: envPath(environmentID, "/policy"), json: rulesBody{nonNil(rules)}}, &out)
}

// DeleteForEnvironment deletes the environment's own policy, going back to the developer default policy.
// Idempotent.
func (s *PoliciesService) DeleteForEnvironment(ctx context.Context, environmentID string) error {
	return s.c.do(ctx, request{method: "DELETE", path: envPath(environmentID, "/policy")}, nil)
}

// Check evaluates, read-only, whether an action of this category on this connector would be allowed, need
// approval or be denied, and why. It creates no approval request and consumes no grant.
func (s *PoliciesService) Check(ctx context.Context, environmentID string, p PolicyCheckParams) (*PolicyCheckResult, error) {
	var out PolicyCheckResult
	return &out, s.c.do(ctx, request{method: "POST", path: envPath(environmentID, "/policy/check"), json: p}, &out)
}

// ApprovalsService manages approval requests (approving and denying them) and grants.
type ApprovalsService struct{ c *Client }

// List returns a page of approval requests in ascending creation order. Requests that expired undecided are
// already reported as expired when read.
func (s *ApprovalsService) List(ctx context.Context, p ListApprovalsParams) (*ApprovalList, error) {
	var out ApprovalList
	query := newQuery().str("environment_id", p.EnvironmentID).str("status", p.Status).int("limit", p.Limit).str("after", p.After).values()
	return &out, s.c.do(ctx, request{method: "GET", path: "/v1/approvals", query: query}, &out)
}

// Iter iterates over approval requests, fetching pages automatically.
func (s *ApprovalsService) Iter(p ListApprovalsParams) *Iterator[Approval] {
	return newIterator(p.After, func(ctx context.Context, after string) ([]Approval, string, error) {
		page, err := s.List(ctx, ListApprovalsParams{EnvironmentID: p.EnvironmentID, Status: p.Status, Limit: p.Limit, After: after})
		if err != nil {
			return nil, "", err
		}
		return page.Data, deref(page.Next), nil
	})
}

// Get returns an approval request.
func (s *ApprovalsService) Get(ctx context.Context, id string) (*Approval, error) {
	var out Approval
	return &out, s.c.do(ctx, request{method: "GET", path: "/v1/approvals/" + seg(id)}, &out)
}

// Approve approves a request. scope can't be wider than the request's MaxScope (scope_too_wide); a session or task
// id that doesn't match the request's, or an until that isn't in the future, returns invalid_scope; a request that
// was already decided or has expired returns already_decided. The decision is made with the API key.
func (s *ApprovalsService) Approve(ctx context.Context, id string, scope GrantScope) (*ApprovalDecision, error) {
	var out ApprovalDecision
	body := struct {
		Decision string     `json:"decision"`
		Scope    GrantScope `json:"scope"`
	}{"approve", scope}
	return &out, s.c.do(ctx, request{method: "POST", path: "/v1/approvals/" + seg(id) + "/decision", json: body}, &out)
}

// Deny denies a request.
func (s *ApprovalsService) Deny(ctx context.Context, id string) (*ApprovalDecision, error) {
	var out ApprovalDecision
	body := map[string]string{"decision": "deny"}
	return &out, s.c.do(ctx, request{method: "POST", path: "/v1/approvals/" + seg(id) + "/decision", json: body}, &out)
}

// ListGrants returns active grants (not revoked and, if time-limited, not expired), at most 1000. An empty
// environmentID lists all of them.
func (s *ApprovalsService) ListGrants(ctx context.Context, environmentID string) ([]Grant, error) {
	var out struct {
		Data []Grant `json:"data"`
	}
	query := newQuery().str("environment_id", environmentID).values()
	return out.Data, s.c.do(ctx, request{method: "GET", path: "/v1/grants", query: query}, &out)
}

// RevokeGrant revokes a grant. Idempotent.
func (s *ApprovalsService) RevokeGrant(ctx context.Context, id string) error {
	return s.c.do(ctx, request{method: "DELETE", path: "/v1/grants/" + seg(id)}, nil)
}

// OnceScope allows only the pending request, without leaving a grant.
func OnceScope() GrantScope { return GrantScope{Kind: "once"} }

// PermanentScope grants permanently.
func PermanentScope() GrantScope { return GrantScope{Kind: "permanent"} }

// UntilScope grants until t, which must be in the future. Stored grants call this scope time_limited.
func UntilScope(t time.Time) GrantScope { return GrantScope{Kind: "until", Until: t.Unix()} }

// SessionScope scopes the grant to the approval request's session (the id must be the request's own). It returns
// an error if the request has no session.
func SessionScope(a *Approval) (GrantScope, error) {
	if a.SessionID == nil || *a.SessionID == "" {
		return GrantScope{}, &Error{Code: CodeInvalidScope, Message: "approval " + a.ID + " has no session_id"}
	}
	return GrantScope{Kind: "session", SessionID: *a.SessionID}, nil
}

// TaskScope scopes the grant to the approval request's task (the id must be the request's own). It returns an
// error if the request has no task.
func TaskScope(a *Approval) (GrantScope, error) {
	if a.TaskID == nil || *a.TaskID == "" {
		return GrantScope{}, &Error{Code: CodeInvalidScope, Message: "approval " + a.ID + " has no task_id"}
	}
	return GrantScope{Kind: "task", TaskID: *a.TaskID}, nil
}
