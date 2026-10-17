package temper

import "context"

// SecretsService manages the developer's custom secrets and egress allowlists.
//
// Secret values are stored encrypted in the control plane; the VM only gets a stand-in token. The agent reads it
// from /run/temper/secrets/NAME and puts it in a request header as usual; when the request goes to one of the
// secret's hosts, the credential gateway on the host swaps the header for the real value. Once written, a value
// is never returned by any endpoint.
type SecretsService struct{ c *Client }

// List returns the developer-level secrets (available in every environment), sorted by name.
func (s *SecretsService) List(ctx context.Context) ([]Secret, error) {
	var out struct {
		Data []Secret `json:"data"`
	}
	return out.Data, s.c.do(ctx, request{method: "GET", path: "/v1/secrets"}, &out)
}

// Set stores (or replaces) a developer-level secret. The name must look like an environment variable
// (OPENROUTER_API_KEY); invalid parameters return bad_request.
func (s *SecretsService) Set(ctx context.Context, name string, p PutSecretParams) (*Secret, error) {
	var out Secret
	return &out, s.c.do(ctx, request{method: "PUT", path: "/v1/secrets/" + seg(name), json: p}, &out)
}

// Delete deletes a developer-level secret (effective immediately). It returns not_found if it doesn't exist.
func (s *SecretsService) Delete(ctx context.Context, name string) error {
	return s.c.do(ctx, request{method: "DELETE", path: "/v1/secrets/" + seg(name)}, nil)
}

// ListForEnvironment returns the environment-level secrets (only this environment's own, not developer-level
// ones).
func (s *SecretsService) ListForEnvironment(ctx context.Context, environmentID string) ([]Secret, error) {
	var out struct {
		Data []Secret `json:"data"`
	}
	return out.Data, s.c.do(ctx, request{method: "GET", path: envPath(environmentID, "/secrets")}, &out)
}

// SetForEnvironment stores (or replaces) an environment-level secret; it overrides a developer-level secret with
// the same name.
func (s *SecretsService) SetForEnvironment(ctx context.Context, environmentID, name string, p PutSecretParams) (*Secret, error) {
	var out Secret
	return &out, s.c.do(ctx, request{method: "PUT", path: envPath(environmentID, "/secrets/"+seg(name)), json: p}, &out)
}

// DeleteForEnvironment deletes an environment-level secret; if there is a developer-level secret with the same
// name, the environment goes back to using it.
func (s *SecretsService) DeleteForEnvironment(ctx context.Context, environmentID, name string) error {
	return s.c.do(ctx, request{method: "DELETE", path: envPath(environmentID, "/secrets/"+seg(name))}, nil)
}

// GetEgressMode returns the developer-level egress mode (EgressAllowlist, EgressOpen or EgressAsk), used by every
// environment without its own.
func (s *SecretsService) GetEgressMode(ctx context.Context) (string, error) {
	var out struct {
		Mode string `json:"mode"`
	}
	return out.Mode, s.c.do(ctx, request{method: "GET", path: "/v1/egress"}, &out)
}

// SetEgressMode sets what happens to hosts outside the allowlist for every environment without its own mode.
func (s *SecretsService) SetEgressMode(ctx context.Context, mode string) (string, error) {
	var out struct {
		Mode string `json:"mode"`
	}
	return out.Mode, s.c.do(ctx, request{method: "PUT", path: "/v1/egress", json: map[string]any{"mode": mode}}, &out)
}

// SetEgressHook sets (or with nil removes) the endpoint that EgressHook mode asks before a host outside the
// allowlist is allowed.
func (s *SecretsService) SetEgressHook(ctx context.Context, hook *EgressHookSettings) (*EgressHookSettings, error) {
	var out struct {
		Hook *EgressHookSettings `json:"hook"`
	}
	return out.Hook, s.c.do(ctx, request{method: "PUT", path: "/v1/egress", json: map[string]any{"hook": hook}}, &out)
}

// SetEnvironmentEgressMode sets the environment's own egress mode; nil reverts to the developer-level one.
func (s *SecretsService) SetEnvironmentEgressMode(ctx context.Context, environmentID string, mode *string) (*EnvironmentEgress, error) {
	var out EnvironmentEgress
	return &out, s.c.do(ctx, request{method: "PUT", path: envPath(environmentID, "/egress"), json: map[string]any{"mode": mode}}, &out)
}

// GetEgress returns the developer-level egress allowlist (used by every environment without its own allowlist).
func (s *SecretsService) GetEgress(ctx context.Context) ([]string, error) {
	var out struct {
		Allow []string `json:"allow"`
	}
	return out.Allow, s.c.do(ctx, request{method: "GET", path: "/v1/egress"}, &out)
}

// SetEgress replaces the whole developer-level allowlist (exact hostnames or *.example.com, at most 200). Changes
// take effect immediately.
func (s *SecretsService) SetEgress(ctx context.Context, allow []string) ([]string, error) {
	if allow == nil {
		allow = []string{}
	}
	var out struct {
		Allow []string `json:"allow"`
	}
	return out.Allow, s.c.do(ctx, request{method: "PUT", path: "/v1/egress", json: map[string][]string{"allow": allow}}, &out)
}

// GetEnvironmentEgress returns the allowlist the environment currently uses, and whether it is inherited from the
// developer level.
func (s *SecretsService) GetEnvironmentEgress(ctx context.Context, environmentID string) (*EnvironmentEgress, error) {
	var out EnvironmentEgress
	return &out, s.c.do(ctx, request{method: "GET", path: envPath(environmentID, "/egress")}, &out)
}

// SetEnvironmentEgress sets the environment's own allowlist (the developer-level one is no longer consulted
// afterwards); a nil allow goes back to the developer-level allowlist. Changes take effect immediately.
func (s *SecretsService) SetEnvironmentEgress(ctx context.Context, environmentID string, allow []string) (*EnvironmentEgress, error) {
	var out EnvironmentEgress
	body := map[string][]string{"allow": allow} // nil encodes as null
	return &out, s.c.do(ctx, request{method: "PUT", path: envPath(environmentID, "/egress"), json: body}, &out)
}

// AddEnvironmentEgressHost adds one host to the environment's allowlist without touching the others (concurrent
// adds don't overwrite each other). If the environment still uses the developer-level allowlist, that list is
// copied first.
func (s *SecretsService) AddEnvironmentEgressHost(ctx context.Context, environmentID, host string) (*EnvironmentEgress, error) {
	var out EnvironmentEgress
	return &out, s.c.do(ctx, request{method: "PUT", path: envPath(environmentID, "/egress/"+seg(host))}, &out)
}

// RemoveEnvironmentEgressHost removes a host from the environment's allowlist (it succeeds even if the host isn't
// there).
func (s *SecretsService) RemoveEnvironmentEgressHost(ctx context.Context, environmentID, host string) (*EnvironmentEgress, error) {
	var out EnvironmentEgress
	return &out, s.c.do(ctx, request{method: "DELETE", path: envPath(environmentID, "/egress/"+seg(host))}, &out)
}
