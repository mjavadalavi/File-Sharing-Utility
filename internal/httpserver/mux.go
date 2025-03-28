package httpserver

import (
	"net/http"
)

// Mux is a custom HTTP multiplexer
type Mux struct {
	routes map[string]http.HandlerFunc
}

// muxEntry represents a route entry in the mux
type muxEntry struct {
	pattern string
	handler http.HandlerFunc
}

// NewMux creates a new custom multiplexer
func NewMux() *Mux {
	return &Mux{
		routes: make(map[string]http.HandlerFunc),
	}
}

// HandleFunc registers a handler function for a given pattern
func (m *Mux) HandleFunc(pattern string, handler http.HandlerFunc) {
	m.routes[pattern] = handler
}

// ServeHTTP implements the http.Handler interface
func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Look for exact match first
	if handler, ok := m.routes[r.URL.Path]; ok {
		handler(w, r)
		return
	}

	// No match found
	http.NotFound(w, r)
}

// appendSorted is a helper function, likely for maintaining sorted routes
func appendSorted(entries []muxEntry, entry muxEntry) []muxEntry {
	return append(entries, entry)
} 