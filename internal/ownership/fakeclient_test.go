package ownership

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
)

// handler returns the JSON response body and an optional *api.HTTPError.
// Returning a non-nil error short-circuits the response.
type handler func(query string) (string, *api.HTTPError)

// fakeClient is a test-only restClient stub. Routes are keyed by the
// non-query portion of the path; pagination or query-sensitive responses
// are handled inside the handler.
type fakeClient struct {
	handlers map[string]handler
}

func newFakeClient() *fakeClient {
	return &fakeClient{handlers: map[string]handler{}}
}

func (f *fakeClient) on(path string, h handler) {
	f.handlers[path] = h
}

func (f *fakeClient) Get(path string, response interface{}) error {
	base, query := splitURL(path)
	h, ok := f.handlers[base]
	if !ok {
		return &api.HTTPError{StatusCode: 404, Message: fmt.Sprintf("no fake handler for %q", base)}
	}
	body, herr := h(query)
	if herr != nil {
		return herr
	}
	if body == "" || response == nil {
		return nil
	}
	return json.Unmarshal([]byte(body), response)
}

func splitURL(p string) (base, query string) {
	if i := strings.Index(p, "?"); i >= 0 {
		return p[:i], p[i+1:]
	}
	return p, ""
}

// staticJSON returns the same body for every page/query — adequate for
// strategies that stop pagination on the first short batch (any response
// with fewer than pageSize items breaks the loop).
func staticJSON(body string) handler {
	return func(string) (string, *api.HTTPError) { return body, nil }
}

func notFound() handler {
	return func(string) (string, *api.HTTPError) {
		return "", &api.HTTPError{StatusCode: 404, Message: "not found"}
	}
}

// base64File builds the JSON payload GitHub's contents API returns, with
// the file content base64-encoded as the real API does.
func base64File(content string) string {
	return fmt.Sprintf(`{"content":%q,"encoding":"base64"}`, base64.StdEncoding.EncodeToString([]byte(content)))
}
