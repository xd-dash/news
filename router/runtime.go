package router

import (
	"context"
	"errors"
	"log"
	"os"
	"sync"
	"time"

	"github.com/xd-dash/logma/serverless/callbacks/axiom"
	"github.com/xd-dash/logma/serverless/pubsub"
	"github.com/xd-dash/news/internal/feed"
)

type NewsRuntime struct {
	pubsub.Runtime

	cfg            streamConfig
	feedClient     *feed.Client
	globalChannels bool
	axiomObserver  *axiom.Observer
	closeOnce      sync.Once
}

func NewNewsRuntime() *NewsRuntime {
	observer := axiom.FromEnv()
	runtime := pubsub.NewRuntimeFromEnv()
	runtime.SetObserver(observer)
	return &NewsRuntime{
		Runtime:        runtime,
		feedClient:     feed.NewClient(),
		globalChannels: os.Getenv("NEWS_GLOBAL_CHANNELS") == "true",
		axiomObserver:  observer,
	}
}

func (rt *NewsRuntime) Configure(cfg streamConfig) {
	rt.cfg = cfg
	rt.Runtime.ConfigureDefaultWithLifecycle(
		deploymentLifecyclePolicy(),
		rt.streamFeeds,
		nil,
	)
}

func deploymentLifecyclePolicy() pubsub.Policy {
	switch os.Getenv("NEWS_LIFECYCLE_POLICY") {
	case string(pubsub.Policy3S):
		return pubsub.Policy3S
	case string(pubsub.Policy3Publishes):
		return pubsub.Policy3Publishes
	case "", string(pubsub.Policy30S64Publishes):
		return pubsub.Policy30S64Publishes
	case string(pubsub.Policy5M):
		return pubsub.Policy5M
	case string(pubsub.Policy20M):
		return pubsub.Policy20M
	default:
		log.Printf("news: unknown NEWS_LIFECYCLE_POLICY; using 30s-64-publishes")
		return pubsub.Policy30S64Publishes
	}
}

// Close releases the optional observer and session-owned Redis client. It is
// idempotent so every handler exit path can defer it immediately after a
// successful Claim. Axiom remains best-effort and never changes data-plane
// success or lifecycle authority.
func (rt *NewsRuntime) Close() {
	rt.closeOnce.Do(func() {
		if rt.axiomObserver != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := rt.axiomObserver.Close(ctx); err != nil {
				log.Printf("news: close axiom observer: %v", err)
			}
			cancel()
			if sent, failed, dropped := rt.axiomObserver.Sent(), rt.axiomObserver.Failed(), rt.axiomObserver.Dropped(); sent > 0 || failed > 0 || dropped > 0 {
				log.Printf("news: axiom observer sent=%d failed=%d dropped=%d", sent, failed, dropped)
			}
		}
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
				if errors.Is(err, pubsub.ErrLifecyclePublishLimit) || ctx.Err() != nil {
					return
				}
				log.Printf("news: %v", err)
			}
			if ctx.Err() != nil {
				return
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
