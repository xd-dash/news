# news

Lifecycle-scoped Go news feed collector that publishes normalized news items to Redis Pub/Sub. The HTTP request controls the lifetime of a collection session; consumers subscribe to Redis channels rather than holding the producer request open for data.

See `router/` for the implementation.
