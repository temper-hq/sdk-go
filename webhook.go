package temper

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Headers on the requests Temper sends to the developer's backend.
const (
	SignatureHeader = "Temper-Signature"
	EventIDHeader   = "Temper-Event-Id"
)

// DefaultWebhookTolerance is the allowed clock skew for the signature timestamp.
const DefaultWebhookTolerance = 5 * time.Minute

// WebhookEvent is an event sent to the developer's backend: environment events, connection.*,
// approval.requested, browser.takeover_requested and others.
type WebhookEvent struct {
	// ID stays the same across retries; use it for deduplication.
	ID      string    `json:"id"`
	Type    string    `json:"type"`
	Created time.Time `json:"created"`
	// EnvironmentID is empty for connection.created and connection.revoked.
	EnvironmentID string          `json:"environment_id"`
	Data          json.RawMessage `json:"data"`
}

// ApprovalRequested is the Data of an approval.requested event.
type ApprovalRequested struct {
	ApprovalID string `json:"approval_id"`
	EnvID      string `json:"env_id"`
	Connector  string `json:"connector"`
	Category   string `json:"category"`
	Summary    string `json:"summary"`
	MaxScope   string `json:"max_scope"`
	// ExpiresAt is in Unix seconds.
	ExpiresAt   int64  `json:"expires_at"`
	DecisionURL string `json:"decision_url"`
}

// VerifyWebhook verifies the Temper-Signature header (t=<unix seconds>,v1=<hex>; there may be several v1 values,
// since both the old and new secrets sign for 24 hours after a rotation) and parses the event.
// payload must be the raw request body bytes. A zero tolerance uses DefaultWebhookTolerance; a zero now uses the
// current time. On failure it returns an *Error with Code webhook_signature.
func VerifyWebhook(secret string, payload []byte, header string, tolerance time.Duration, now time.Time) (*WebhookEvent, error) {
	fail := func(msg string) error { return &Error{Code: CodeWebhookSignature, Message: msg} }
	if header == "" {
		return nil, fail("missing Temper-Signature header")
	}
	if tolerance == 0 {
		tolerance = DefaultWebhookTolerance
	}
	if now.IsZero() {
		now = time.Now()
	}
	var ts int64 = -1
	var sigs [][]byte
	for _, part := range strings.Split(header, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch k {
		case "t":
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				ts = n
			}
		case "v1":
			if b, err := hex.DecodeString(v); err == nil {
				sigs = append(sigs, b)
			}
		}
	}
	if ts < 0 || len(sigs) == 0 {
		return nil, fail("malformed Temper-Signature header")
	}
	if math.Abs(float64(now.Unix()-ts)) > tolerance.Seconds() {
		return nil, fail("timestamp outside the tolerance")
	}
	want := sign(secret, ts, payload)
	ok := false
	for _, s := range sigs {
		if hmac.Equal(s, want) {
			ok = true
		}
	}
	if !ok {
		return nil, fail("no matching signature")
	}
	var ev WebhookEvent
	if err := json.Unmarshal(payload, &ev); err != nil {
		return nil, fail("payload is not a JSON event: " + err.Error())
	}
	return &ev, nil
}

// SignWebhook builds a Temper-Signature header value (for testing your own webhook handler).
func SignWebhook(secret string, payload []byte, t time.Time) string {
	ts := t.Unix()
	return fmt.Sprintf("t=%d,v1=%s", ts, hex.EncodeToString(sign(secret, ts, payload)))
}

func sign(secret string, ts int64, payload []byte) []byte {
	m := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(m, "%d.", ts)
	m.Write(payload)
	return m.Sum(nil)
}
