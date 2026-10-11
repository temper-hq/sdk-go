package temper

import "context"

// TeamService manages the current identity, team members, API keys and webhook settings.
//
// With an API key you can call Me, list members, list keys, read the webhook settings, and revoke the key itself.
// Adding, changing or removing members, issuing keys and changing the webhook require a console session (calling
// them with an API key returns session_required); issuing keys, changing the webhook and removing members also
// require a login within the last 10 minutes (reauth_required).
type TeamService struct{ c *Client }

// Me returns the current developer (team), member (nil with an API key) and authentication method.
func (s *TeamService) Me(ctx context.Context) (*Me, error) {
	var out Me
	return &out, s.c.do(ctx, request{method: "GET", path: "/v1/me"}, &out)
}

// ListMembers returns the team members in the order they joined.
func (s *TeamService) ListMembers(ctx context.Context) ([]Member, error) {
	var out struct {
		Data []Member `json:"data"`
	}
	return out.Data, s.c.do(ctx, request{method: "GET", path: "/v1/members"}, &out)
}

// AddMember adds a member (owner session). It returns member_exists if the person is already a member.
func (s *TeamService) AddMember(ctx context.Context, email, role string) (*Member, error) {
	var out Member
	body := map[string]string{"email": email, "role": role}
	return &out, s.c.do(ctx, request{method: "POST", path: "/v1/members", json: body}, &out)
}

// SetMemberRole changes a member's role (owner session). The last owner can't be demoted (last_owner).
func (s *TeamService) SetMemberRole(ctx context.Context, id, role string) (*Member, error) {
	var out Member
	return &out, s.c.do(ctx, request{method: "PATCH", path: "/v1/members/" + seg(id), json: map[string]string{"role": role}}, &out)
}

// RemoveMember removes a member (owner session with a recent login); their console sessions end immediately.
func (s *TeamService) RemoveMember(ctx context.Context, id string) error {
	return s.c.do(ctx, request{method: "DELETE", path: "/v1/members/" + seg(id)}, nil)
}

// ListAPIKeys lists API keys (including revoked ones), without the keys themselves.
func (s *TeamService) ListAPIKeys(ctx context.Context) ([]APIKey, error) {
	var out struct {
		Data []APIKey `json:"data"`
	}
	return out.Data, s.c.do(ctx, request{method: "GET", path: "/v1/api-keys"}, &out)
}

// CreateAPIKey issues an API key (owner session with a recent login). The returned Key is shown only this once.
func (s *TeamService) CreateAPIKey(ctx context.Context, name string) (*APIKey, error) {
	var out APIKey
	return &out, s.c.do(ctx, request{method: "POST", path: "/v1/api-keys", json: map[string]string{"name": name}}, &out)
}

// RevokeAPIKey revokes an API key, effective immediately; idempotent. A key can revoke itself; revoking another
// key requires an owner session with a recent login.
func (s *TeamService) RevokeAPIKey(ctx context.Context, id string) (*APIKey, error) {
	var out APIKey
	return &out, s.c.do(ctx, request{method: "DELETE", path: "/v1/api-keys/" + seg(id)}, &out)
}

// GetWebhook returns the webhook URL (without the signing secret).
func (s *TeamService) GetWebhook(ctx context.Context) (*WebhookSettings, error) {
	var out WebhookSettings
	return &out, s.c.do(ctx, request{method: "GET", path: "/v1/webhook"}, &out)
}

// SetWebhook sets the webhook URL (owner session with a recent login). It returns a new whsec_ secret the first
// time the URL is set, when it changes, or with rotateSecret (shown only this once), and an empty string
// otherwise. The old secret stays valid for another 24 hours (during which each webhook carries two signatures).
func (s *TeamService) SetWebhook(ctx context.Context, url string, rotateSecret bool) (string, error) {
	var out struct {
		Secret *string `json:"secret"`
	}
	body := map[string]any{"url": url}
	if rotateSecret {
		body["rotate_secret"] = true
	}
	if err := s.c.do(ctx, request{method: "PUT", path: "/v1/webhook", json: body}, &out); err != nil {
		return "", err
	}
	return deref(out.Secret), nil
}
