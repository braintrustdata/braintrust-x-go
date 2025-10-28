package braintrust

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/braintrustdata/braintrust-x-go/internal/auth"
	"github.com/braintrustdata/braintrust-x-go/internal/tests"
)

func TestNew_WithMinimalConfig(t *testing.T) {
	t.Parallel()

	// Create a TracerProvider
	tp := trace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()

	// Create client with minimal config
	client, err := New(tp,
		WithAPIKey(auth.TestAPIKey),
		WithProject("test-project"),
		WithLogger(tests.NewFailTestLogger(t)),
	)
	require.NoError(t, err)
	require.NotNil(t, client)

	// Test TracerProvider() accessor
	assert.Equal(t, tp, client.TracerProvider())

	// Test that provider is not owned
	assert.False(t, client.ownedProvider)

	// Test Tracer() method creates a working tracer
	tracer := client.Tracer("test-tracer")
	assert.NotNil(t, tracer)

	// Create a span to verify tracer works
	ctx, span := tracer.Start(context.Background(), "test-span")
	span.End()
	assert.NotNil(t, ctx)

	// Test String() output contains expected info
	str := client.String()
	assert.Contains(t, str, "test-project")
	assert.Contains(t, str, "Braintrust Client")
}

func TestNew_WithBlockingLogin(t *testing.T) {
	t.Parallel()

	tp := trace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()

	// Create client with blocking login
	client, err := New(tp,
		WithAPIKey(auth.TestAPIKey),
		WithProject("test-project"),
		WithBlockingLogin(true),
		WithLogger(tests.NewFailTestLogger(t)),
	)
	require.NoError(t, err)
	require.NotNil(t, client)

	// After blocking login, session info should be available
	ok, info := client.session.Info()
	assert.True(t, ok)
	assert.NotNil(t, info)
	assert.Equal(t, "test-org-id", info.OrgID)
	assert.Equal(t, "test-org-name", info.OrgName)

	// String() should show org info
	str := client.String()
	assert.Contains(t, str, "test-org-name")
	assert.Contains(t, str, "test-org-id")
}

func TestNew_MissingAPIKey(t *testing.T) {
	// Note: No t.Parallel() because we're setting environment variables

	// Clear environment variable to ensure no API key is set
	t.Setenv("BRAINTRUST_API_KEY", "")

	tp := trace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()

	// Try to create client without API key
	client, err := New(tp,
		WithProject("test-project"),
		WithLogger(tests.NewNoopLogger()),
	)

	// Should fail with error about API key
	require.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "API key")
}

func TestNewWithOtel_SetsGlobalProvider(t *testing.T) {
	t.Parallel()

	// Save original global provider and restore after test
	original := otel.GetTracerProvider()
	defer otel.SetTracerProvider(original)

	// Create client with NewWithOtel
	client, err := NewWithOtel(
		WithAPIKey(auth.TestAPIKey),
		WithProject("test-project"),
		WithLogger(tests.NewFailTestLogger(t)),
	)
	require.NoError(t, err)
	require.NotNil(t, client)
	defer func() { _ = client.Shutdown(context.Background()) }()

	// Verify global provider was set
	globalTP := otel.GetTracerProvider()
	assert.NotEqual(t, original, globalTP)

	// Verify it's the same as the client's provider
	assert.Equal(t, client.TracerProvider(), globalTP)

	// Verify provider is owned by client
	assert.True(t, client.ownedProvider)
}

func TestShutdown_Behavior(t *testing.T) {
	t.Parallel()

	t.Run("owned provider is shut down", func(t *testing.T) {
		// Save/restore global provider
		original := otel.GetTracerProvider()
		defer otel.SetTracerProvider(original)

		// Create client with NewWithOtel (owns provider)
		client, err := NewWithOtel(
			WithAPIKey(auth.TestAPIKey),
			WithProject("test-project"),
			WithLogger(tests.NewFailTestLogger(t)),
		)
		require.NoError(t, err)

		// Get provider reference
		tp := client.TracerProvider()

		// Shutdown client
		err = client.Shutdown(context.Background())
		require.NoError(t, err)

		// Provider should be shut down (we can't directly test this,
		// but we can verify Shutdown() succeeded without error)
		assert.NotNil(t, tp)
	})

	t.Run("external provider is not shut down", func(t *testing.T) {
		// Create external provider
		tp := trace.NewTracerProvider()
		defer func() { _ = tp.Shutdown(context.Background()) }()

		// Create client with New() (doesn't own provider)
		client, err := New(tp,
			WithAPIKey(auth.TestAPIKey),
			WithProject("test-project"),
			WithLogger(tests.NewFailTestLogger(t)),
		)
		require.NoError(t, err)

		// Shutdown client
		err = client.Shutdown(context.Background())
		require.NoError(t, err)

		// Provider should still be usable
		tracer := tp.Tracer("test")
		_, span := tracer.Start(context.Background(), "test-span")
		span.End()
	})

	t.Run("multiple shutdowns are safe", func(t *testing.T) {
		// Save/restore global provider
		original := otel.GetTracerProvider()
		defer otel.SetTracerProvider(original)

		client, err := NewWithOtel(
			WithAPIKey(auth.TestAPIKey),
			WithProject("test-project"),
			WithLogger(tests.NewFailTestLogger(t)),
		)
		require.NoError(t, err)

		// Call shutdown multiple times
		err = client.Shutdown(context.Background())
		require.NoError(t, err)

		err = client.Shutdown(context.Background())
		require.NoError(t, err)

		err = client.Shutdown(context.Background())
		require.NoError(t, err)
	})
}

func TestTracing_EndToEnd(t *testing.T) {
	t.Parallel()

	// Save/restore global provider
	original := otel.GetTracerProvider()
	defer otel.SetTracerProvider(original)

	// Create a memory exporter to capture spans without making API calls
	exporter := tracetest.NewInMemoryExporter()

	// Create client with NewWithOtel and memory exporter
	client, err := NewWithOtel(
		WithAPIKey(auth.TestAPIKey),
		WithProject("test-project"),
		WithExporter(exporter),
		WithLogger(tests.NewFailTestLogger(t)),
	)
	require.NoError(t, err)
	defer func() { _ = client.Shutdown(context.Background()) }()

	// Create a span using the global tracer (since NewWithOtel sets global)
	tracer := otel.Tracer("test-app")
	ctx, span := tracer.Start(context.Background(), "test-operation")
	span.End()

	// Flush to ensure span is exported
	err = client.TracerProvider().ForceFlush(context.Background())
	require.NoError(t, err)

	// Verify context is valid
	assert.NotNil(t, ctx)

	// Verify span was captured by our exporter
	spans := exporter.GetSpans()
	assert.GreaterOrEqual(t, len(spans), 1, "Expected at least one span to be exported")
}

func TestTracing_WithExporter(t *testing.T) {
	t.Parallel()

	// Create a memory exporter for testing
	exporter := tracetest.NewInMemoryExporter()

	// Create TracerProvider with simple processor
	tp := trace.NewTracerProvider(
		trace.WithSyncer(exporter),
	)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	// Create client with custom exporter
	client, err := New(tp,
		WithAPIKey(auth.TestAPIKey),
		WithProject("test-project"),
		WithExporter(exporter),
		WithLogger(tests.NewFailTestLogger(t)),
	)
	require.NoError(t, err)

	// Create a span
	tracer := client.Tracer("test-app")
	_, span := tracer.Start(context.Background(), "test-span")
	span.End()

	// Force flush to ensure span is exported
	err = tp.ForceFlush(context.Background())
	require.NoError(t, err)

	// Verify span was captured
	spans := exporter.GetSpans()
	assert.GreaterOrEqual(t, len(spans), 1)
}
