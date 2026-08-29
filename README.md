# news

`news` is a lifecycle-scoped Go news collector that polls RSS/Atom feeds and publishes normalized items to Redis Pub/Sub. It follows the same claim-once/session-owned runtime pattern used by `xd-dash/stonks` on its `codex/lifecycle-hardening` branch: the HTTP container may survive many requests, but each active stream gets a fresh runtime, owns its Redis client, blocks for the stream lifetime, joins workers on cancellation, and closes idempotently.

The producer does not stream article data back over HTTP. Consumers listen on Redis directly or through `logma-serverless`.

## HTTP API

`router.NewRouter()` returns the plain `http.Handler` expected by the existing gospace/simple-router deployment shell.

Start one collection session with `POST /stream`:

```json
{
  "feeds": [
    {
      "name": "globenewswire",
      "url": "https://example.com/globenewswire.rss",
      "poll_interval_seconds": 60
    },
    {
      "name": "prnewswire",
      "url": "https://example.com/prnewswire.rss",
      "poll_interval_seconds": 60
    }
  ],
  "combined_channel": false,
  "publish_existing": false
}
```

`feeds` is required. Feed URLs may be HTTP or HTTPS RSS/Atom endpoints. Poll intervals default to 60 seconds and must be between 15 and 3600 seconds.

`publish_existing` defaults to false. On the first successful fetch the runtime seeds its in-memory dedupe set without publishing the feed's existing backlog; only items first observed on later fetches are emitted. Set it to true when replaying the current feed is desired.

The collector sends `If-None-Match` and `If-Modified-Since` when the source provides `ETag` or `Last-Modified`, so unchanged feeds normally return `304 Not Modified` instead of being reparsed.

## Redis channels

By default each source publishes to an instance-scoped channel:

```text
news:item:<source>:<instanceID>
```

For example:

```text
news:item:globenewswire:7f4c...
```

With `"combined_channel": true`, every configured feed publishes to:

```text
news:item:all:<instanceID>
```

Set `NEWS_GLOBAL_CHANNELS=true` only for a deliberately single-producer/global-channel deployment. Channels then end in `:global` instead of the runtime instance ID.

Each Redis payload is normalized JSON:

```json
{
  "id": "source-guid-or-stable-sha256",
  "source": "globenewswire",
  "title": "Example headline",
  "summary": "Example summary",
  "url": "https://example.com/story",
  "published_at": "2026-08-28T03:00:00Z",
  "fetched_at": "2026-08-28T03:00:07Z"
}
```

## Lifecycle

A stream request follows this shape:

```text
POST /stream
    |
    v
validate immutable feed config
    |
    v
claim fresh NewsRuntime
    |
    v
RecordInvocation
    |
    v
Configure
    |
    v
Start
    |
    +--> Redis control:shutdown subscription
    |
    +--> one polling goroutine per feed
             |
             +--> conditional HTTP GET
             +--> parse RSS/Atom
             +--> dedupe
             +--> Redis PUBLISH
    |
request cancellation / control:shutdown
    |
    v
cancel pollers
    |
    v
join all pollers
    |
    v
close Redis exactly once
```

Transient source errors are logged and retried on the next polling interval rather than killing the whole multi-feed session.

News currently selects Logma's package-owned `sandbox_bounded` lifecycle preset in code. The HTTP request cannot choose or override the concrete limits. The preset combines a Redis-owned shutdown timer with a total-publish cap; whichever condition is reached first emits the runtime shutdown signal. This is intentionally a sandbox policy and can later be replaced by deployment-selected/licensed presets without changing the stream request schema.

## Configuration

Redis configuration follows `logma-serverless` conventions:

- `REDIS_URI`
- `REDISCLI_AUTH`
- `NEWS_GLOBAL_CHANNELS=true` for global output channels

If Redis environment configuration is intentionally absent, the shared `logma-serverless/pubsub` runtime can populate it from request headers before the session starts, matching the stonks lifecycle model.

There are intentionally no GitHub Actions workflows in this repository yet.
