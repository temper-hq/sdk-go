package temper

import "context"

// ConnectionsService manages end users' per-user OAuth authorizations and the developer's own OAuth apps.
// Responses never include tokens.
type ConnectionsService struct{ c *Client }

// Authorize creates an authorization link for an end user (valid for 10 minutes, single use). It returns
// no_oauth_client if no OAuth app is registered and the platform test app isn't configured either.
func (s *ConnectionsService) Authorize(ctx context.Context, p AuthorizeConnectionParams) (*ConnectionAuthorization, error) {
	var out ConnectionAuthorization
	return &out, s.c.do(ctx, request{method: "POST", path: "/v1/connections/authorize", json: p}, &out)
}

// List lists connections (newest first, at most 500, not paginated). An empty endUserID lists all of them;
// revoked connections are left out unless includeRevoked is true.
func (s *ConnectionsService) List(ctx context.Context, endUserID string, includeRevoked bool) (*ConnectionList, error) {
	var out ConnectionList
	query := newQuery().str("end_user_id", endUserID).boolean("include_revoked", includeRevoked).values()
	return &out, s.c.do(ctx, request{method: "GET", path: "/v1/connections", query: query}, &out)
}

// Get returns a connection.
func (s *ConnectionsService) Get(ctx context.Context, id string) (*Connection, error) {
	var out Connection
	return &out, s.c.do(ctx, request{method: "GET", path: "/v1/connections/" + seg(id)}, &out)
}

// Revoke revokes a connection (also revoking it at the provider where possible) and returns the revoked
// connection. An already revoked connection is returned unchanged.
func (s *ConnectionsService) Revoke(ctx context.Context, id string) (*Connection, error) {
	var out Connection
	return &out, s.c.do(ctx, request{method: "DELETE", path: "/v1/connections/" + seg(id)}, &out)
}

// ListOAuthClients returns one entry per provider: the client_id you registered (the secret is never returned),
// whether the platform test app is available, and the redirect URI.
func (s *ConnectionsService) ListOAuthClients(ctx context.Context) ([]OAuthClient, error) {
	var out struct {
		Data []OAuthClient `json:"data"`
	}
	return out.Data, s.c.do(ctx, request{method: "GET", path: "/v1/oauth-clients"}, &out)
}

// SetOAuthClient registers (or replaces) the developer's own OAuth app for a provider (google or slack).
func (s *ConnectionsService) SetOAuthClient(ctx context.Context, provider, clientID, clientSecret string) (*OAuthClientRegistered, error) {
	var out OAuthClientRegistered
	body := map[string]string{"client_id": clientID, "client_secret": clientSecret}
	return &out, s.c.do(ctx, request{method: "PUT", path: "/v1/oauth-clients/" + seg(provider), json: body}, &out)
}
