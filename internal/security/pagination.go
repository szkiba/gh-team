package security

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// paginate fetches GitHub pages following the Link header's rel="next" cursor.
//
// The Dependabot and code-scanning alert endpoints reject the legacy
// `?page=N` query parameter with HTTP 400 — they use opaque cursor
// pagination, exposed only through the Link response header. The previous
// "increment until short batch" loop worked for the team/repos endpoints
// but cannot work here.
//
// `decode` is called once per page with the raw JSON body and should
// project entries into the caller's accumulator. The loop terminates when
// the response has no `rel="next"` link, regardless of batch size — short
// last-page detection is unsafe on a cursor API.
//
// ctx is forwarded into the HTTP transport via RequestWithContext so a
// canceled parent context aborts the current in-flight request and the
// next-page lookup.
func paginate(ctx context.Context, c Client, initialPath string, decode func(body []byte) error) error {
	path := initialPath
	for {
		resp, err := c.RequestWithContext(ctx, "GET", path, nil)
		if err != nil {
			return err
		}
		body, readErr := io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read response body: %w", readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close response body: %w", closeErr)
		}
		if err := decode(body); err != nil {
			return err
		}
		next := nextLink(resp.Header.Get("Link"))
		if next == "" {
			return nil
		}
		path = next
	}
}

// nextLink parses an RFC 8288 Link header and returns the URL whose rel
// parameter equals "next". Returns "" when no such link is present.
//
// GitHub's format is:
//
//	<https://api.github.com/...&after=cur>; rel="next", <...>; rel="last"
//
// We tolerate extra whitespace and additional rel-parameter values such as
// rel="prev" / rel="first" because GitHub occasionally includes them.
func nextLink(header string) string {
	if header == "" {
		return ""
	}
	for _, part := range strings.Split(header, ",") {
		segment := strings.TrimSpace(part)
		if !strings.Contains(segment, `rel="next"`) {
			continue
		}
		start := strings.Index(segment, "<")
		end := strings.Index(segment, ">")
		if start < 0 || end < 0 || end <= start+1 {
			continue
		}
		return segment[start+1 : end]
	}
	return ""
}

// decodeJSONArray is a small helper around json.Unmarshal that handles the
// "empty body" case explicitly. Some non-2xx paths would already be caught
// by Client.Request, but a 200 with no body still has to decode cleanly.
func decodeJSONArray(body []byte, into interface{}) error {
	if len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, into)
}
