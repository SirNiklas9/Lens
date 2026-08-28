package inspector

import (
	"encoding/json"
	"net/http"
)

// Handler exposes Inspector's local read-only API. It intentionally has no
// write routes and no UI assets; a future native embedded-web window consumes
// these endpoints without coupling the API to any harness or browser.
func Handler(sources map[string]Source) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/snapshot", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, Build(r.Context(), sources))
	})
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		s := Build(r.Context(), sources)
		writeJSON(w, s.Health)
	})
	mux.HandleFunc("/v1/ai-context", func(w http.ResponseWriter, r *http.Request) {
		value, err := BuildAIContext(Build(r.Context(), sources))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, value)
	})
	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}
