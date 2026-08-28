package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/xd-dash/logma-serverless/pubsub"
	baserouter "github.com/xd-dash/logma-serverless/router"
)

// NewRouter returns the HTTP shell expected by the existing gospace/simple-router
// deployment pattern. A process can serve many sequential requests, but each
// active collection session gets one claim-once NewsRuntime and one Redis client.
func NewRouter() http.Handler {
	holder := pubsub.NewHolder(NewNewsRuntime)
	return baserouter.Build(func(r chi.Router) {
		r.Post("/stream", streamHandler(holder))
	})
}
