package auth

import (
	"context"
	"fmt"
	"sync"

	"github.com/braintrustdata/braintrust-x-go/logger"
)

// Session manages authentication and login state.
type Session struct {
	mu     sync.RWMutex
	info   *Info
	err    error
	done   chan struct{}
	logger logger.Logger
	ctx    context.Context
	cancel context.CancelFunc
	opts   Options // Store original options for access before login completes
}

// Endpoints holds the API credentials and URLs from session options.
type Endpoints struct {
	APIKey string
	APIURL string
	AppURL string
}

// NewSession creates a session and starts login with retry in the background.
// Returns an error if required fields (APIKey, AppURL) are missing.
// The context is used for the background login goroutine.
// If opts.Logger is nil, a noop logger is used.
func NewSession(ctx context.Context, opts Options) (*Session, error) {
	if opts.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}
	if opts.AppURL == "" {
		return nil, fmt.Errorf("app URL is required")
	}

	// Use discard logger if none provided
	log := opts.Logger
	if log == nil {
		log = logger.Discard()
	}

	sessionCtx, cancel := context.WithCancel(ctx)
	s := &Session{
		logger: log,
		done:   make(chan struct{}),
		ctx:    sessionCtx,
		cancel: cancel,
		opts:   opts,
	}
	go s.loginWithRetry(opts)
	return s, nil
}

// Close cancels the background login goroutine.
func (s *Session) Close() {
	if s.cancel != nil {
		s.cancel()
	}
}

// Endpoints returns the API credentials that were passed to the session.
// This is always available immediately, no login required.
func (s *Session) Endpoints() Endpoints {
	apiURL := s.opts.APIURL
	if apiURL == "" {
		apiURL = "https://api.braintrust.dev" // Default
	}
	return Endpoints{
		APIKey: s.opts.APIKey,
		APIURL: apiURL,
		AppURL: s.opts.AppURL,
	}
}

// OrgName returns the organization name if available.
// Returns empty string if login hasn't completed yet.
func (s *Session) OrgName() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.info != nil {
		return s.info.OrgName
	}
	return ""
}

// Info returns current auth info (non-blocking).
// Returns (ok=true, info) if login succeeded.
// Returns (ok=false, nil) if login is in progress or failed.
func (s *Session) Info() (bool, *Info) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.info != nil && s.info.LoggedIn {
		return true, s.info
	}
	return false, nil
}

// Login blocks until login completes or context is cancelled.
// Returns info and error if login failed.
func (s *Session) Login(ctx context.Context) (*Info, error) {
	select {
	case <-s.done:
		s.mu.RLock()
		defer s.mu.RUnlock()
		return s.info, s.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Session) loginWithRetry(opts Options) {
	defer close(s.done)

	s.logger.Debug("starting login with retry")

	// Use loginUntilSuccess which retries on network/5xx errors
	info, err := loginUntilSuccess(s.ctx, opts.APIKey, opts.AppURL, opts.AppPublicURL, opts.OrgName, opts.Logger)

	s.mu.Lock()
	defer s.mu.Unlock()

	if err != nil {
		s.err = err
		s.logger.Warn("login failed", "error", err)
		return
	}

	s.info = info
	s.logger.Info("login successful",
		"org_name", s.info.OrgName,
		"org_id", s.info.OrgID)
}
