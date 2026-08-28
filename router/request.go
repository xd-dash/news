package router

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type FeedRequest struct {
	Name                string `json:"name"`
	URL                 string `json:"url"`
	PollIntervalSeconds int    `json:"poll_interval_seconds,omitempty"`
}

type StreamRequest struct {
	Feeds           []FeedRequest `json:"feeds"`
	CombinedChannel bool          `json:"combined_channel,omitempty"`
	PublishExisting bool          `json:"publish_existing,omitempty"`
}

type feedConfig struct {
	name     string
	url      string
	interval time.Duration
}

type streamConfig struct {
	feeds           []feedConfig
	combinedChannel bool
	publishExisting bool
}

func (r StreamRequest) validate() (streamConfig, error) {
	if len(r.Feeds) == 0 {
		return streamConfig{}, errors.New("feeds must be non-empty")
	}

	seen := make(map[string]struct{}, len(r.Feeds))
	feeds := make([]feedConfig, 0, len(r.Feeds))
	for i, raw := range r.Feeds {
		name := strings.ToLower(strings.TrimSpace(raw.Name))
		if name == "" {
			return streamConfig{}, fmt.Errorf("feeds[%d].name must be non-empty", i)
		}
		if strings.Contains(name, ":") {
			return streamConfig{}, fmt.Errorf("feeds[%d].name must not contain ':'", i)
		}
		if _, ok := seen[name]; ok {
			return streamConfig{}, fmt.Errorf("duplicate feed name %q", name)
		}
		seen[name] = struct{}{}

		u, err := url.Parse(strings.TrimSpace(raw.URL))
		if err != nil || u.Scheme == "" || u.Host == "" {
			return streamConfig{}, fmt.Errorf("feeds[%d].url must be an absolute URL", i)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return streamConfig{}, fmt.Errorf("feeds[%d].url must use http or https", i)
		}

		seconds := raw.PollIntervalSeconds
		if seconds == 0 {
			seconds = 60
		}
		if seconds < 15 || seconds > 3600 {
			return streamConfig{}, fmt.Errorf("feeds[%d].poll_interval_seconds must be between 15 and 3600", i)
		}
		feeds = append(feeds, feedConfig{name: name, url: u.String(), interval: time.Duration(seconds) * time.Second})
	}

	return streamConfig{feeds: feeds, combinedChannel: r.CombinedChannel, publishExisting: r.PublishExisting}, nil
}
