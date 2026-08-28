package router

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/xd-dash/logma/serverless/pubsub"
)

func streamHandler(holder *pubsub.Holder[*NewsRuntime]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req StreamRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		cfg, err := req.validate()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		rt, ok := holder.Claim()
		if !ok {
			http.Error(w, "news stream already running", http.StatusConflict)
			return
		}
		defer rt.Close()

		rt.RecordInvocation(r, middleware.GetReqID(r.Context()))
		rt.Configure(cfg)
		rt.Start(r.Context())

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
	}
}
