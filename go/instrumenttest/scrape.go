// SPDX-License-Identifier: Apache-2.0

package instrumenttest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// AssertScrapes fails unless h serves a scrape with status 200. Pass the
// handler the binary serves, built the same way, from a test that links the
// same packages, so that every registry it combines holds what it holds in
// production. A series exported by two registries fails the scrape, and this
// is where that shows up.
func AssertScrapes(t testing.TB, h http.Handler) {
	t.Helper()
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, err := srv.Client().Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusOK {
		return
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	t.Errorf("scrape returned %d; a series exported by two registries fails the whole scrape, so rename one of them:\n%s", resp.StatusCode, body)
}
