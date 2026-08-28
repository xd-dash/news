package router

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/xd-dash/logma/serverless/pubsub"
	"github.com/xd-dash/news/internal/feed"
)

type NewsRuntime struct {
	pubsub.Runtime

	cfg            streamConfig
	feedClient     *feed.Client
	globalChannels bool
	closeOnce      sync.Once
}

func NewNewsRuntime() *NewsRuntime {
	return &NewsRuntime{
		Runtime:        pubsub.NewRuntimeFromEnv(),
		feedClient:     feed.NewClient(),
		globalChannels: os.Getenv("NEWS_GLOBAL_CHANNELS") == "true",
	}
}

func (rt *NewsRuntime) Configure(cfg streamConfig) {
	rt.cfg = cfg
	rt.Runtime.ConfigureDefault(rt.streamFeeds, nil)
}

// Close releases the session-owned Redis client. It is idempotent so every
// handler exit path can defer it immediately after a successful Claim.
func (rt *NewsRuntime) Close() {
	rt.closeOnce.Do(func() {
		if rt.Client == nil {
			return
		}
		if err := rt.Client.Close(); err != nil {
			log.Printf("news: close redis client: %v", err)
		}
	})
}

func (rt *NewsRuntime) streamFeeds(ctx context.Context) error {
	var wg sync.WaitGroup
	for _, cfg := range rt.cfg.feeds {
		cfg := cfg
		wg.Add(1)
		go func() {
			defer wg.Done()
			rt.pollFeed(ctx, cfg)
		}()
	}

	<-ctx.Done()
	wg.Wait()
	return nil
}

func (rt *NewsRuntime) pollFeed(ctx context.Context, cfg feedConfig) {
	state := &feed.State{}
	seen := make(map[string]struct{})
	firstSuccessfulFetch := true

	poll := func() {
		items, changed, err := rt.feedClient.Fetch(ctx, cfg.name, cfg.url, state)
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("news: fetch %s: %v", cfg.name, err)
			}
			return
		}
		if !changed {
			return
		}

		for _, item := range items {
			if _, ok := seen[item.ID]; ok {
				continue
			}
			seen[item.ID] = struct{}{}
			if firstSuccessfulFetch && !rt.cfg.publishExisting {
				continue
			}
			if err := rt.Runtime.Publish(rt.publishChannel(cfg.name), item); err != nil {
				log.Printf("news: %v", err)
			}
		}
		firstSuccessfulFetch = false
	}

	poll()
	ticker := time.NewTicker(cfg.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			poll()
		}
	}
}

func (rt *NewsRuntime) publishChannel(source string) string {
	base := "news:item:" + source
	if rt.cfg.combinedChannel {
		base = "news:item:all"
	}
	if rt.globalChannels {
		return rt.GlobalChannel(base)
	}
	return rt.InstanceChannel(base)
}
