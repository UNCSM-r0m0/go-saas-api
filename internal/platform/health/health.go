package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
)

// Checker performs health checks on dependencies.
type Checker struct {
	pgPool *pgxpool.Pool
	redis  *redis.Client
	nats   *nats.Conn
}

// NewChecker creates a health checker.
func NewChecker(pgPool *pgxpool.Pool, redisClient *redis.Client, nc *nats.Conn) *Checker {
	return &Checker{
		pgPool: pgPool,
		redis:  redisClient,
		nats:   nc,
	}
}

// Status holds the health status of a component.
type Status struct {
	Healthy bool   `json:"healthy"`
	Error   string `json:"error,omitempty"`
	Latency string `json:"latency,omitempty"`
}

// Report holds the full health report.
type Report struct {
	Healthy    bool              `json:"healthy"`
	Timestamp  time.Time         `json:"timestamp"`
	Components map[string]Status `json:"components"`
}

// Check runs all health checks and returns a report.
func (c *Checker) Check(ctx context.Context) Report {
	report := Report{
		Healthy:    true,
		Timestamp:  time.Now().UTC(),
		Components: make(map[string]Status),
	}

	if c.pgPool != nil {
		start := time.Now()
		err := c.pgPool.Ping(ctx)
		latency := time.Since(start)
		status := Status{Healthy: err == nil, Latency: latency.String()}
		if err != nil {
			status.Error = err.Error()
			report.Healthy = false
		}
		report.Components["postgres"] = status
	}

	if c.redis != nil {
		start := time.Now()
		err := c.redis.Ping(ctx).Err()
		latency := time.Since(start)
		status := Status{Healthy: err == nil, Latency: latency.String()}
		if err != nil {
			status.Error = err.Error()
			report.Healthy = false
		}
		report.Components["redis"] = status
	}

	if c.nats != nil {
		start := time.Now()
		err := c.checkNATS()
		latency := time.Since(start)
		status := Status{Healthy: err == nil, Latency: latency.String()}
		if err != nil {
			status.Error = err.Error()
			report.Healthy = false
		}
		report.Components["nats"] = status
	}

	return report
}

func (c *Checker) checkNATS() error {
	if c.nats == nil || !c.nats.IsConnected() {
		return fmt.Errorf("not connected")
	}
	return nil
}

// HTTPHandler returns an HTTP handler for the health endpoint.
func (c *Checker) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report := c.Check(r.Context())
		code := http.StatusOK
		if !report.Healthy {
			code = http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(report)
	}
}

