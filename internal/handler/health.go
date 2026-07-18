package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthHandler serves liveness and readiness probes for load balancers
// and DigitalOcean App Platform health checks.
type HealthHandler struct {
	appEnv    string
	startedAt time.Time
}

// NewHealthHandler constructs a health handler with the current app environment.
func NewHealthHandler(appEnv string) *HealthHandler {
	return &HealthHandler{
		appEnv:    appEnv,
		startedAt: time.Now().UTC(),
	}
}

// healthResponse is the JSON body returned by health endpoints.
type healthResponse struct {
	Health    string `json:"health"` // "healthy" when the service is working
	Status    string `json:"status"`
	AppEnv    string `json:"app_env,omitempty"`
	UptimeSec int64  `json:"uptime_seconds"`
}

// Live handles GET /healthz and GET /health.
// Used as a liveness probe: process is up and serving HTTP.
func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, healthResponse{
		Health:    "healthy",
		Status:    "ok",
		AppEnv:    h.appEnv,
		UptimeSec: int64(time.Since(h.startedAt).Seconds()),
	})
}

// Ready handles GET /ready.
// Used as a readiness probe: safe to receive traffic.
func (h *HealthHandler) Ready(c *gin.Context) {
	c.JSON(http.StatusOK, healthResponse{
		Health:    "healthy",
		Status:    "ready",
		AppEnv:    h.appEnv,
		UptimeSec: int64(time.Since(h.startedAt).Seconds()),
	})
}
