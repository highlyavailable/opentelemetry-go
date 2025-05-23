// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opencensus // import "go.opentelemetry.io/otel/bridge/opencensus"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	octrace "go.opencensus.io/trace"

	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func TestNewTraceBridge(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(trace.WithSyncer(exporter))
	bridge := newTraceBridge([]TraceOption{WithTracerProvider(tp)})
	_, span := bridge.StartSpan(context.Background(), "foo")
	span.End()
	gotSpans := exporter.GetSpans()
	require.Len(t, gotSpans, 1)
	gotSpan := gotSpans[0]
	assert.Equal(t, scopeName, gotSpan.InstrumentationScope.Name)
	assert.Equal(t, gotSpan.InstrumentationScope.Version, Version())
}

func TestOCSpanContextToOTel(t *testing.T) {
	input := octrace.SpanContext{
		TraceID:      [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:       [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
		TraceOptions: octrace.TraceOptions(1),
	}
	want := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    oteltrace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     oteltrace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
		TraceFlags: oteltrace.TraceFlags(1),
	})
	got := OCSpanContextToOTel(input)
	assert.Equal(t, want, got)
}

func TestInstallTraceBridge(t *testing.T) {
	// Store the original DefaultTracer to restore it later
	originalTracer := octrace.DefaultTracer
	defer func() {
		octrace.DefaultTracer = originalTracer
	}()

	tests := []struct {
		name             string
		opts             []TraceOption
		expectValidSpans bool
	}{
		{
			name:             "install with default options",
			opts:             nil,
			expectValidSpans: false, // Default uses global no-op tracer provider
		},
		{
			name: "install with custom tracer provider",
			opts: []TraceOption{
				WithTracerProvider(trace.NewTracerProvider()),
			},
			expectValidSpans: true,
		},
		{
			name: "install with tracer provider with exporter",
			opts: []TraceOption{
				WithTracerProvider(
					trace.NewTracerProvider(
						trace.WithSyncer(tracetest.NewInMemoryExporter()),
					),
				),
			},
			expectValidSpans: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get the current DefaultTracer before installation
			beforeTracer := octrace.DefaultTracer

			// Install the trace bridge
			InstallTraceBridge(tt.opts...)

			// Verify that DefaultTracer was changed
			assert.NotEqual(t, beforeTracer, octrace.DefaultTracer, "DefaultTracer should be updated after InstallTraceBridge")
			assert.NotNil(t, octrace.DefaultTracer, "DefaultTracer should not be nil after InstallTraceBridge")

			// Verify that the installed tracer can create spans
			ctx, span := octrace.DefaultTracer.StartSpan(context.Background(), "test-span")
			assert.NotNil(t, span, "Should be able to create spans with the installed tracer")
			assert.NotNil(t, ctx, "Should return a valid context")

			// Verify the span has a span context (may be empty for no-op tracers)
			spanContext := span.SpanContext()
			if tt.expectValidSpans {
				// For real tracer providers, expect non-zero IDs
				assert.NotEqual(t, octrace.TraceID{}, spanContext.TraceID, "Span should have a non-zero TraceID")
				assert.NotEqual(t, octrace.SpanID{}, spanContext.SpanID, "Span should have a non-zero SpanID")
			}
			// Always verify that we get a span context (even if empty)
			// This shows the tracer is properly installed and functional

			span.End()

			// Verify that the tracer can be used to get spans from context
			spanFromContext := octrace.DefaultTracer.FromContext(ctx)
			assert.NotNil(t, spanFromContext, "Should be able to get span from context")
		})
	}
}
