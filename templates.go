package temper

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

// Error codes for templates.
const (
	CodeVersionExists       = "version_exists"
	CodeTemplateNotReady    = "template_not_ready"
	CodeTemplateInUse       = "template_in_use"
	CodeTemplateBuilding    = "template_building"
	CodeTemplateBuildFailed = "template_build_failed"
)

// TemplatesService manages environment templates: system packages and tools layered on top of the platform's default
// runtime, built from steps. To build from a Dockerfile, translate it into steps with the TypeScript SDK's CLI
// (npx @temper-hq/sdk template translate -f Dockerfile) and pass the result to CreateVersion.
type TemplatesService struct{ c *Client }

// TemplateStep is one build step. Set exactly one field, or use RunStep, EnvStep, WorkdirStep, CopyStep and CopyBytes.
type TemplateStep struct {
	// Run is a shell command, run as root with /bin/sh -c in the current workdir, with the variables from earlier env steps.
	Run string `json:"run,omitempty"`
	// Env holds variables for later steps and for every environment using the template (lowest priority).
	Env map[string]string `json:"env,omitempty"`
	// Workdir is an absolute directory for later run steps and for the start command; created if missing.
	Workdir string `json:"workdir,omitempty"`
	// Copy writes one file into the template.
	Copy *TemplateCopy `json:"copy,omitempty"`
}

// TemplateCopy is a file written by a copy step. All copy contents together are limited to 4 MiB (decoded).
type TemplateCopy struct {
	// Path is an absolute file path; parent directories are created.
	Path    string `json:"path"`
	Content string `json:"content"`
	// Encoding is empty (utf8) or "base64".
	Encoding string `json:"encoding,omitempty"`
	// Mode is octal permissions such as "0755"; empty = "0644".
	Mode string `json:"mode,omitempty"`
}

// RunStep returns a step that runs command as root with /bin/sh -c.
func RunStep(command string) TemplateStep { return TemplateStep{Run: command} }

// EnvStep returns a step that sets environment variables.
func EnvStep(env map[string]string) TemplateStep { return TemplateStep{Env: env} }

// WorkdirStep returns a step that changes the working directory.
func WorkdirStep(dir string) TemplateStep { return TemplateStep{Workdir: dir} }

// CopyStep returns a step that writes text content to path.
func CopyStep(path, content string) TemplateStep {
	return TemplateStep{Copy: &TemplateCopy{Path: path, Content: content}}
}

// CopyBytes returns a step that writes binary content to path with the given octal mode (empty = "0644").
func CopyBytes(path string, content []byte, mode string) TemplateStep {
	return TemplateStep{Copy: &TemplateCopy{Path: path, Content: base64.StdEncoding.EncodeToString(content), Encoding: "base64", Mode: mode}}
}

// TemplateStart is what environments with the template run when they have no Agent package: a command string
// (run with /bin/sh -c) or an argv list, started in the last workdir. Build one with StartShell or StartArgv.
type TemplateStart struct {
	Shell string
	Argv  []string
}

// StartShell returns a start command run with /bin/sh -c.
func StartShell(command string) *TemplateStart { return &TemplateStart{Shell: command} }

// StartArgv returns a start command given as an argv list.
func StartArgv(argv ...string) *TemplateStart { return &TemplateStart{Argv: argv} }

// MarshalJSON encodes the start command as a string or an array.
func (s TemplateStart) MarshalJSON() ([]byte, error) {
	if s.Argv != nil {
		return json.Marshal(s.Argv)
	}
	return json.Marshal(s.Shell)
}

// UnmarshalJSON accepts a string or an array.
func (s *TemplateStart) UnmarshalJSON(b []byte) error {
	if strings.HasPrefix(strings.TrimSpace(string(b)), "[") {
		return json.Unmarshal(b, &s.Argv)
	}
	return json.Unmarshal(b, &s.Shell)
}

// CreateTemplateVersionParams are the parameters of TemplatesService.CreateVersion.
type CreateTemplateVersionParams struct {
	// From is a base version as "name:version" (one of yours, and ready); empty = the platform's default runtime.
	From string `json:"from,omitempty"`
	// FromImage starts from a public OCI image instead of From ("python:3.12-slim", "ghcr.io/org/app:1.0"); the build pulls
	// it and uses it as the whole root filesystem.
	FromImage string `json:"from_image,omitempty"`
	// RegistrySecret names the developer-level secret with the login for a private FromImage (base64(user:password),
	// prefix "Basic "); the login never enters the build VM.
	RegistrySecret string `json:"registry_secret,omitempty"`
	// Steps are up to 64 build steps, run in order; at least one unless FromImage is set.
	Steps []TemplateStep `json:"steps,omitempty"`
	Start *TemplateStart `json:"start,omitempty"`
}

// TemplateVersion is one immutable version of a template.
type TemplateVersion struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	// From is the base version ("name:version"); nil = the platform's default runtime or an OCI image (FromImage).
	From *string `json:"from"`
	// FromImage is the base OCI image, normalised ("docker.io/library/python:3.12-slim"); empty unless the version starts from an image.
	FromImage string `json:"from_image,omitempty"`
	// ImageDigest is the image manifest the build pulled ("sha256:..."), once the import has run.
	ImageDigest string `json:"image_digest,omitempty"`
	// RegistrySecret is the secret used to pull a private FromImage.
	RegistrySecret string `json:"registry_secret,omitempty"`
	// State is building, ready or failed.
	State string `json:"state"`
	// Error says why the build failed.
	Error *string `json:"error"`
	// LayerCount counts the layers on top of the default runtime, including the base version's; 0 until the version is ready.
	LayerCount int            `json:"layer_count"`
	Start      *TemplateStart `json:"start"`
	// Log is the last 64 KiB of the build log.
	Log        string     `json:"log"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

// TemplateVersionSummary is a version as listed by TemplatesService.List.
type TemplateVersionSummary struct {
	Version   string    `json:"version"`
	State     string    `json:"state"`
	From      *string   `json:"from"`
	CreatedAt time.Time `json:"created_at"`
}

// Template is a template with its versions in creation order.
type Template struct {
	Name     string                   `json:"name"`
	Versions []TemplateVersionSummary `json:"versions"`
}

// CreateVersion starts building a new, immutable version and returns at once with State "building"; see WaitForVersion.
// An existing version returns CodeVersionExists; a base (From) that is not ready returns CodeTemplateNotReady.
func (s *TemplatesService) CreateVersion(ctx context.Context, name, version string, p CreateTemplateVersionParams) (*TemplateVersion, error) {
	var out TemplateVersion
	return &out, s.c.do(ctx, request{method: "POST", path: "/v1/templates/" + seg(name) + "/versions/" + seg(version), json: p}, &out)
}

// GetVersion returns the build state, the reason a build failed, the number of layers and the tail of the build log.
func (s *TemplatesService) GetVersion(ctx context.Context, name, version string) (*TemplateVersion, error) {
	var out TemplateVersion
	return &out, s.c.do(ctx, request{method: "GET", path: "/v1/templates/" + seg(name) + "/versions/" + seg(version)}, &out)
}

// List returns all templates, each with its versions.
func (s *TemplatesService) List(ctx context.Context) ([]Template, error) {
	var out struct {
		Data []Template `json:"data"`
	}
	return out.Data, s.c.do(ctx, request{method: "GET", path: "/v1/templates"}, &out)
}

// DeleteVersion deletes a version. One that is still building returns CodeTemplateBuilding; one used by an environment
// that has not been destroyed, or by another version as its base, returns CodeTemplateInUse.
func (s *TemplatesService) DeleteVersion(ctx context.Context, name, version string) error {
	return s.c.do(ctx, request{method: "DELETE", path: "/v1/templates/" + seg(name) + "/versions/" + seg(version)}, nil)
}

// WaitForVersionOptions are the options of TemplatesService.WaitForVersion.
type WaitForVersionOptions struct {
	// Timeout defaults to an hour, the longest a build can run.
	Timeout time.Duration
	// Interval between polls; default 3 seconds.
	Interval time.Duration
	// OnPoll, if set, is called with each polled version, for example to print new build log lines.
	OnPoll func(*TemplateVersion)
}

// WaitForVersion polls until the version is ready and returns it. A failed build returns CodeTemplateBuildFailed
// (the reason and the end of the build log are in the message); still building after the timeout returns CodeWaitTimeout.
// Both have Status 0. When ctx is canceled it returns ctx's error.
func (s *TemplatesService) WaitForVersion(ctx context.Context, name, version string, opts *WaitForVersionOptions) (*TemplateVersion, error) {
	o := WaitForVersionOptions{Timeout: time.Hour, Interval: 3 * time.Second}
	if opts != nil {
		if opts.Timeout > 0 {
			o.Timeout = opts.Timeout
		}
		if opts.Interval > 0 {
			o.Interval = opts.Interval
		}
		o.OnPoll = opts.OnPoll
	}
	deadline := time.Now().Add(o.Timeout)
	for {
		v, err := s.GetVersion(ctx, name, version)
		if err != nil {
			return nil, err
		}
		if o.OnPoll != nil {
			o.OnPoll(v)
		}
		switch {
		case v.State == "ready":
			return v, nil
		case v.State == "failed":
			reason := "unknown error"
			if v.Error != nil {
				reason = *v.Error
			}
			lines := strings.Split(strings.TrimRight(v.Log, "\n"), "\n")
			if len(lines) > 20 {
				lines = lines[len(lines)-20:]
			}
			return nil, &Error{Code: CodeTemplateBuildFailed, Message: name + ":" + version + " failed: " + reason + "\n" + strings.Join(lines, "\n")}
		case time.Now().Add(o.Interval).After(deadline):
			return nil, &Error{Code: CodeWaitTimeout, Message: name + ":" + version + " is still building after " + o.Timeout.String()}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(o.Interval):
		}
	}
}
