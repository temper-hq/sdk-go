package temper

import (
	"context"
	"io"
	"net/http"
)

// AgentService manages agent packages and rollouts.
type AgentService struct{ c *Client }

// UploadPackage uploads an agent package (squashfs, up to 2 GiB), streaming body as the upload, with no timeout
// by default. created is false if this version already exists with identical content; if the content differs, it
// returns version_exists.
func (s *AgentService) UploadPackage(ctx context.Context, version string, body io.Reader) (pkg *Package, created bool, err error) {
	res, cancel, err := s.c.send(ctx, request{
		method: "PUT", path: "/v1/agent/packages/" + seg(version), body: body, ctype: "application/octet-stream", timeout: -1,
	})
	if err != nil {
		return nil, false, err
	}
	defer cancel()
	defer res.Body.Close()
	var out Package
	if err := decode(res, &out); err != nil {
		return nil, false, err
	}
	return &out, res.StatusCode == http.StatusCreated, nil
}

// ListPackages returns all packages (in upload order) and the default version.
func (s *AgentService) ListPackages(ctx context.Context) (*PackageList, error) {
	var out PackageList
	return &out, s.c.do(ctx, request{method: "GET", path: "/v1/agent/packages"}, &out)
}

// SetDefaultVersion sets the default version for new environments. It returns bad_request if the version hasn't
// been uploaded.
func (s *AgentService) SetDefaultVersion(ctx context.Context, version string) error {
	return s.c.do(ctx, request{method: "PUT", path: "/v1/agent/default", json: map[string]string{"version": version}}, nil)
}

// StartRollout starts a rollout (201). It returns rollout_in_progress if a rollout is already active or paused.
func (s *AgentService) StartRollout(ctx context.Context, p StartRolloutParams) (*Rollout, error) {
	var out Rollout
	return &out, s.c.do(ctx, request{method: "POST", path: "/v1/agent/rollouts", json: p}, &out)
}

// ListRollouts returns the 100 most recent rollouts, newest first.
func (s *AgentService) ListRollouts(ctx context.Context) ([]Rollout, error) {
	var out struct {
		Data []Rollout `json:"data"`
	}
	return out.Data, s.c.do(ctx, request{method: "GET", path: "/v1/agent/rollouts"}, &out)
}

// GetRollout returns a rollout and the progress of each environment it has upgraded.
func (s *AgentService) GetRollout(ctx context.Context, id string) (*RolloutDetail, error) {
	var out RolloutDetail
	return &out, s.c.do(ctx, request{method: "GET", path: "/v1/agent/rollouts/" + seg(id)}, &out)
}

// WidenRollout increases the rollout percentage (it can only go up).
func (s *AgentService) WidenRollout(ctx context.Context, id string, percent int) (*Rollout, error) {
	var out Rollout
	return &out, s.c.do(ctx, request{method: "PATCH", path: "/v1/agent/rollouts/" + seg(id), json: map[string]int{"percent": percent}}, &out)
}

func (s *AgentService) rolloutAction(ctx context.Context, id, action string) (*Rollout, error) {
	var out Rollout
	return &out, s.c.do(ctx, request{method: "POST", path: "/v1/agent/rollouts/" + seg(id) + "/" + action}, &out)
}

// PauseRollout pauses a rollout. Only an active rollout can be paused; not idempotent.
func (s *AgentService) PauseRollout(ctx context.Context, id string) (*Rollout, error) {
	return s.rolloutAction(ctx, id, "pause")
}

// ResumeRollout resumes a rollout. Only a paused rollout can be resumed; not idempotent.
func (s *AgentService) ResumeRollout(ctx context.Context, id string) (*Rollout, error) {
	return s.rolloutAction(ctx, id, "resume")
}

// RollbackRollout rolls a rollout back manually. Only the most recent rollout can be rolled back; otherwise it
// returns not_latest_rollout.
func (s *AgentService) RollbackRollout(ctx context.Context, id string) (*Rollout, error) {
	return s.rolloutAction(ctx, id, "rollback")
}
