package security

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/cli/go-gh/v2/pkg/api"
)

// handler returns the response body, optional Link header, and an optional
// *api.HTTPError. Path queries are passed through so paginated handlers can
// branch on cursor parameters when needed.
type handler func(query string) (body string, linkHeader string, herr *api.HTTPError)

// fakeClient is a test-only Client stub that mimics *api.RESTClient.Request.
// Routes are keyed by the non-query portion of the path; per-page responses
// are returned by the handler based on the query string.
type fakeClient struct {
	mu       sync.Mutex
	handlers map[string]handler
	calls    map[string]int
}

func newFakeClient() *fakeClient {
	return &fakeClient{
		handlers: map[string]handler{},
		calls:    map[string]int{},
	}
}

func (f *fakeClient) on(path string, h handler) {
	f.handlers[path] = h
}

// RequestWithContext implements security.Client. It accepts either relative
// GitHub API paths (e.g. "repos/foo/bar/dependabot/alerts?state=open") or
// absolute URLs that the paginator extracted from a Link header — those
// are normalized to the same key the test registered. Cancellation is
// honored at call entry so the cancel-propagation tests can verify the
// collector actually stops issuing requests when ctx is canceled.
func (f *fakeClient) RequestWithContext(ctx context.Context, _ string, path string, _ io.Reader) (*http.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	base, query := splitURL(stripHost(path))
	f.mu.Lock()
	f.calls[base]++
	h, ok := f.handlers[base]
	f.mu.Unlock()
	if !ok {
		return nil, &api.HTTPError{StatusCode: 404, Message: fmt.Sprintf("no fake handler for %q", base)}
	}
	body, link, herr := h(query)
	if herr != nil {
		return nil, herr
	}
	header := http.Header{}
	if link != "" {
		header.Set("Link", link)
	}
	return &http.Response{
		StatusCode: 200,
		Header:     header,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}, nil
}

// stripHost mirrors how go-gh's restURL works in reverse: real callers
// always pass a relative path, but the paginator may forward an absolute
// "next" URL extracted from the Link header (GitHub returns absolute
// URLs). Stripping the scheme+host lets tests register handlers against
// the canonical relative form regardless of which call site produced the
// request.
func stripHost(p string) string {
	if i := strings.Index(p, "://"); i >= 0 {
		rest := p[i+3:]
		if j := strings.Index(rest, "/"); j >= 0 {
			return strings.TrimPrefix(rest[j:], "/")
		}
	}
	return p
}

func splitURL(p string) (base, query string) {
	if i := strings.Index(p, "?"); i >= 0 {
		return p[:i], p[i+1:]
	}
	return p, ""
}

// staticJSON returns the same body with no Link header — a single-page
// response. The paginator stops after one call because there is no next.
func staticJSON(body string) handler {
	return func(string) (string, string, *api.HTTPError) { return body, "", nil }
}

// pagedJSON returns `body` and constructs a Link header pointing at
// `nextRelPath` (a relative API path the fake's stripHost normalization
// turns back into a registered route). Pass nextRelPath="" for the final
// page.
func pagedJSON(body, nextRelPath string) handler {
	return func(string) (string, string, *api.HTTPError) {
		if nextRelPath == "" {
			return body, "", nil
		}
		link := fmt.Sprintf(`<https://api.github.com/%s>; rel="next"`, nextRelPath)
		return body, link, nil
	}
}

// httpErr builds an api.HTTPError with a known message and (optionally) the
// accepted-scopes header so the security_events fatal path is exercisable.
func httpErr(code int, msg string, headers http.Header) handler {
	return func(string) (string, string, *api.HTTPError) {
		h := headers
		if h == nil {
			h = http.Header{}
		}
		return "", "", &api.HTTPError{StatusCode: code, Message: msg, Headers: h}
	}
}
