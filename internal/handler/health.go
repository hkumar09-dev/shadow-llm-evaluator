package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthHandler serves liveness and readiness probes.
type HealthHandler struct {
	appCtx    context.Context
	appEnv    string
	startedAt time.Time
}

// NewHealthHandler constructs a health handler with the root app context.
func NewHealthHandler(appCtx context.Context, appEnv string) *HealthHandler {
	if appCtx == nil {
		appCtx = context.Background()
	}
	return &HealthHandler{
		appCtx:    appCtx,
		appEnv:    appEnv,
		startedAt: time.Now().UTC(),
	}
}

type healthResponse struct {
	Health    string `json:"health"`
	Status    string `json:"status"`
	AppEnv    string `json:"app_env,omitempty"`
	UptimeSec int64  `json:"uptime_seconds"`
}

// Live handles GET /healthz and GET /health.
func (h *HealthHandler) Live(c *gin.Context) {
	ctx := c.Request.Context()
	if err := ctx.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, healthResponse{Health: "unhealthy", Status: "canceled"})
		return
	}
	if err := h.appCtx.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, healthResponse{Health: "unhealthy", Status: "shutting_down"})
		return
	}

	c.JSON(http.StatusOK, healthResponse{
		Health:    "healthy",
		Status:    "ok",
		AppEnv:    h.appEnv,
		UptimeSec: int64(time.Since(h.startedAt).Seconds()),
	})
}

// Ready handles GET /ready.
func (h *HealthHandler) Ready(c *gin.Context) {
	ctx := c.Request.Context()
	if err := ctx.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, healthResponse{Health: "unhealthy", Status: "canceled"})
		return
	}
	if err := h.appCtx.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, healthResponse{Health: "unhealthy", Status: "shutting_down"})
		return
	}

	c.JSON(http.StatusOK, healthResponse{
		Health:    "healthy",
		Status:    "ready",
		AppEnv:    h.appEnv,
		UptimeSec: int64(time.Since(h.startedAt).Seconds()),
	})
}
