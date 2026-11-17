package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"weather-forecast-database"

	"weather-forecast-system/internal/weather"
)

/*
Package handlers exposes suitability scores and historical forecast verification metrics.
Serving precomputed assessments ensures deterministic, auditable decisions without client-side calculation drift.
*/

// IntelligenceHandler computes outdoor suitability ratings against live weather data for user-selected temporal windows.
func IntelligenceHandler(c *gin.Context) {
	fc, ok := forecastFromQuery(c, false)
	if !ok {
		return
	}
	date := c.Query("date")
	if date == "" {
		date = time.Now().In(database.LocTime(fc)).Format("2006-01-02")
	}
	in := weather.BuildInputs(fc, date)
	only := strings.TrimSpace(c.Query("activity"))
	resp := IntelligenceResponse{Location: in.Location, Date: date, Activities: []ActivityScore{}}
	for _, p := range weather.ActivityProfiles {
		if only != "" && only != p.Key {
			continue
		}
		resp.Activities = append(resp.Activities, weather.ScoreActivity(in, p))
	}
	c.JSON(http.StatusOK, resp)
}

// ActivitiesHandler exposes all supported activity taxonomies to drive interactive frontend category pickers.
func ActivitiesHandler(c *gin.Context) {
	list := make([]ActivityDef, 0, len(weather.ActivityProfiles))
	for _, p := range weather.ActivityProfiles {
		list = append(list, ActivityDef{Key: p.Key, Name: p.Name, Emoji: p.Emoji})
	}
	c.JSON(http.StatusOK, gin.H{"activities": list})
}

// AccuracyHandler aggregates historical forecast variances against actual observations to establish model reliability metrics.
func AccuracyHandler(c *gin.Context) {
	lat, _ := parseFloat(c.Query("lat"))
	lon, _ := parseFloat(c.Query("lon"))
	name := c.Query("name")
	key := ""
	if lat != 0 || lon != 0 {
		key = fmt.Sprintf("%.3f|%.3f", lat, lon)
	}
	report := AccuracyReport{Location: name, Metrics: map[string]float64{}, Recent: []AccuracySample{}}
	query := `SELECT id,location_name,date,metric,forecast,observed,percent,ts FROM accuracy_samples`
	args := []interface{}{}
	if key != "" {
		query += ` WHERE location_key = ?`
		args = append(args, key)
	}
	query += ` ORDER BY id DESC LIMIT 200`
	rows, err := database.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Accuracy data could not be read.", Code: "server_error"})
		return
	}
	defer rows.Close()
	sums := map[string]float64{}
	counts := map[string]int{}
	total, totalN := 0.0, 0
	for rows.Next() {
		var s AccuracySample
		var ts string
		if err := rows.Scan(&s.ID, &s.Location, &s.Date, &s.Metric, &s.Forecast, &s.Observed, &s.Percent, &ts); err != nil {
			continue
		}
		s.TS = database.ParseTime(ts)
		if report.Location == "" {
			report.Location = s.Location
		}
		sums[s.Metric] += s.Percent
		counts[s.Metric]++
		total += s.Percent
		totalN++
		if len(report.Recent) < 12 {
			report.Recent = append(report.Recent, s)
		}
	}
	for m, s := range sums {
		report.Metrics[m] = weather.Round1(s / float64(counts[m]))
	}
	report.Samples = totalN
	if totalN > 0 {
		report.Overall = weather.Round1(total / float64(totalN))
	}
	c.JSON(http.StatusOK, report)
}
