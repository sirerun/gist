package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/sirerun/gist/hosted/internal/app"
)

func main() {
	maintenance, err := maintenanceTargets(os.Getenv("GIST_EVENT_MAINTENANCE_TARGETS"))
	if err != nil {
		log.Fatal(err)
	}
	cfg := app.Config{
		ListenAddress: env("GIST_LISTEN_ADDRESS", ":8080"),
		PublicOrigin:  env("GIST_PUBLIC_ORIGIN", ""), ResourceAudience: env("GIST_RESOURCE_AUDIENCE", ""),
		DatabaseURL: env("GIST_DATABASE_URL", ""), ObjectStoreRoot: env("GIST_OBJECT_STORE_ROOT", ""), BrokerURL: env("GIST_CONNECTION_BROKER_URL", ""),
		RequestTimeout: durationEnv("GIST_REQUEST_TIMEOUT", 30*time.Second), MaxPackageBytes: int64Env("GIST_MAX_PACKAGE_BYTES", 10<<20), MaxExpandedBytes: int64Env("GIST_MAX_EXPANDED_PACKAGE_BYTES", 50<<20), MaxRequestBytes: int64Env("GIST_MAX_REQUEST_BYTES", 1<<20), MaxResponseBytes: int(int64Env("GIST_MAX_RESPONSE_BYTES", 2<<20)), MaxCatalogEntries: int(int64Env("GIST_MAX_CATALOG_ENTRIES", 100000)), MaxConcurrentRequests: int(int64Env("GIST_MAX_CONCURRENT_REQUESTS", 80)), MaxDiscoveryResults: int(int64Env("GIST_MAX_DISCOVERY_RESULTS", 50)), RetryAfter: 1,
		OAuthConsentSecret:      []byte(os.Getenv("GIST_OAUTH_CONSENT_SECRET")),
		SigningKeyConfig:        []byte(os.Getenv("GIST_SIGNING_KEY_CONFIG")),
		EventMaintenanceTargets: maintenance,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	a, err := app.New(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		if err := a.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.Shutdown(shutdown); err != nil {
		log.Fatal(err)
	}
}

func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
func durationEnv(k string, fallback time.Duration) time.Duration {
	if v, err := time.ParseDuration(os.Getenv(k)); err == nil && v > 0 {
		return v
	}
	return fallback
}
func int64Env(k string, fallback int64) int64 {
	if v, err := strconv.ParseInt(os.Getenv(k), 10, 64); err == nil && v > 0 {
		return v
	}
	return fallback
}

func maintenanceTargets(raw string) ([]app.MaintenanceTarget, error) {
	return app.DecodeMaintenanceTargets([]byte(raw))
}
