package feed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Item struct {
	ID          string    `json:"id"`
	Source      string    `json:"source"`
	Title       string    `json:"title"`
	Summary     string    `json:"summary,omitempty"`
	URL         string    `json:"url,omitempty"`
	PublishedAt time.Time `json:"published_at,omitempty"`
	FetchedAt   time.Time `json:"fetched_at"`
}

type State struct {
	ETag         string
	LastModified string
}

type Client struct {
	HTTP *http.Client
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 20 * time.Second}}
}

func (c *Client) Fetch(ctx context.Context, source, url string, state *State) ([]Item, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "xd-dash-news/1.0 (+https://github.com/xd-dash/news)")
	if state.ETag != "" {
		req.Header.Set("If-None-Match", state.ETag)
	}
	if state.LastModified != "" {
		req.Header.Set("If-Modified-Since", state.LastModified)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("fetch feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return nil, false, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		return nil, false, fmt.Errorf("fetch feed: unexpected status %s", resp.Status)
	}

	state.ETag = resp.Header.Get("ETag")
	state.LastModified = resp.Header.Get("Last-Modified")

	items, err := Parse(source, resp.Body)
	if err != nil {
		return nil, false, err
	}
	now := time.Now().UTC()
	for i := range items {
		items[i].FetchedAt = now
	}
	return items, true, nil
}

type rssDocument struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	GUID        string `xml:"guid"`
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
}

type atomDocument struct {
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	ID      string `xml:"id"`
	Title   string `xml:"title"`
	Summary string `xml:"summary"`
	Content string `xml:"content"`
	Updated string `xml:"updated"`
	Published string `xml:"published"`
	Links   []struct {
		Href string `xml:"href,attr"`
		Rel  string `xml:"rel,attr"`
	} `xml:"link"`
}

func Parse(source string, r io.Reader) ([]Item, error) {
	data, err := io.ReadAll(io.LimitReader(r, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("read feed: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, errors.New("empty feed")
	}

	var rss rssDocument
	if err := xml.Unmarshal(data, &rss); err == nil && len(rss.Channel.Items) != 0 {
		items := make([]Item, 0, len(rss.Channel.Items))
		for _, raw := range rss.Channel.Items {
			published := parseTime(raw.PubDate)
			item := Item{Source: source, Title: strings.TrimSpace(raw.Title), Summary: strings.TrimSpace(raw.Description), URL: strings.TrimSpace(raw.Link), PublishedAt: published}
			item.ID = stableID(strings.TrimSpace(raw.GUID), item.URL, item.Title, published)
			items = append(items, item)
		}
		return items, nil
	}

	var atom atomDocument
	if err := xml.Unmarshal(data, &atom); err != nil {
		return nil, fmt.Errorf("parse feed XML: %w", err)
	}
	if len(atom.Entries) == 0 {
		return nil, errors.New("feed contains no RSS items or Atom entries")
	}

	items := make([]Item, 0, len(atom.Entries))
	for _, raw := range atom.Entries {
		url := ""
		for _, link := range raw.Links {
			if link.Rel == "" || link.Rel == "alternate" {
				url = strings.TrimSpace(link.Href)
				break
			}
		}
		published := parseTime(firstNonEmpty(raw.Published, raw.Updated))
		item := Item{Source: source, Title: strings.TrimSpace(raw.Title), Summary: strings.TrimSpace(firstNonEmpty(raw.Summary, raw.Content)), URL: url, PublishedAt: published}
		item.ID = stableID(strings.TrimSpace(raw.ID), item.URL, item.Title, published)
		items = append(items, item)
	}
	return items, nil
}

func stableID(explicit, url, title string, published time.Time) string {
	if explicit != "" {
		return explicit
	}
	sum := sha256.Sum256([]byte(url + "\x00" + title + "\x00" + published.UTC().Format(time.RFC3339Nano)))
	return hex.EncodeToString(sum[:])
}

func parseTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
