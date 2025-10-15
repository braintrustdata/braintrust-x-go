package devserver

import (
	"log"
	"net/http"
	"strings"
)

// corsMiddleware adds CORS headers to allow requests from Braintrust web interface.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		log.Printf("[CORS] Method: %s, Path: %s, Origin: %q", r.Method, r.URL.Path, origin)

		// Set CORS headers if origin is allowed
		allowed := false
		if origin != "" {
			allowed = isAllowedOrigin(origin)
			log.Printf("[CORS] Origin allowed: %v", allowed)

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				log.Printf("[CORS] Set Allow-Origin: %s", origin)
			}
		}

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			log.Printf("[CORS] Handling OPTIONS preflight")

			// Must set CORS headers on preflight response
			if origin != "" && allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Api-Key, x-bt-auth-token, x-bt-parent, x-bt-org-name, x-bt-stream-fmt, x-bt-use-cache")
			w.Header().Set("Access-Control-Max-Age", "86400")

			// Check for private network access request
			privateNetwork := r.Header.Get("Access-Control-Request-Private-Network")
			if privateNetwork == "true" {
				w.Header().Set("Access-Control-Allow-Private-Network", "true")
				log.Printf("[CORS] Set Allow-Private-Network: true")
			}

			// Log all response headers
			log.Printf("[CORS] Preflight response headers:")
			for name, values := range w.Header() {
				for _, value := range values {
					log.Printf("[CORS]   %s: %s", name, value)
				}
			}

			w.WriteHeader(http.StatusOK)
			return
		}

		// Add expose headers for actual requests
		w.Header().Set("Access-Control-Expose-Headers", "x-bt-cursor, x-bt-found-existing-experiment, x-bt-span-id, x-bt-span-export")

		next.ServeHTTP(w, r)
	})
}

// isAllowedOrigin checks if the origin is in the whitelist.
func isAllowedOrigin(origin string) bool {
	// List of allowed origins
	allowedOrigins := []string{
		"https://www.braintrust.dev",
		"https://www.braintrustdata.com",
	}

	// Check exact matches
	for _, allowed := range allowedOrigins {
		if origin == allowed {
			return true
		}
	}

	// Check preview environment pattern: https://*.preview.braintrust.dev
	if strings.HasSuffix(origin, ".preview.braintrust.dev") && strings.HasPrefix(origin, "https://") {
		return true
	}

	// For development, also allow localhost origins
	if strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "http://127.0.0.1") {
		return true
	}

	return false
}
