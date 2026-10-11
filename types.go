package temper

import (
	"encoding/base64"
	"encoding/json"
	"time"
)

// ---- Environments ----

// Resources describes an environment's resources.
type Resources struct {
	CPUs          int `json:"cpus"`
	MemoryMiB     int `json:"memory_mib"`
	BaseMemoryMiB int `json:"base_memory_mib"`
	DiskGiB       int `json:"disk_gib"`
}

// IdleState is the current idle ladder configuration.
type IdleState struct {
	SuspendAfterSecs int        `json:"suspend_after_secs"`
	StopAfterSecs    int        `json:"stop_after_secs"`
	KeepAwakeUntil   *time.Time `json:"keep_awake_until"`
}

// Environment is one end user's microVM plus data disk.
type Environment struct {
	ID           string  `json:"id"`
	EndUserID    string  `json:"end_user_id"`
	AgentVersion *string `json:"agent_version"`
	// Template is the template version the environment runs ("name:version"); nil = the platform's default runtime.
	Template  *string   `json:"template"`
	Resources Resources `json:"resources"`
	// DesiredState is running, suspended, stopped or destroyed.
	DesiredState string `json:"desired_state"`
	// State is the actual state (reported by the host): pending, running, suspended, stopped, destroyed or failed.
	State        string    `json:"state"`
	Idle         IdleState `json:"idle"`
	LastActiveAt time.Time `json:"last_active_at"`
	// OverLimit is the monthly limit that was exceeded (a human-readable reason), or nil. An environment over a
	// limit is suspended, and resuming, waking or running exec on it returns limit_exceeded.
	OverLimit *string   `json:"over_limit"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ResourcesInput is the resources requested when creating an environment; fields left zero use server defaults.
type ResourcesInput struct {
	CPUs          int `json:"cpus,omitempty"`
	MemoryMiB     int `json:"memory_mib,omitempty"`
	BaseMemoryMiB int `json:"base_memory_mib,omitempty"`
	DiskGiB       int `json:"disk_gib,omitempty"`
}

// IdleSettings are idle thresholds in seconds. 0 means that step never happens automatically, so pointers are
// used to tell "don't change" apart from "set to 0".
type IdleSettings struct {
	SuspendAfterSecs *int `json:"suspend_after_secs,omitempty"`
	StopAfterSecs    *int `json:"stop_after_secs,omitempty"`
}

// CreateEnvironmentParams are the parameters for creating an environment.
type CreateEnvironmentParams struct {
	EndUserID string `json:"end_user_id"`
	// AgentVersion defaults to the developer's default version.
	AgentVersion string          `json:"agent_version,omitempty"`
	Resources    *ResourcesInput `json:"resources,omitempty"`
	Idle         *IdleSettings   `json:"idle,omitempty"`
	// Env and Files are non-secret environment variables and a few files; see EnvironmentConfig.
	Env   map[string]string     `json:"env,omitempty"`
	Files map[string]ConfigFile `json:"files,omitempty"`
	// Secrets are environment-level secrets, created in the same transaction as the environment; if any of them is
	// invalid, the environment isn't created.
	Secrets map[string]PutSecretParams `json:"secrets,omitempty"`
	// EgressAllow is the environment's own egress allowlist; nil uses the developer-level one.
	EgressAllow []string `json:"egress_allow,omitempty"`
	// Template is a template version as "name:version" (it must be ready); empty = the platform's default runtime.
	Template string `json:"template,omitempty"`
	// IdempotencyKey (1–255 visible ASCII characters) is sent as the Idempotency-Key header. Only Create uses it;
	// CreateBatch doesn't.
	IdempotencyKey string `json:"-"`
}

// ConfigFile is a configuration file. Use FileBytes for binary content.
type ConfigFile struct {
	Content string `json:"content"`
	// Encoding is empty for utf8; base64 means Content is base64-encoded.
	Encoding string `json:"encoding,omitempty"`
}

// FileBytes wraps arbitrary bytes as a configuration file sent as base64.
func FileBytes(b []byte) ConfigFile {
	return ConfigFile{Content: base64.StdEncoding.EncodeToString(b), Encoding: "base64"}
}

// EnvironmentConfig is an environment's non-secret configuration. Env is written to /run/temper/env in the
// runtime unit (read by the agent service, exec and login shells; it overrides the agent package's
// /opt/agent/env), and files appear read-only at /run/temper/files/<name>. Limits: 100 variables (32 KiB in
// total) and 20 files (256 KiB in total). The platform's proxy and certificate variables, HOME, USER, LOGNAME
// and names starting with TEMPER_ can't be set; values can't contain single quotes or control characters.
// Put secret values in secrets instead.
type EnvironmentConfig struct {
	Env   map[string]string     `json:"env"`
	Files map[string]ConfigFile `json:"files"`
}

// ListEnvironmentsParams are the parameters for listing environments.
type ListEnvironmentsParams struct {
	// State lists only environments whose actual state is this value.
	State string
	// Limit defaults to 50; the server clamps it to 1–200.
	Limit int
	After string
}

// EnvironmentList is a page of environments.
type EnvironmentList struct {
	Data []Environment `json:"data"`
	Next *string       `json:"next"`
}

// EnvironmentEvent is an environment event.
type EnvironmentEvent struct {
	// ID is an increasing integer cursor.
	ID     int64           `json:"id"`
	Kind   string          `json:"kind"`
	Detail json.RawMessage `json:"detail"`
	At     time.Time       `json:"at"`
}

// EventList is a page of environment events. When Next is non-nil, pass it back as After.
type EventList struct {
	Data []EnvironmentEvent `json:"data"`
	Next *int64             `json:"next"`
}

// ListEventsParams are the parameters for listing environment events.
type ListEventsParams struct {
	After int64
	// Limit defaults to 100; the server clamps it to 1–200.
	Limit int
	Kind  string
}

// ---- Exec and files ----

// ExecParams are the parameters for synchronous exec. Command is run with bash -lc and Argv is executed as is;
// set exactly one of the two.
type ExecParams struct {
	Command string            `json:"command,omitempty"`
	Argv    []string          `json:"argv,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Cwd     string            `json:"cwd,omitempty"`
	// TimeoutSecs is 1–600; the default is 60.
	TimeoutSecs int `json:"timeout_secs,omitempty"`
	// Stdin is written to the process's standard input, which is then closed.
	Stdin string `json:"stdin,omitempty"`
}

// ExecResult is the result of a synchronous exec; a non-zero exit code is still a normal result.
type ExecResult struct {
	// ExitCode is nil if the process was killed by a signal (see Signal).
	ExitCode        *int   `json:"exit_code"`
	Signal          *int   `json:"signal"`
	TimedOut        bool   `json:"timed_out"`
	Stdout          string `json:"stdout"`
	Stderr          string `json:"stderr"`
	StdoutTruncated bool   `json:"stdout_truncated"`
	StderrTruncated bool   `json:"stderr_truncated"`
	DurationMs      int64  `json:"duration_ms"`
}

// SpawnParams are the parameters for starting a background process.
type SpawnParams struct {
	Command string            `json:"command,omitempty"`
	Argv    []string          `json:"argv,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Cwd     string            `json:"cwd,omitempty"`
	// TimeoutSecs is the maximum run time in seconds; 0 means no limit.
	TimeoutSecs int `json:"timeout_secs,omitempty"`
}

// ProcessExit is the exit status of a background process.
type ProcessExit struct {
	// Code is nil if the process was killed by a signal.
	Code     *int `json:"code"`
	Signal   *int `json:"signal,omitempty"`
	TimedOut bool `json:"timed_out"`
}

// Process is a background process. It is tracked in VM memory: it survives suspend and resume, but is lost when
// the environment moves to a different VM.
type Process struct {
	ID string `json:"id"`
	// Argv is the argv actually executed.
	Argv []string `json:"argv"`
	// StartedAt is the start time in Unix seconds.
	StartedAt   int64        `json:"started_at"`
	Running     bool         `json:"running"`
	Exit        *ProcessExit `json:"exit,omitempty"`
	OutputBytes int64        `json:"output_bytes"`
	OutputStart int64        `json:"output_start"`
	StdinOpen   bool         `json:"stdin_open"`
}

// OutputChunk is a chunk of output.
type OutputChunk struct {
	// Stream is stdout or stderr.
	Stream string `json:"stream"`
	// Offset is the offset of the chunk's first byte.
	Offset int64  `json:"offset"`
	Data   string `json:"data"`
}

// ProcessOutput is the result of one output fetch; pass Next as since in the next call.
type ProcessOutput struct {
	Process Process       `json:"process"`
	Chunks  []OutputChunk `json:"chunks"`
	Next    int64         `json:"next"`
	Exited  bool          `json:"exited"`
	Exit    *ProcessExit  `json:"exit"`
}

// WriteFileResult is the result of writing a file.
type WriteFileResult struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// DirectoryEntry is one entry in a directory (symlinks aren't followed).
type DirectoryEntry struct {
	Name string `json:"name"`
	// Type is file, directory, symlink or other.
	Type       string     `json:"type"`
	Size       int64      `json:"size"`
	ModifiedAt *time.Time `json:"modified_at"`
	// Mode is the permission bits in octal, such as 644.
	Mode string `json:"mode"`
}

// DirectoryListing is the result of listing a directory.
type DirectoryListing struct {
	Path    string           `json:"path"`
	Entries []DirectoryEntry `json:"entries"`
	// Truncated is true if the directory has more than 10,000 entries and the rest were cut off.
	Truncated bool `json:"truncated"`
}

// ---- Connections ----

// AuthorizeConnectionParams are the parameters for creating an authorization link.
type AuthorizeConnectionParams struct {
	EndUserID string `json:"end_user_id"`
	// Connectors must all belong to one provider: gmail and calendar (google), slack (slack).
	Connectors []string `json:"connectors"`
	// ReturnURL is where the user is sent after authorizing, with status=ok&connection_id=… or
	// status=error&error=… appended.
	ReturnURL string `json:"return_url"`
}

// ConnectionAuthorization is an authorization link (valid for 10 minutes, single use).
type ConnectionAuthorization struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Connection is one end user's authorization with one provider. It never includes tokens.
type Connection struct {
	ID         string   `json:"id"`
	EndUserID  string   `json:"end_user_id"`
	Provider   string   `json:"provider"`
	Connectors []string `json:"connectors"`
	Scopes     []string `json:"scopes"`
	// Account is the email address for Google and the team for Slack.
	Account *string `json:"account"`
	// Status is active, revoked or error.
	Status          string     `json:"status"`
	Error           *string    `json:"error"`
	AccessExpiresAt *time.Time `json:"access_expires_at"`
	RefreshedAt     *time.Time `json:"refreshed_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	RevokedAt       *time.Time `json:"revoked_at"`
}

// ConnectionList is a list of connections (at most 500, not paginated).
type ConnectionList struct {
	Data []Connection `json:"data"`
	// HasMore is true if there are more than 500 connections and the list is incomplete.
	HasMore bool `json:"has_more"`
}

// OAuthClient is the OAuth app setting for one provider.
type OAuthClient struct {
	Provider string `json:"provider"`
	// ClientID is the app the developer registered, or nil if none is registered (the platform test app is used).
	ClientID             *string    `json:"client_id"`
	UpdatedAt            *time.Time `json:"updated_at"`
	PlatformAppAvailable bool       `json:"platform_app_available"`
	// RedirectURI is the redirect URI registered with the provider.
	RedirectURI string `json:"redirect_uri"`
}

// OAuthClientRegistered is the result of registering an OAuth app.
type OAuthClientRegistered struct {
	Provider    string `json:"provider"`
	ClientID    string `json:"client_id"`
	RedirectURI string `json:"redirect_uri"`
}

// ---- Policies and approvals ----

// PolicyRule is a policy rule. An empty Connector or Category matches any value. MaxScope can only be set on ask
// rules; if it isn't set, it is permanent.
type PolicyRule struct {
	Connector string `json:"connector,omitempty"`
	// Category is read, write, send, pay, delete or change_permission.
	Category string `json:"category,omitempty"`
	// Effect is allow, deny or ask.
	Effect string `json:"effect"`
	// MaxScope is once, task, session, time_limited or permanent.
	MaxScope string `json:"max_scope,omitempty"`
}

// DefaultPolicy is the developer default policy; UpdatedAt is nil if it has never been set.
type DefaultPolicy struct {
	Rules     []PolicyRule `json:"rules"`
	UpdatedAt *time.Time   `json:"updated_at"`
}

// EnvironmentPolicy is the policy an environment actually uses.
type EnvironmentPolicy struct {
	EnvironmentID string `json:"environment_id"`
	// Source is environment, developer or default (the built-in behavior, with empty Rules).
	Source string       `json:"source"`
	Rules  []PolicyRule `json:"rules"`
}

// PolicyCheckParams are the parameters for a read-only policy check.
type PolicyCheckParams struct {
	// Connector is a built-in connector (gmail, calendar, slack, browser) or a developer secret as secret:NAME.
	Connector string `json:"connector"`
	Category  string `json:"category"`
	// Task- and session-scoped grants count only when these IDs match.
	TaskID    string `json:"task_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// PolicyCheckResult is the result of a read-only policy check (no approval request is created and no grant is
// consumed).
type PolicyCheckResult struct {
	// Decision is allow, ask or deny.
	Decision string `json:"decision"`
	// Reason is rule, grant, deny_rule, ask_rule, high_risk, no_rule or infrastructure_credential.
	Reason string `json:"reason"`
	// MaxScope is the widest grant the user can give when approval is required.
	MaxScope string `json:"max_scope,omitempty"`
	// GrantID is the grant that was used.
	GrantID string `json:"grant_id,omitempty"`
	// MatchedRules are the indexes (0-based) of the matching rules in the effective policy.
	MatchedRules []int  `json:"matched_rules"`
	Source       string `json:"source"`
}

// Approval is an approval request.
type Approval struct {
	ID            string  `json:"id"`
	EnvironmentID string  `json:"environment_id"`
	Connector     string  `json:"connector"`
	Category      string  `json:"category"`
	Summary       string  `json:"summary"`
	SessionID     *string `json:"session_id"`
	TaskID        *string `json:"task_id"`
	MaxScope      string  `json:"max_scope"`
	// Status is pending, approved, denied or expired.
	Status string  `json:"status"`
	Scope  *string `json:"scope"`
	// DecidedVia is signature or api_key.
	DecidedVia *string    `json:"decided_via"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	DecidedAt  *time.Time `json:"decided_at"`
}

// ApprovalList is a page of approval requests.
type ApprovalList struct {
	Data []Approval `json:"data"`
	Next *string    `json:"next"`
}

// ListApprovalsParams are the parameters for listing approval requests.
type ListApprovalsParams struct {
	EnvironmentID string
	// Status is pending, approved, denied or expired.
	Status string
	Limit  int
	After  string
}

// GrantScope is the scope given when approving. Build it with OnceScope, TaskScope, SessionScope, UntilScope or
// PermanentScope.
type GrantScope struct {
	// Kind is once, task, session, until or permanent. The time-limited scope is called until in the decision
	// request and time_limited in stored grants.
	Kind      string `json:"kind"`
	TaskID    string `json:"task_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Until     int64  `json:"until,omitempty"`
}

// ApprovalDecision is the state after a decision.
type ApprovalDecision struct {
	// Status is approved or denied.
	Status string `json:"status"`
	// Scope is set only when approved.
	Scope *string `json:"scope,omitempty"`
}

// Grant is an active grant.
type Grant struct {
	ID            string     `json:"id"`
	EnvironmentID string     `json:"environment_id"`
	Connector     string     `json:"connector"`
	Category      string     `json:"category"`
	Scope         string     `json:"scope"`
	ScopeID       *string    `json:"scope_id"`
	Until         *time.Time `json:"until"`
	ApprovalID    *string    `json:"approval_id"`
	CreatedAt     time.Time  `json:"created_at"`
}

// ---- Audit ----

// AuditRecord is an audit record.
type AuditRecord struct {
	Chain  string          `json:"chain"`
	Seq    int64           `json:"seq"`
	At     time.Time       `json:"at"`
	Source string          `json:"source"`
	Record json.RawMessage `json:"record"`
}

// AuditRecordList is a page of audit records; Next is an opaque pagination cursor.
type AuditRecordList struct {
	Data []AuditRecord `json:"data"`
	Next *string       `json:"next"`
}

// ListAuditParams are the parameters for listing audit records.
type ListAuditParams struct {
	Source string
	Chain  string
	From   time.Time
	To     time.Time
	// Limit defaults to 100; the server clamps it to 1–1000.
	Limit int
	After string
}

// AuditChain is the state of one hash chain.
type AuditChain struct {
	Chain    string `json:"chain"`
	FirstSeq int64  `json:"first_seq"`
	LastSeq  int64  `json:"last_seq"`
	Head     string `json:"head"`
	// BrokenAtSeq is the sequence number from which the chain no longer links up; nil if the chain is intact.
	BrokenAtSeq *int64    `json:"broken_at_seq"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AuditChains is the state of all chains.
type AuditChains struct {
	Data   []AuditChain `json:"data"`
	Intact bool         `json:"intact"`
}

// ---- Usage and limits ----

// UsageTotals is a set of usage amounts.
type UsageTotals struct {
	ActiveHours     float64 `json:"active_hours"`
	CPUHours        float64 `json:"cpu_hours"`
	MemoryGiBHours  float64 `json:"memory_gib_hours"`
	StorageGiBHours float64 `json:"storage_gib_hours"`
	EgressGiB       float64 `json:"egress_gib"`
	BrowserHours    float64 `json:"browser_hours"`
}

// EnvironmentUsage is one environment's usage.
type EnvironmentUsage struct {
	EnvironmentID string `json:"environment_id"`
	EndUserID     string `json:"end_user_id"`
	UsageTotals
}

// Usage is the usage over a time range.
type Usage struct {
	From  time.Time          `json:"from"`
	To    time.Time          `json:"to"`
	Total UsageTotals        `json:"total"`
	Data  []EnvironmentUsage `json:"data"`
}

// UsageParams are the parameters for querying usage. Zero times mean from the start of the current month (UTC) to
// now.
type UsageParams struct {
	From          time.Time
	To            time.Time
	EnvironmentID string
}

// Usage metrics.
const (
	MetricActiveHours     = "active_hours"
	MetricCPUHours        = "cpu_hours"
	MetricMemoryGiBHours  = "memory_gib_hours"
	MetricStorageGiBHours = "storage_gib_hours"
	MetricEgressGiB       = "egress_gib"
	MetricBrowserHours    = "browser_hours"
)

// UsageLimit is a monthly limit.
type UsageLimit struct {
	// EnvironmentID is nil for a developer-level limit (the total across all environments).
	EnvironmentID *string `json:"environment_id"`
	Metric        string  `json:"metric"`
	MonthlyLimit  float64 `json:"monthly_limit"`
	// Used is the amount used so far this month (UTC). It is not set in SetLimit's response.
	Used      float64    `json:"used"`
	UpdatedAt *time.Time `json:"updated_at"`
}

// SetLimitParams are the parameters for setting a monthly limit. A limit counts as exceeded once usage reaches it,
// so 0 allows no usage at all.
type SetLimitParams struct {
	// EnvironmentID is empty for a developer-level limit.
	EnvironmentID string  `json:"environment_id,omitempty"`
	Metric        string  `json:"metric"`
	MonthlyLimit  float64 `json:"monthly_limit"`
}

// BatchCreateResult is one item of a batch create: Environment is set on success, Error and Message on failure.
type BatchCreateResult struct {
	Environment *Environment `json:"environment,omitempty"`
	Error       string       `json:"error,omitempty"`
	Message     string       `json:"message,omitempty"`
	// EnvironmentID is the id of the existing environment for environment_exists.
	EnvironmentID string `json:"environment_id,omitempty"`
}

// OK reports whether this item was created.
func (r BatchCreateResult) OK() bool { return r.Environment != nil }

// Wakeup is a scheduled one-time wake-up.
type Wakeup struct {
	ID        string    `json:"id"`
	At        time.Time `json:"at"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

// Schedule is a recurring schedule (registered by the agent in the VM, or by the developer).
type Schedule struct {
	ID string `json:"id"`
	// Source is agent or developer.
	Source      string     `json:"source"`
	Key         string     `json:"key"`
	Cron        *string    `json:"cron"`
	TZ          *string    `json:"tz"`
	At          *time.Time `json:"at"`
	NextAt      *time.Time `json:"next_at"`
	LastFiredAt *time.Time `json:"last_fired_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// AgentSettings are the agent's quotas for registering its own schedules and keeping the environment awake.
type AgentSettings struct {
	Scheduling         bool `json:"scheduling"`
	MaxSchedules       int  `json:"max_schedules"`
	MinIntervalSecs    int  `json:"min_interval_secs"`
	KeepAwakeMaxSecs   int  `json:"keep_awake_max_secs"`
	KeepAwakeDailySecs int  `json:"keep_awake_daily_secs"`
}

// AgentSettingsPatch holds quota changes; only the fields that are set (non-nil pointers) are changed.
type AgentSettingsPatch struct {
	Scheduling         *bool `json:"scheduling,omitempty"`
	MaxSchedules       *int  `json:"max_schedules,omitempty"`
	MinIntervalSecs    *int  `json:"min_interval_secs,omitempty"`
	KeepAwakeMaxSecs   *int  `json:"keep_awake_max_secs,omitempty"`
	KeepAwakeDailySecs *int  `json:"keep_awake_daily_secs,omitempty"`
}

// ---- Agent packages and rollouts ----

// Package is an agent package version.
type Package struct {
	Version   string    `json:"version"`
	SHA256    string    `json:"sha256"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

// PackageList is all packages and the default version.
type PackageList struct {
	Data           []Package `json:"data"`
	DefaultVersion *string   `json:"default_version"`
}

// Rollout is a gradual rollout of an agent package version.
type Rollout struct {
	ID          string `json:"id"`
	Version     string `json:"version"`
	Percent     int    `json:"percent"`
	BatchSize   int    `json:"batch_size"`
	MaxFailures int    `json:"max_failures"`
	// State is active, paused, completed or rolled_back.
	State      string     `json:"state"`
	Reason     *string    `json:"reason"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

// RolloutMember is an environment that a rollout has upgraded.
type RolloutMember struct {
	EnvironmentID string     `json:"environment_id"`
	FromVersion   *string    `json:"from_version"`
	UpgradedAt    time.Time  `json:"upgraded_at"`
	Deferred      bool       `json:"deferred"`
	HealthyAt     *time.Time `json:"healthy_at"`
	FailedAt      *time.Time `json:"failed_at"`
	Error         *string    `json:"error"`
	RolledBackAt  *time.Time `json:"rolled_back_at"`
}

// RolloutDetail is a rollout and the progress of each environment it has upgraded.
type RolloutDetail struct {
	Rollout
	Environments []RolloutMember `json:"environments"`
}

// StartRolloutParams are the parameters for starting a rollout.
type StartRolloutParams struct {
	Version string `json:"version"`
	// Percent is 1–100.
	Percent int `json:"percent"`
	// BatchSize is how many environments are upgraded at a time; the default is 1.
	BatchSize int `json:"batch_size,omitempty"`
	// MaxFailures is the number of failures above which the rollout rolls back automatically; the default is 0.
	MaxFailures int `json:"max_failures,omitempty"`
}

// ---- Browser pool ----

// BrowserProfile is a profile holding saved login state.
type BrowserProfile struct {
	ID        string     `json:"id"`
	EndUserID string     `json:"end_user_id"`
	Name      string     `json:"name"`
	Version   int64      `json:"version"`
	SavedAt   *time.Time `json:"saved_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// BrowserProfileList is a page of profiles.
type BrowserProfileList struct {
	Data []BrowserProfile `json:"data"`
	Next *string          `json:"next"`
}

// BrowserLease is a browser lease.
type BrowserLease struct {
	ID            string  `json:"id"`
	EnvironmentID string  `json:"environment_id"`
	EndUserID     string  `json:"end_user_id"`
	ProfileID     *string `json:"profile_id"`
	// Mode is write, read or clean.
	Mode           string `json:"mode"`
	ProfileVersion *int64 `json:"profile_version"`
	// State is pending, active, releasing, released, expired or failed.
	State string `json:"state"`
	// SaveOutcome is saved or discarded once a write lease has settled; nil otherwise.
	SaveOutcome *string    `json:"save_outcome"`
	CreatedAt   time.Time  `json:"created_at"`
	RenewedAt   time.Time  `json:"renewed_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	EndedAt     *time.Time `json:"ended_at"`
	// TakeoverStartedAt is when the end user's takeover started; nil if no takeover is in progress.
	TakeoverStartedAt *time.Time `json:"takeover_started_at"`
}

// BrowserLeaseList is a page of leases.
type BrowserLeaseList struct {
	Data []BrowserLease `json:"data"`
	Next *string        `json:"next"`
}

// AcquireLeaseParams are the parameters for acquiring a lease.
type AcquireLeaseParams struct {
	EnvironmentID string `json:"environment_id"`
	// ProfileID is empty for a clean browser.
	ProfileID string `json:"profile_id,omitempty"`
	// Mode defaults to write when a profile is given; without a profile it can only be clean.
	Mode string `json:"mode,omitempty"`
}

// ListLeasesParams are the parameters for listing leases.
type ListLeasesParams struct {
	EnvironmentID string
	State         string
	Limit         int
	After         string
}

// BrowserTakeover is the result of a takeover.
type BrowserTakeover struct {
	Lease BrowserLease `json:"lease"`
	// ViewURL is the view page for the end user to open; single use (valid for 10 minutes). Don't log it.
	ViewURL   string    `json:"view_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// BrowserLimits are the lease concurrency limits and the lease lifetime.
type BrowserLimits struct {
	MaxLeasesPerDeveloper int `json:"max_leases_per_developer"`
	MaxLeasesPerEndUser   int `json:"max_leases_per_end_user"`
	LeaseTTLSecs          int `json:"lease_ttl_secs"`
}

// ---- Team (members, API keys, webhook) ----

// Member is a person on the team.
type Member struct {
	ID string `json:"id"`
	// Email is lowercase.
	Email string  `json:"email"`
	Name  *string `json:"name"`
	// Role is owner or member.
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// Team is a team (developer) that an account can access.
type Team struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// Me is the current identity.
type Me struct {
	Developer struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"developer"`
	// Member is nil when using an API key.
	Member *Member `json:"member"`
	// Auth is session or api_key.
	Auth string `json:"auth"`
	// Developers is set only for console sessions.
	Developers []Team `json:"developers,omitempty"`
}

// APIKey is an API key record (without the key itself, except right after it is issued).
type APIKey struct {
	ID string `json:"id"`
	// Name is nil for keys issued by an operator command.
	Name      *string   `json:"name"`
	Prefix    string    `json:"prefix"`
	CreatedAt time.Time `json:"created_at"`
	// CreatedBy is the email of the member who created the key; nil for keys issued by an operator command.
	CreatedBy  *string    `json:"created_by"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
	// Key is set only in CreateAPIKey's response (temper_sk_...), and only this once.
	Key string `json:"key,omitempty"`
}

// WebhookSettings are the webhook settings (without the signing secret).
type WebhookSettings struct {
	URL             *string    `json:"url"`
	SecretRotatedAt *time.Time `json:"secret_rotated_at"`
	// PreviousSecretExpiresAt is when the old secret stops working after a rotation; nil if there is no old secret
	// that is still valid.
	PreviousSecretExpiresAt *time.Time `json:"previous_secret_expires_at"`
}

// ---- Developer secrets and egress allowlist ----

// PutSecretParams are the parameters for storing a secret.
type PutSecretParams struct {
	// Value is the real value (up to 8192 bytes, no newlines). No endpoint ever returns it afterwards.
	Value string `json:"value"`
	// Hosts are the exact hostnames (1–20) this value may be sent to.
	Hosts []string `json:"hosts"`
	// Paths is an allowlist of path prefixes (starting with /); empty means no restriction.
	Paths []string `json:"paths,omitempty"`
	// Header is the request header whose value is replaced with the real value; empty means authorization.
	Header string `json:"header,omitempty"`
	// Prefix is prepended to the real value in the header. nil means the default ("Bearer " for authorization,
	// empty for custom headers); use Ptr("") for no prefix at all.
	Prefix *string `json:"prefix,omitempty"`
	// Kind is empty for standard (subject to action policies); infrastructure is for the developer's own
	// infrastructure credentials, which skip action policies but are still audited.
	Kind string `json:"kind,omitempty"`
}

// Secret is a secret (without its value).
type Secret struct {
	Name string `json:"name"`
	// EnvironmentID is set only for environment-level secrets; nil for developer-level ones.
	EnvironmentID *string  `json:"environment_id"`
	Hosts         []string `json:"hosts"`
	Paths         []string `json:"paths"`
	Header        string   `json:"header"`
	Prefix        string   `json:"prefix"`
	// Kind is standard or infrastructure.
	Kind      string    `json:"kind"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// LastUsedAt is when the credential gateway last swapped in this value.
	LastUsedAt *time.Time `json:"last_used_at"`
}

// EnvironmentEgress is the egress allowlist an environment currently uses.
type EnvironmentEgress struct {
	Allow []string `json:"allow"`
	// Inherited is true if the environment has no allowlist of its own and uses the developer-level one.
	Inherited bool `json:"inherited"`
}
