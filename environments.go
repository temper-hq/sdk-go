package temper

import (
	"context"
	"strconv"
	"time"
)

// EnvironmentsService creates, reads and lists environments, changes their state, and provides their events and
// synchronous exec.
type EnvironmentsService struct{ c *Client }

// Create creates an environment (201). If the end user already has an environment that isn't destroyed, it
// returns environment_exists, with the existing environment's id in the *Error's EnvironmentID. To retry safely
// after a timeout, set p.IdempotencyKey (generate one per logical create, such as a UUID): a retry with the same
// key returns the environment from the first create; using the same key for a different end user returns
// idempotency_key_reused.
func (s *EnvironmentsService) Create(ctx context.Context, p CreateEnvironmentParams) (*Environment, error) {
	var out Environment
	r := request{method: "POST", path: "/v1/environments", json: p}
	if p.IdempotencyKey != "" {
		r.headers = map[string]string{"Idempotency-Key": p.IdempotencyKey}
	}
	return &out, s.c.do(ctx, r, &out)
}

// Get returns an environment (including destroyed ones).
func (s *EnvironmentsService) Get(ctx context.Context, id string) (*Environment, error) {
	var out Environment
	return &out, s.c.do(ctx, request{method: "GET", path: envPath(id, "")}, &out)
}

// GetConfig returns the environment's current configuration (environment variables and files).
func (s *EnvironmentsService) GetConfig(ctx context.Context, id string) (*EnvironmentConfig, error) {
	var out EnvironmentConfig
	return &out, s.c.do(ctx, request{method: "GET", path: envPath(id, "/config")}, &out)
}

// SetConfig replaces the environment's whole configuration (zero values clear it). It doesn't wake the
// environment or replace the VM; files are updated within about 10 seconds, and environment variables apply to
// processes started afterwards.
func (s *EnvironmentsService) SetConfig(ctx context.Context, id string, config EnvironmentConfig) (*EnvironmentConfig, error) {
	if config.Env == nil {
		config.Env = map[string]string{}
	}
	if config.Files == nil {
		config.Files = map[string]ConfigFile{}
	}
	var out EnvironmentConfig
	return &out, s.c.do(ctx, request{method: "PUT", path: envPath(id, "/config"), json: config}, &out)
}

// WaitOptions are options for WaitFor. Zero values use the defaults: wait up to 2 minutes, checking once a second.
type WaitOptions struct {
	Timeout  time.Duration
	Interval time.Duration
}

// WaitFor waits until the environment's actual state becomes state and returns the environment at that point. It
// currently polls; it will switch to long polling once the server supports it, with no change in usage. If the
// environment fails while waiting (and state isn't failed), it returns CodeEnvironmentFailed; if it is destroyed
// (desired state destroyed, and state isn't destroyed), CodeEnvironmentDestroyed; on timeout, CodeWaitTimeout.
// All three have Status 0. If ctx is canceled, it returns ctx's error.
// Typical use: WaitFor(ctx, id, "running", nil) after Resume.
func (s *EnvironmentsService) WaitFor(ctx context.Context, id, state string, opts *WaitOptions) (*Environment, error) {
	o := WaitOptions{Timeout: 2 * time.Minute, Interval: time.Second}
	if opts != nil {
		if opts.Timeout > 0 {
			o.Timeout = opts.Timeout
		}
		if opts.Interval > 0 {
			o.Interval = opts.Interval
		}
	}
	deadline := time.Now().Add(o.Timeout)
	for {
		env, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		switch {
		case env.State == state:
			return env, nil
		case env.State == "failed":
			return nil, &Error{Code: CodeEnvironmentFailed, Message: "environment " + id + " failed"}
		case env.DesiredState == "destroyed" && state != "destroyed":
			return nil, &Error{Code: CodeEnvironmentDestroyed, Message: "environment " + id + " was destroyed"}
		case time.Now().Add(o.Interval).After(deadline):
			return nil, &Error{Code: CodeWaitTimeout, Message: "environment " + id + " is " + env.State + ", not " + state + ", after " + o.Timeout.String()}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(o.Interval):
		}
	}
}

// List returns a page of environments (including destroyed ones) in ascending creation order.
func (s *EnvironmentsService) List(ctx context.Context, p ListEnvironmentsParams) (*EnvironmentList, error) {
	var out EnvironmentList
	query := newQuery().str("state", p.State).int("limit", p.Limit).str("after", p.After).values()
	return &out, s.c.do(ctx, request{method: "GET", path: "/v1/environments", query: query}, &out)
}

// Iter iterates over all environments, fetching pages automatically.
func (s *EnvironmentsService) Iter(p ListEnvironmentsParams) *Iterator[Environment] {
	return newIterator(p.After, func(ctx context.Context, after string) ([]Environment, string, error) {
		page, err := s.List(ctx, ListEnvironmentsParams{State: p.State, Limit: p.Limit, After: after})
		if err != nil {
			return nil, "", err
		}
		return page.Data, deref(page.Next), nil
	})
}

func (s *EnvironmentsService) action(ctx context.Context, id, action string, body any) (*Environment, error) {
	var out Environment
	return &out, s.c.do(ctx, request{method: "POST", path: envPath(id, "/"+action), json: body}, &out)
}

// Suspend suspends the environment (memory snapshot). Idempotent; it returns invalid_state_transition if the
// environment is stopped or destroyed.
func (s *EnvironmentsService) Suspend(ctx context.Context, id string) (*Environment, error) {
	return s.action(ctx, id, "suspend", nil)
}

// CreateSnapshot snapshots the data disk (/workspace) without stopping the environment. It is returned pending and
// becomes ready once the host has taken it (about a second); a running environment's /workspace is frozen for that
// moment.
func (s *EnvironmentsService) CreateSnapshot(ctx context.Context, id string, p CreateSnapshotParams) (*Snapshot, error) {
	var out Snapshot
	return &out, s.c.do(ctx, request{method: "POST", path: envPath(id, "/snapshots"), json: p}, &out)
}

// ListSnapshots returns the environment's snapshots, newest first.
func (s *EnvironmentsService) ListSnapshots(ctx context.Context, id string) ([]Snapshot, error) {
	var out struct {
		Data []Snapshot `json:"data"`
	}
	return out.Data, s.c.do(ctx, request{method: "GET", path: envPath(id, "/snapshots")}, &out)
}

// GetSnapshot returns one snapshot.
func (s *EnvironmentsService) GetSnapshot(ctx context.Context, id, snapshotID string) (*Snapshot, error) {
	var out Snapshot
	return &out, s.c.do(ctx, request{method: "GET", path: envPath(id, "/snapshots/"+seg(snapshotID))}, &out)
}

// DeleteSnapshot deletes a snapshot (deleting until the host has removed it). Environments branched from it are
// not affected.
func (s *EnvironmentsService) DeleteSnapshot(ctx context.Context, id, snapshotID string) (*Snapshot, error) {
	var out Snapshot
	return &out, s.c.do(ctx, request{method: "DELETE", path: envPath(id, "/snapshots/"+seg(snapshotID))}, &out)
}

// Restore restores the data disk to a snapshot: the VM is shut down, the disk swapped and the VM cold-started (the
// in-memory state is lost). With keepCurrent the current disk is kept as a snapshot first. Restoring stays true
// until the host has finished.
func (s *EnvironmentsService) Restore(ctx context.Context, id, snapshotID string, keepCurrent bool) (*Environment, error) {
	return s.action(ctx, id, "restore", map[string]any{"snapshot_id": snapshotID, "keep_current": keepCurrent})
}

// Resume brings the environment back to running (without recording activity). It returns limit_exceeded if a
// usage limit has been exceeded.
func (s *EnvironmentsService) Resume(ctx context.Context, id string) (*Environment, error) {
	return s.action(ctx, id, "resume", nil)
}

// Wake wakes the environment; call it when a request arrives. A suspended or stopped environment is resumed; for
// a running one it only records activity.
func (s *EnvironmentsService) Wake(ctx context.Context, id string) (*Environment, error) {
	return s.action(ctx, id, "wake", nil)
}

// KeepAwake keeps the environment awake for the given number of seconds (1–86400), waking it first if it isn't
// running.
func (s *EnvironmentsService) KeepAwake(ctx context.Context, id string, seconds int) (*Environment, error) {
	return s.action(ctx, id, "keep-awake", map[string]int{"seconds": seconds})
}

// Destroy destroys the environment, deleting its data disk and backups; this can't be undone. Idempotent.
func (s *EnvironmentsService) Destroy(ctx context.Context, id string) (*Environment, error) {
	var out Environment
	return &out, s.c.do(ctx, request{method: "DELETE", path: envPath(id, "")}, &out)
}

// UpdateIdle changes the idle thresholds; only the fields that are set are changed.
func (s *EnvironmentsService) UpdateIdle(ctx context.Context, id string, p IdleSettings) (*Environment, error) {
	var out Environment
	return &out, s.c.do(ctx, request{method: "PATCH", path: envPath(id, "/idle"), json: p}, &out)
}

// AgentLogs returns the Agent's recent output (the runtime keeps the last 8 MiB since the VM started) and its state.
// Wakes a suspended environment, like exec.
func (s *EnvironmentsService) AgentLogs(ctx context.Context, id string, p AgentLogsParams) (*AgentLogs, error) {
	var out AgentLogs
	query := newQuery().int64("since", p.Since).int("limit", p.Limit).values()
	return &out, s.c.do(ctx, request{method: "GET", path: envPath(id, "/agent/logs"), query: query}, &out)
}

// Events returns a page of environment events in chronological order. Destroyed environments can be queried too.
func (s *EnvironmentsService) Events(ctx context.Context, id string, p ListEventsParams) (*EventList, error) {
	var out EventList
	query := newQuery().int64("after", p.After).int("limit", p.Limit).str("kind", p.Kind).values()
	return &out, s.c.do(ctx, request{method: "GET", path: envPath(id, "/events"), query: query}, &out)
}

// IterEvents iterates over environment events, fetching pages automatically.
func (s *EnvironmentsService) IterEvents(id string, p ListEventsParams) *Iterator[EnvironmentEvent] {
	start := ""
	if p.After != 0 {
		start = strconv.FormatInt(p.After, 10)
	}
	return newIterator(start, func(ctx context.Context, after string) ([]EnvironmentEvent, string, error) {
		var a int64
		if after != "" {
			a, _ = strconv.ParseInt(after, 10, 64)
		}
		page, err := s.Events(ctx, id, ListEventsParams{After: a, Limit: p.Limit, Kind: p.Kind})
		if err != nil {
			return nil, "", err
		}
		next := ""
		if page.Next != nil {
			next = strconv.FormatInt(*page.Next, 10)
		}
		return page.Data, next, nil
	})
}

// Exec runs a command synchronously and waits for it to finish (a non-zero exit code is still a normal result).
// An environment that isn't running is woken first, waiting up to 90 seconds: it returns environment_not_placed if
// no host is assigned within 15 seconds, and environment_not_running if it isn't up within 90 seconds.
// The default timeout is 105 seconds for the wake-up + TimeoutSecs (default 60) + 30 seconds.
func (s *EnvironmentsService) Exec(ctx context.Context, id string, p ExecParams) (*ExecResult, error) {
	t := p.TimeoutSecs
	if t == 0 {
		t = 60
	}
	var out ExecResult
	budget := wakeBudget + time.Duration(t+30)*time.Second
	return &out, s.c.do(ctx, request{method: "POST", path: envPath(id, "/exec"), json: p, timeout: budget}, &out)
}

// CreateBatch creates 1–200 environments, one at a time. Failures are reported in the corresponding results
// (check OK()) rather than as an error; an error is returned only if the request as a whole is invalid.
func (s *EnvironmentsService) CreateBatch(ctx context.Context, envs []CreateEnvironmentParams) ([]BatchCreateResult, error) {
	var out struct {
		Results []BatchCreateResult `json:"results"`
	}
	body := map[string][]CreateEnvironmentParams{"environments": envs}
	return out.Results, s.c.do(ctx, request{method: "POST", path: "/v1/environments/batch", json: body}, &out)
}

// CreateWakeup schedules a one-time wake-up (201). Each environment can have at most 100 pending wake-ups;
// beyond that it returns wakeup_limit_reached.
func (s *EnvironmentsService) CreateWakeup(ctx context.Context, id string, at time.Time, reason string) (*Wakeup, error) {
	var out Wakeup
	body := struct {
		At     string `json:"at"`
		Reason string `json:"reason,omitempty"`
	}{at.UTC().Format(time.RFC3339Nano), reason}
	return &out, s.c.do(ctx, request{method: "POST", path: envPath(id, "/wakeups"), json: body}, &out)
}

// ListWakeups returns the scheduled wake-ups that haven't fired or been canceled, in time order.
func (s *EnvironmentsService) ListWakeups(ctx context.Context, id string) ([]Wakeup, error) {
	var out struct {
		Data []Wakeup `json:"data"`
	}
	return out.Data, s.c.do(ctx, request{method: "GET", path: envPath(id, "/wakeups")}, &out)
}

// CancelWakeup cancels a scheduled wake-up. It returns not_found if the wake-up already fired or was canceled.
func (s *EnvironmentsService) CancelWakeup(ctx context.Context, id, wakeupID string) error {
	return s.c.do(ctx, request{method: "DELETE", path: envPath(id, "/wakeups/"+seg(wakeupID))}, nil)
}

// ListSchedules returns the recurring schedules (registered by the agent or by the developer).
func (s *EnvironmentsService) ListSchedules(ctx context.Context, id string) ([]Schedule, error) {
	var out struct {
		Data []Schedule `json:"data"`
	}
	return out.Data, s.c.do(ctx, request{method: "GET", path: envPath(id, "/schedules")}, &out)
}

// CancelSchedule cancels a schedule (including one registered by the agent).
func (s *EnvironmentsService) CancelSchedule(ctx context.Context, id, scheduleID string) error {
	return s.c.do(ctx, request{method: "DELETE", path: envPath(id, "/schedules/"+seg(scheduleID))}, nil)
}

// UpdateAgentSettings changes the agent's quotas for registering its own schedules and keeping the environment
// awake. Only the fields that are set are changed; it returns all current values. Pass an empty
// AgentSettingsPatch to just read the current values (there is no separate read endpoint; this records an event).
func (s *EnvironmentsService) UpdateAgentSettings(ctx context.Context, id string, p AgentSettingsPatch) (*AgentSettings, error) {
	var out AgentSettings
	return &out, s.c.do(ctx, request{method: "PATCH", path: envPath(id, "/agent"), json: p}, &out)
}

// ListIngressRoutes returns the ingress routes (public HTTPS address → a port inside the VM), yours and the
// agent's, plus the environment's public hostname.
func (s *EnvironmentsService) ListIngressRoutes(ctx context.Context, id string) (*IngressRouteList, error) {
	var out IngressRouteList
	return &out, s.c.do(ctx, request{method: "GET", path: envPath(id, "/ingress/routes")}, &out)
}

// SetIngressRoute registers (or replaces) an ingress route: requests to https://<host><path> are verified, stored,
// answered, then delivered to the port inside the VM (waking it first). The secret never enters the VM.
func (s *EnvironmentsService) SetIngressRoute(ctx context.Context, id, key string, p IngressRouteSpec) (*IngressRoute, error) {
	var out IngressRoute
	return &out, s.c.do(ctx, request{method: "PUT", path: envPath(id, "/ingress/routes/"+seg(key)), json: p}, &out)
}

// DeleteIngressRoute deletes an ingress route (including one the agent registered).
func (s *EnvironmentsService) DeleteIngressRoute(ctx context.Context, id, key string) error {
	return s.c.do(ctx, request{method: "DELETE", path: envPath(id, "/ingress/routes/"+seg(key))}, nil)
}

// ListIngressDeliveries returns recent ingress requests (newest first, at most limit; 0 = the default 50) and
// whether they reached the VM. Bodies are not included.
func (s *EnvironmentsService) ListIngressDeliveries(ctx context.Context, id string, limit int) ([]IngressDelivery, error) {
	var out struct {
		Data []IngressDelivery `json:"data"`
	}
	query := newQuery().int("limit", limit).values()
	return out.Data, s.c.do(ctx, request{method: "GET", path: envPath(id, "/ingress/deliveries"), query: query}, &out)
}
