// Package otelboot wires optional OpenTelemetry export for gist hosted registry.
// Activation requires OTEL_EXPORTER_OTLP_ENDPOINT (or _TRACES_ENDPOINT).
package otelboot

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Enabled reports whether OTLP export is configured via endpoint env vars.
func Enabled() bool {
	return endpointFromEnv() != ""
}

func endpointFromEnv() string {
	return firstNonEmpty(
		os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"),
	)
}

// Start configures OTLP exporters when an endpoint env var is set.
// Returns a no-op shutdown when telemetry is disabled.
func Start(ctx context.Context) (func(context.Context) error, error) {
	raw := endpointFromEnv()
	if raw == "" {
		return func(context.Context) error { return nil }, nil
	}
	target, secure, err := parseEndpoint(raw)
	if err != nil {
		return nil, err
	}
	if err := rejectUnsafeTarget(target); err != nil {
		return nil, err
	}

	serviceName := firstNonEmpty(os.Getenv("OTEL_SERVICE_NAME"), "gist")
	serviceVersion := firstNonEmpty(os.Getenv("OTEL_SERVICE_VERSION"), "unknown")
	// Prefer explicit DEPLOYMENT_ENVIRONMENT; default unknown (not "canary") so
	// production enablement cannot silently mislabel signals.
	deployEnv := firstNonEmpty(os.Getenv("DEPLOYMENT_ENVIRONMENT"), "unknown")
	// Build attributes without resource.Default() merge: Default and pinned
	// semconv schema URLs can conflict across otel minor releases.
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(serviceVersion),
		semconv.DeploymentEnvironmentName(deployEnv),
	)

	var creds credentials.TransportCredentials
	if secure {
		creds = credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
	} else {
		creds = insecure.NewCredentials()
	}
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("otel grpc dial: %w", err)
	}

	traceExp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("otel trace exporter: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	metricExp, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithGRPCConn(conn))
	if err != nil {
		_ = tp.Shutdown(ctx)
		_ = conn.Close()
		return nil, fmt.Errorf("otel metric exporter: %w", err)
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp, sdkmetric.WithInterval(15*time.Second))),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	logExp, err := otlploggrpc.New(ctx, otlploggrpc.WithGRPCConn(conn))
	if err != nil {
		_ = mp.Shutdown(ctx)
		_ = tp.Shutdown(ctx)
		_ = conn.Close()
		return nil, fmt.Errorf("otel log exporter: %w", err)
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
		sdklog.WithResource(res),
	)
	otel.SetLoggerProvider(lp)
	slog.SetDefault(slog.New(otelslog.NewHandler(serviceName, otelslog.WithLoggerProvider(lp))))

	slog.Info("otelboot started", "endpoint", target, "secure", secure, "service", serviceName, "version", serviceVersion)

	// Explicit counter so metric export is not dependent on otelhttp alone.
	meter := otel.Meter("gist")
	up, err := meter.Int64Counter("gist_otel_up", metric.WithDescription("gist otelboot active"))
	if err != nil {
		_ = lp.Shutdown(ctx)
		_ = mp.Shutdown(ctx)
		_ = tp.Shutdown(ctx)
		_ = conn.Close()
		return nil, fmt.Errorf("otel counter: %w", err)
	}
	up.Add(ctx, 1)

	return func(shutdownCtx context.Context) error {
		var first error
		for _, fn := range []func(context.Context) error{lp.Shutdown, mp.Shutdown, tp.Shutdown} {
			if err := fn(shutdownCtx); err != nil && first == nil {
				first = err
			}
		}
		if err := conn.Close(); err != nil && first == nil {
			first = err
		}
		return first
	}, nil
}

// WrapHandler adds panic recovery and HTTP tracing only when OTLP is enabled.
// When telemetry is off, returns next unchanged so panic/error behavior matches
// the pre-otelboot server. Canary induction is not installed here — register it
// on the authenticated mux via api.Server.RegisterOTELCanary.
func WrapHandler(next http.Handler) http.Handler {
	if !Enabled() {
		return next
	}
	inner := RecoverHandler(next)
	return otelhttp.NewHandler(inner, "gist.http",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + r.URL.Path
		}),
	)
}

// RecoverHandler converts panics into error spans/logs (crash family C) and a 500.
func RecoverHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			ctx, span := otel.Tracer("gist").Start(r.Context(), "panic")
			defer span.End()
			err := fmt.Errorf("panic: %v", rec)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			slog.ErrorContext(ctx, "panic recovered", "error", err.Error())
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		}()
		next.ServeHTTP(w, r)
	})
}

// InduceCanaryError writes a controlled 500 with an error span. Call only from
// an auth-gated route (see api.Server.RegisterOTELCanary).
func InduceCanaryError(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("gist").Start(r.Context(), "canary.error")
	defer span.End()
	err := fmt.Errorf("gist canary induced error")
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	slog.ErrorContext(ctx, "canary induced error", "error", err.Error())
	http.Error(w, `{"error":"canary induced error"}`, http.StatusInternalServerError)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// parseEndpoint returns the gRPC dial target and whether TLS is required.
// https:// uses TLS; http://, grpc://, and bare host:port use insecure
// (typical for in-cluster Alloy). Paths other than "/" are rejected — OTLP
// gRPC does not use /v1/traces URL paths.
func parseEndpoint(raw string) (target string, secure bool, err error) {
	raw = strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(raw, "https://"):
		secure = true
		raw = "https://" + strings.TrimPrefix(raw, "https://")
	case strings.HasPrefix(raw, "http://"):
		raw = "http://" + strings.TrimPrefix(raw, "http://")
	case strings.HasPrefix(raw, "grpc://"):
		raw = "http://" + strings.TrimPrefix(raw, "grpc://")
	default:
		// bare host:port
		if strings.Contains(raw, "/") {
			return "", false, fmt.Errorf("otel endpoint: path not allowed on bare target %q", raw)
		}
		return raw, false, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", false, fmt.Errorf("otel endpoint: %w", err)
	}
	if u.Host == "" {
		return "", false, fmt.Errorf("otel endpoint: missing host in %q", raw)
	}
	if u.Path != "" && u.Path != "/" {
		return "", false, fmt.Errorf("otel endpoint: gRPC target must not include path %q", u.Path)
	}
	return u.Host, secure, nil
}

func rejectUnsafeTarget(target string) error {
	host, _, err := net.SplitHostPort(target)
	if err != nil {
		// host without port
		host = target
	}
	lower := strings.ToLower(host)
	if lower == "metadata.google.internal" || lower == "metadata" {
		return fmt.Errorf("otel endpoint: refusing metadata host %q", host)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return nil // hostname; allow (cluster DNS / operator-set / localhost name)
	}
	// Block link-local and cloud metadata; allow loopback for local Alloy.
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return fmt.Errorf("otel endpoint: refusing unsafe address %q", host)
	}
	return nil
}
