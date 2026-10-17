package temper

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// DefaultBaseURL is the Temper API used when neither WithBaseURL nor TEMPER_BASE_URL is set.
const DefaultBaseURL = "https://api.temper.im"

// Version is the SDK version. It matches info.version in the Temper API spec.
const Version = "0.6.0"

const (
	defaultTimeout = 30 * time.Second
	// Exec and file calls first wake the environment, waiting up to 90 seconds, plus time to connect to the VM.
	wakeBudget = 105 * time.Second
)

// Client is a client for the Temper control plane. It is safe for concurrent use.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	headers http.Header
	timeout time.Duration

	Environments *EnvironmentsService
	Files        *FilesService
	Processes    *ProcessesService
	Connections  *ConnectionsService
	Policies     *PoliciesService
	Approvals    *ApprovalsService
	Audit        *AuditService
	Metering     *MeteringService
	Agent        *AgentService
	Browser      *BrowserService
	Team         *TeamService
	Secrets      *SecretsService
	// Templates manages environment templates: system packages and tools on top of the default runtime.
	Templates *TemplatesService
}

// Option configures a Client.
type Option func(*Client)

// WithAPIKey sets the API key (temper_sk_...). If it is not set, the TEMPER_API_KEY environment variable is used.
func WithAPIKey(key string) Option { return func(c *Client) { c.apiKey = key } }

// WithBaseURL sets the control plane address. If it is not set, TEMPER_BASE_URL is used, falling back to
// DefaultBaseURL.
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

// WithHTTPClient uses your own http.Client (for proxies, retries or tests). Its Timeout should preferably be 0,
// leaving timeouts to the context.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// WithHeader adds a header to every request.
func WithHeader(name, value string) Option { return func(c *Client) { c.headers.Set(name, value) } }

// WithTimeout sets the default timeout for regular calls, used when the caller's context has no deadline. The
// default is 30 seconds; 0 means no timeout.
func WithTimeout(d time.Duration) Option { return func(c *Client) { c.timeout = d } }

// New creates a client. It returns an error if no API key is available.
func New(opts ...Option) (*Client, error) {
	c := &Client{headers: http.Header{}, timeout: defaultTimeout}
	for _, o := range opts {
		o(c)
	}
	if c.apiKey == "" {
		c.apiKey = os.Getenv("TEMPER_API_KEY")
	}
	if c.apiKey == "" {
		return nil, &Error{Code: "missing_api_key", Message: "an API key is required (WithAPIKey or TEMPER_API_KEY)"}
	}
	if c.baseURL == "" {
		c.baseURL = os.Getenv("TEMPER_BASE_URL")
	}
	if c.baseURL == "" {
		c.baseURL = DefaultBaseURL
	}
	c.baseURL = strings.TrimRight(c.baseURL, "/")
	if c.http == nil {
		c.http = &http.Client{}
	}
	c.Environments = &EnvironmentsService{c}
	c.Files = &FilesService{c}
	c.Processes = &ProcessesService{c}
	c.Connections = &ConnectionsService{c}
	c.Policies = &PoliciesService{c}
	c.Approvals = &ApprovalsService{c}
	c.Audit = &AuditService{c}
	c.Metering = &MeteringService{c}
	c.Agent = &AgentService{c}
	c.Browser = &BrowserService{c}
	c.Team = &TeamService{c}
	c.Secrets = &SecretsService{c}
	c.Templates = &TemplatesService{c}
	return c, nil
}

// BaseURL returns the control plane address.
func (c *Client) BaseURL() string { return c.baseURL }

// request describes one call.
type request struct {
	method  string
	path    string
	query   url.Values
	json    any       // encoded as the JSON request body if set
	body    io.Reader // raw request body (mutually exclusive with json)
	ctype   string
	accept  string
	timeout time.Duration     // used when the caller's context has no deadline; 0 = client default; -1 = none
	headers map[string]string // extra headers for this request (e.g. Idempotency-Key)
}

// send sends the request. On 2xx it returns the response (the caller must close Body); otherwise it returns *Error.
func (c *Client) send(ctx context.Context, r request) (*http.Response, context.CancelFunc, error) {
	cancel := context.CancelFunc(func() {})
	if _, ok := ctx.Deadline(); !ok {
		d := r.timeout
		if d == 0 {
			d = c.timeout
		}
		if d > 0 {
			ctx, cancel = context.WithTimeout(ctx, d)
		}
	}
	u := c.baseURL + r.path
	if len(r.query) > 0 {
		u += "?" + r.query.Encode()
	}
	body := r.body
	ctype := r.ctype
	if r.json != nil {
		b, err := json.Marshal(r.json)
		if err != nil {
			cancel()
			return nil, nil, err
		}
		body = bytes.NewReader(b)
		ctype = "application/json"
	}
	req, err := http.NewRequestWithContext(ctx, r.method, u, body)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	for k, v := range c.headers {
		req.Header[k] = v
	}
	for k, v := range r.headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", "temper-go/"+Version)
	if r.accept != "" {
		req.Header.Set("Accept", r.accept)
	} else {
		req.Header.Set("Accept", "application/json")
	}
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	res, err := c.http.Do(req)
	if err != nil {
		cancel()
		code := CodeNetwork
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			code = CodeTimeout
		case errors.Is(err, context.Canceled):
			code = CodeAborted
		}
		return nil, nil, &Error{Code: code, Message: r.method + " " + r.path + ": " + err.Error(), Err: err}
	}
	if res.StatusCode >= 300 {
		defer cancel()
		defer res.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
		return nil, nil, errorFromResponse(res.StatusCode, res.Header.Get("Content-Type"), b)
	}
	return res, cancel, nil
}

// do sends the request and decodes the JSON response into out (the body is discarded when out is nil).
func (c *Client) do(ctx context.Context, r request, out any) error {
	res, cancel, err := c.send(ctx, r)
	if err != nil {
		return err
	}
	defer cancel()
	defer res.Body.Close()
	if out == nil || res.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, res.Body)
		return nil
	}
	return decode(res, out)
}

func decode(res *http.Response, out any) error {
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return &Error{Status: res.StatusCode, Code: "invalid_response", Message: err.Error(), Err: err}
	}
	return nil
}

// cancelOnClose cancels the context when Body is closed (for streaming responses).
type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b cancelOnClose) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

// Ptr returns a pointer to v, for filling in optional fields: temper.Ptr(0).
func Ptr[T any](v T) *T { return &v }

// seg URL-encodes one path segment (an id) so it can't turn into a different path.
func seg(s string) string { return url.PathEscape(s) }

func envPath(id, rest string) string { return "/v1/environments/" + seg(id) + rest }

// q is a query-string helper that only sets non-zero values.
type q url.Values

func newQuery() q { return q(url.Values{}) }

func (v q) str(k, s string) q {
	if s != "" {
		url.Values(v).Set(k, s)
	}
	return v
}

func (v q) int(k string, n int) q {
	if n != 0 {
		url.Values(v).Set(k, itoa(n))
	}
	return v
}

func (v q) int64(k string, n int64) q {
	if n != 0 {
		url.Values(v).Set(k, itoa64(n))
	}
	return v
}

func (v q) boolean(k string, b bool) q {
	if b {
		url.Values(v).Set(k, "true")
	}
	return v
}

func (v q) time(k string, t time.Time) q {
	if !t.IsZero() {
		url.Values(v).Set(k, t.UTC().Format(time.RFC3339Nano))
	}
	return v
}

func (v q) values() url.Values { return url.Values(v) }
