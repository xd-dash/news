package feed

import (
	"strings"
	"testing"
)

func TestParseRSS(t *testing.T) {
	items, err := Parse("wire", strings.NewReader(`<?xml version="1.0"?><rss><channel><item><guid>abc</guid><title>Headline</title><description>Summary</description><link>https://example.com/a</link><pubDate>Thu, 27 Aug 2026 20:00:00 -0700</pubDate></item></channel></rss>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "abc" || items[0].Source != "wire" || items[0].Title != "Headline" {
		t.Fatalf("unexpected items: %#v", items)
	}
}

func TestParseAtom(t *testing.T) {
	items, err := Parse("atom", strings.NewReader(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"><entry><id>x1</id><title>Atom headline</title><summary>Summary</summary><published>2026-08-28T03:00:00Z</published><link rel="alternate" href="https://example.com/x1"/></entry></feed>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].URL != "https://example.com/x1" {
		t.Fatalf("unexpected items: %#v", items)
	}
}
