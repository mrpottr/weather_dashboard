package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"weather-forecast-database"

	"weather-forecast-system/internal/weather"
)

/*
Package handlers exposes the primary meteorological data retrieval and system diagnostic endpoints.
Centralizing these handlers coordinates upstream caching, audit trails, and real-time accuracy scoring.
*/

// ForecastHandler handles forecast queries while orchestrating cache evaluation, accuracy telemetry, and query logging.
// Running accuracy comparisons exclusively on fresh payloads prevents comparing identical records against themselves.
func ForecastHandler(c *gin.Context) {
	u := currentUser(c)
	fc, ok := forecastFromQuery(c, false)
	if !ok {
		return
	}
	date := c.Query("date")
	if !fc.FromCache {
		weather.RecordAccuracy(fc.Location, fc)
	}
	database.RecordForecastHistory(u, fc, date)
	database.LogSystemForUser(u, "forecast.fetch", fmt.Sprintf("%s (%.1f ms)%s", fc.Location.Name, float64(fc.LatencyMS),
		map[bool]string{true: " [cached]", false: ""}[fc.FromCache]))
	c.JSON(http.StatusOK, fc)
}

// HealthHandler surfaces runtime vitals and upstream availability metrics for infrastructure monitoring and cluster health checks.
func HealthHandler(c *gin.Context) {
	avg, success, _, n := weather.Metrics.Snapshot()
	c.JSON(http.StatusOK, gin.H{
		"status":          "ok",
		"service":         "atmos-backend",
		"database":        "sqlite",
		"source":          weather.SourceName,
		"licence":         weather.SourceLicence,
		"avg_latency_ms":  int64(avg),
		"api_success_pct": weather.Round1(success),
		"measurements":    n,
		"server_time":     time.Now(),
	})
}
