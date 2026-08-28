package feed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchUsesConditionalRequestsAndParsesItems(t *testing.T) {
	const (
		etag         = `"news-v1"`
		lastModified = "Wed, 26 Aug 2026 20:00:00 GMT"
	)

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if got := r.Header.Get("User-Agent"); got == "" {
			t.Error("missing User-Agent")
		}

		if requests == 2 {
			if got := r.Header.Get("If-None-Match"); got != etag {
				t.Errorf("If-None-Match = %q, want %q", got, etag)
			}
			if got := r.Header.Get("If-Modified-Since"); got != lastModified {
				t.Errorf("If-Modified-Since = %q, want %q", got, lastModified)
			}
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("Content-Type", "application/rss+xml")
		w.Header().Set("ETag", etag)
		w.Header().Set("Last-Modified", lastModified)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><item><guid>wire-1</guid><title>Public wire item</title><description>summary</description><link>https://example.test/item</link><pubDate>Wed, 26 Aug 2026 20:00:00 GMT</pubDate></item></channel></rss>`))
	}))
	defer server.Close()

	client := NewClient()
	state := &State{}

	items, changed, err := client.Fetch(context.Background(), "test-wire", server.URL, state)
	if err != nil {
		t.Fatalf("first Fetch() error = %v", err)
	}
	if !changed {
		t.Fatal("first Fetch() changed = false")
	}
	if len(items) != 1 {
		t.Fatalf("first Fetch() returned %d items, want 1", len(items))
	}
	if items[0].ID != "wire-1" || items[0].Title != "Public wire item" || items[0].Source != "test-wire" {
		t.Fatalf("unexpected parsed item: %+v", items[0])
	}
	if state.ETag != etag || state.LastModified != lastModified {
		t.Fatalf("state = %+v, want ETag and Last-Modified", state)
	}

	items, changed, err = client.Fetch(context.Background(), "test-wire", server.URL, state)
	if err != nil {
		t.Fatalf("second Fetch() error = %v", err)
	}
	if changed {
		t.Fatal("second Fetch() changed = true after 304")
	}
	if len(items) != 0 {
		t.Fatalf("second Fetch() returned %d items, want 0", len(items))
	}
}
