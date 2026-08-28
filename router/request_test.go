package router

import "testing"

func TestStreamRequestValidation(t *testing.T) {
	cfg, err := (StreamRequest{Feeds: []FeedRequest{{Name: " GlobeNewswire ", URL: "https://example.com/feed.xml"}}}).validate()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.feeds) != 1 || cfg.feeds[0].name != "globenewswire" || cfg.feeds[0].interval.Seconds() != 60 {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestStreamRequestRejectsDuplicateNames(t *testing.T) {
	_, err := (StreamRequest{Feeds: []FeedRequest{
		{Name: "wire", URL: "https://example.com/a"},
		{Name: "WIRE", URL: "https://example.com/b"},
	}}).validate()
	if err == nil {
		t.Fatal("expected duplicate feed validation error")
	}
}
