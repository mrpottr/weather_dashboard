package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"weather-forecast-database"
	"weather-forecast-system/internal/handlers"
	"weather-forecast-system/internal/middleware"
)

/*
Package server defines the canonical REST route registry and middleware hierarchy.
Grouping routes by authentication and authorization boundaries guarantees that security rules are evaluated before handler execution.
*/

// RegisterRoutes binds HTTP verbs and URI paths to their respective handlers and authorization tiers.
func RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api")
	{
		api.GET("/health", handlers.HealthHandler)

		// Public endpoints remain unauthenticated to allow initial client handshake and onboarding flows.
		api.POST("/auth/register", handlers.RegisterHandler)
		api.POST("/auth/login", handlers.LoginHandler)

		// Authenticated group applies session token verification to protect private user data and meteorological queries.
		authed := api.Group("", middleware.AuthRequired())
		{
			authed.GET("/auth/me", handlers.MeHandler)
			authed.PUT("/auth/me", handlers.UpdateMeHandler)
			authed.DELETE("/auth/me", handlers.DeleteMeHandler)
			authed.GET("/preferences", handlers.GetPreferencesHandler)
			authed.PUT("/preferences", handlers.PutPreferencesHandler)

			authed.GET("/weather/forecast", handlers.ForecastHandler)
			authed.GET("/weather/geocode", handlers.SearchLocationsHandler)
			authed.GET("/weather/markers", handlers.MapMarkersHandler)
			authed.GET("/meta/locations", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"locations": database.DefaultLocations()})
			})

			authed.GET("/locations", handlers.ListLocationsHandler)
			authed.POST("/locations", handlers.CreateLocationHandler)
			authed.DELETE("/locations/:id", handlers.DeleteLocationHandler)
			authed.GET("/locations/search", handlers.SearchLocationsHandler)

			authed.GET("/intelligence", handlers.IntelligenceHandler)
			authed.GET("/intelligence/activities", handlers.ActivitiesHandler)
			authed.GET("/alerts", handlers.AlertsHandler)
			authed.POST("/alerts/:id/dismiss", handlers.DismissAlertHandler)
			authed.GET("/notifications", handlers.NotificationsHandler)
			authed.GET("/announcements", handlers.PublicAnnouncementsHandler)
			authed.POST("/announcements/:id/dismiss", handlers.DismissAnnouncementHandler)

			authed.GET("/history", handlers.ListHistoryHandler)
			authed.GET("/history/stats", handlers.HistoryStatsHandler)
			authed.GET("/accuracy", handlers.AccuracyHandler)
			authed.GET("/exports/forecast", handlers.ExportForecastHandler)
			authed.GET("/exports/report", handlers.ExportReportHandler)
			authed.GET("/exports/history", handlers.ExportHistoryHandler)

			authed.GET("/images/town", handlers.TownImagesHandler)

			// Admin group restricts access to privileged endpoints via role-based access validation.
			admin := authed.Group("/admin", middleware.AdminRequired())
			{
				admin.GET("/users", handlers.ListUsersHandler)
				admin.GET("/users/export", handlers.ExportUsersHandler)
				admin.PUT("/users/:id/status", handlers.UpdateUserStatusHandler)
				admin.GET("/announcements", handlers.ListAnnouncementsHandler)
				admin.POST("/announcements", handlers.CreateAnnouncementHandler)
				admin.PUT("/announcements/:id", handlers.UpdateAnnouncementHandler)
				admin.GET("/thresholds", handlers.GetThresholdsHandler)
				admin.PUT("/thresholds", handlers.PutThresholdsHandler)
				admin.POST("/refresh", handlers.ManualRefreshHandler)
				admin.GET("/source/check", handlers.SourceCheckHandler)
				admin.GET("/preview", handlers.LivePreviewHandler)
				admin.GET("/logs", handlers.LogsHandler)
				admin.GET("/metrics", handlers.MetricsHandler)
				admin.POST("/login-as", handlers.LoginAsHandler)
			}
		}
	}
	// Fallback routing serves single-page application static assets for non-API frontend URL paths.
	distIndex := filepath.Join("..", "frontend", "dist", "index.html")
	if _, err := os.Stat(distIndex); err == nil {
		r.StaticFile("/", distIndex)
		r.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api") {
				c.JSON(http.StatusNotFound, database.ErrorResponse{Error: "Endpoint not found.", Code: "not_found"})
				return
			}
			c.File(distIndex)
		})
	}
}
