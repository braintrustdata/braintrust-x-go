// Package devserver provides a local HTTP server for remote evaluation of AI models.
// It enables running evaluators defined in local code from the Braintrust web interface.
package devserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/braintrustdata/braintrust-x-go/braintrust/eval"
)

// Config contains configuration options for the dev server.
type Config struct {
	// Host is the hostname to listen on (default: "localhost")
	Host string
	// Port is the port to listen on (default: 8300)
	Port int
	// OrgName optionally restricts the server to a specific organization
	OrgName string
}

// Server is the dev server that handles remote evaluation requests.
type Server struct {
	config   Config
	mux      *http.ServeMux
	registry map[string]*registeredEval
}

// New creates a new dev server with the given configuration.
func New(config Config) *Server {
	// Set defaults
	if config.Host == "" {
		config.Host = "localhost"
	}
	if config.Port == 0 {
		config.Port = 8300
	}

	s := &Server{
		config:   config,
		mux:      http.NewServeMux(),
		registry: make(map[string]*registeredEval),
	}

	// Register handlers
	s.mux.HandleFunc("/", s.handleRoot)
	s.mux.HandleFunc("/list", s.handleList)
	s.mux.HandleFunc("/eval", s.handleEval)

	return s
}

// Start starts the HTTP server and blocks until the server exits.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	log.Printf("Starting dev server on http://%s", addr)

	// Wrap the mux with middleware (CORS first, then logging)
	handler := s.corsMiddleware(s.mux)
	handler = s.loggingMiddleware(handler)

	return http.ListenAndServe(addr, handler)
}

// loggingMiddleware logs all HTTP requests with method, path, status, and duration.
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Log the incoming request immediately
		log.Printf("→ %s %s from %s (Origin: %q)", r.Method, r.URL.Path, r.RemoteAddr, r.Header.Get("Origin"))

		// Create a response writer wrapper to capture the status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		log.Printf("← %s %s - %d (%v)", r.Method, r.URL.Path, wrapped.statusCode, duration)
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Register registers a RemoteEval for remote execution from the Braintrust web interface.
// It accepts a typed RemoteEval[I, R] and internally converts it to a type-erased representation
// that handles JSON serialization/deserialization.
func Register[I, R any](s *Server, re RemoteEval[I, R]) error {
	if re.Name == "" {
		return fmt.Errorf("evaluator name is required")
	}
	if re.ProjectName == "" {
		return fmt.Errorf("evaluator project name is required")
	}
	if re.Task == nil {
		return fmt.Errorf("evaluator task is required")
	}

	// Wrap the typed task to handle JSON conversion
	wrappedTask := func(ctx context.Context, input interface{}) (interface{}, error) {
		var typedInput I

		// Marshal input to JSON and unmarshal to typed input
		// This handles conversion from map[string]interface{} (from HTTP request) to I
		jsonBytes, err := json.Marshal(input)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal input: %w", err)
		}

		if err := json.Unmarshal(jsonBytes, &typedInput); err != nil {
			return nil, fmt.Errorf("failed to unmarshal input to type %T: %w", typedInput, err)
		}

		// Call the user's typed task
		result, err := re.Task(ctx, typedInput)
		if err != nil {
			return nil, err
		}

		// Return result as interface{} (will be marshaled to JSON in response)
		return result, nil
	}

	// Wrap scorers similarly and extract names
	wrappedScorers := make([]func(context.Context, interface{}, interface{}, interface{}, eval.Metadata) (eval.Scores, error), len(re.Scorers))
	scorerNames := make([]string, len(re.Scorers))
	for i, scorer := range re.Scorers {
		scorerCopy := scorer // Capture for closure
		scorerNames[i] = scorer.Name()
		wrappedScorers[i] = func(ctx context.Context, input, expected, result interface{}, meta eval.Metadata) (eval.Scores, error) {
			// Convert interface{} inputs to typed I and R
			var typedInput I
			var typedExpected R
			var typedResult R

			// Marshal/unmarshal to convert types
			if err := convertViaJSON(input, &typedInput); err != nil {
				return nil, fmt.Errorf("failed to convert input: %w", err)
			}
			if err := convertViaJSON(expected, &typedExpected); err != nil {
				return nil, fmt.Errorf("failed to convert expected: %w", err)
			}
			if err := convertViaJSON(result, &typedResult); err != nil {
				return nil, fmt.Errorf("failed to convert result: %w", err)
			}

			return scorerCopy.Run(ctx, typedInput, typedExpected, typedResult, meta)
		}
	}

	// Store in registry
	s.registry[re.Name] = &registeredEval{
		name:        re.Name,
		projectName: re.ProjectName,
		task:        wrappedTask,
		scorers:     wrappedScorers,
		scorerNames: scorerNames,
	}

	log.Printf("Registered evaluator: %s (project: %s)", re.Name, re.ProjectName)
	return nil
}

// convertViaJSON converts src to dst by marshaling to JSON and unmarshaling back.
// This is a helper for type conversion in the type-erased wrappers.
func convertViaJSON(src, dst interface{}) error {
	jsonBytes, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonBytes, dst)
}
