package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()

	router.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"message": "Weather backend is running",
		})
	})

	router.GET("/api/forecast", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"location":          "Sydney, NSW, Australia",
			"timezone":          "Australia/Sydney",
			"data_source":       "sample-development-data",
			"is_sample_data":    true,
			"forecast_summary":  "Sample weather data for Sprint 1 development",
			"forecast": []gin.H{
				{
					"date":          "2026-08-30",
					"day":           "Monday",
					"summary":       "Sunny and warm",
					"condition":     "clear",
					"temperature_c": 22,
					"feels_like_c":  24,
					"humidity_pct":  55,
					"wind_kph":      12,
				},
				{
					"date":          "2026-08-31",
					"day":           "Tuesday",
					"summary":       "Partly cloudy",
					"condition":     "partly_cloudy",
					"temperature_c": 20,
					"feels_like_c":  21,
					"humidity_pct":  60,
					"wind_kph":      10,
				},
				{
					"date":          "2026-09-01",
					"day":           "Wednesday",
					"summary":       "Chance of light rain",
					"condition":     "light_rain",
					"temperature_c": 18,
					"feels_like_c":  17,
					"humidity_pct":  70,
					"wind_kph":      14,
				},
			},
		})
	})

	if err := router.Run(":8080"); err != nil {
		panic(err)
	}
}
