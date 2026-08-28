package feed

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestLivePublicFeeds(t *testing.T) {
	if os.Getenv("NEWS_LIVE_TESTS") != "1" {
		t.Skip("set NEWS_LIVE_TESTS=1 to test public news feeds")
	}

	tests := []struct {
		name   string
		source string
		url    string
	}{
		{
			name:   "GlobeNewswire earnings RSS",
			source: "globenewswire",
			url:    "https://www.globenewswire.com/RssFeed/subjectcode/13-Earnings%20Releases%20And%20Operating%20Results/feedTitle/GlobeNewswire%20-%20Earnings%20Releases%20And%20Operating%20Results",
		},
		{
			name:   "SEC current filings Atom",
			source: "sec",
			url:    "https://www.sec.gov/cgi-bin/browse-edgar?action=getcurrent&output=atom",
		},
	}

	client := NewClient()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			items, changed, err := client.Fetch(ctx, tt.source, tt.url, &State{})
			if err != nil {
				t.Fatalf("Fetch() error = %v", err)
			}
			if !changed {
				t.Fatal("Fetch() changed = false on initial request")
			}
			if len(items) == 0 {
				t.Fatal("Fetch() returned no items")
			}

			for i, item := range items {
				if item.ID == "" {
					t.Fatalf("items[%d].ID is empty", i)
				}
				if item.Source != tt.source {
					t.Fatalf("items[%d].Source = %q, want %q", i, item.Source, tt.source)
				}
				if item.Title == "" {
					t.Fatalf("items[%d].Title is empty", i)
				}
				if item.FetchedAt.IsZero() {
					t.Fatalf("items[%d].FetchedAt is zero", i)
				}
			}
		})
	}
}
