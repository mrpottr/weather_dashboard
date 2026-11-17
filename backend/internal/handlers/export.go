package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"weather-forecast-database"
	"weather-forecast-system/internal/supports"
	"weather-forecast-system/internal/weather"
)

/*
Package handlers manages data serialization and downloadable file generation.
Providing structured CSV and JSON outputs facilitates offline analysis, regulatory compliance, and cross-system data exchange.
*/

// ExportForecastHandler transforms forecast payloads into CSV or JSON downloads for external data pipelines.
func ExportForecastHandler(c *gin.Context) {
	u := currentUser(c)
	format := strings.ToLower(c.DefaultQuery("format", "csv"))
	fc, ok := forecastFromQuery(c, false)
	if !ok {
		return
	}
	date := c.Query("date")
	rows := [][]string{{"date", "location", "min_c", "max_c", "humidity_avg", "rain_probability", "rain_mm", "wind_max_kmh", "gust_max_kmh", "uv_index", "pressure_hpa", "cloud_cover", "visibility_km", "sunrise", "sunset", "condition", "source", "licence"}}
	picked := []DailyForecast{}
	for _, d := range fc.Daily {
		if date != "" && d.Date != date {
			continue
		}
		picked = append(picked, d)
		rows = append(rows, []string{
			d.Date, fc.Location.Name,
			fmt.Sprintf("%.1f", d.TempMinC), fmt.Sprintf("%.1f", d.TempMaxC),
			strconv.Itoa(d.HumidityAvg), strconv.Itoa(d.PrecipProbMax),
			fmt.Sprintf("%.1f", d.PrecipSumMM), fmt.Sprintf("%.1f", d.WindMaxKmh),
			fmt.Sprintf("%.1f", d.WindGustMaxKmh), fmt.Sprintf("%.1f", d.UVIndexMax),
			fmt.Sprintf("%.1f", d.PressureAvgHPa), strconv.Itoa(d.CloudCoverAvg),
			fmt.Sprintf("%.1f", d.VisibilityKm), d.Sunrise, d.Sunset,
			d.Condition, fc.Source, fc.Licence,
		})
	}
	database.LogSystemForUser(u, "export.forecast", fmt.Sprintf("Forecast export (%s) for %s", strings.ToUpper(format), fc.Location.Name))
	if format == "json" {
		supports.WriteJSONResponse(c, "atmos-forecast-"+supports.SafeFilePart(fc.Location.Name)+".json", gin.H{
			"location": fc.Location, "timezone": fc.Timezone, "source": fc.Source, "licence": fc.Licence,
			"exported_at": time.Now(), "days": picked,
		})
		return
	}
	supports.WriteCSVResponse(c, "atmos-forecast-"+supports.SafeFilePart(fc.Location.Name)+".csv", rows)
}

// ExportReportHandler aggregates location forecasts, activity suitability, and accuracy metrics into a printable diagnostic report.
func ExportReportHandler(c *gin.Context) {
	u, ok := requireAuthCtx(c)
	if !ok {
		return
	}
	fc, ok2 := forecastFromQuery(c, false)
	if !ok2 {
		return
	}
	date := c.Query("date")
	if date == "" {
		date = time.Now().In(database.LocTime(fc)).Format("2006-01-02")
	}
	in := weather.BuildInputs(fc, date)
	scores := []ActivityScore{}
	for _, p := range weather.ActivityProfiles {
		scores = append(scores, weather.ScoreActivity(in, p))
	}
	for i := 0; i < len(scores); i++ {
		for j := i + 1; j < len(scores); j++ {
			if scores[j].Score > scores[i].Score {
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}
	th := database.GetThresholds()
	report := gin.H{
		"title":          "ATMOS weather report — " + fc.Location.Name + " — " + date,
		"location":       fc.Location,
		"date":           date,
		"timezone":       fc.Timezone,
		"generated":      time.Now(),
		"generated_by":   u.Email,
		"source":         fc.Source,
		"licence":        fc.Licence,
		"from_cache":     fc.FromCache,
		"daily":          pickDaily(fc, date),
		"current":        fc.Current,
		"top_activities": topScores(scores, 3),
		"thresholds":     th,
		"accuracy":       accuracyFor(database.CacheKey(fc.Location)),
		"note":           "All values are taken directly from the Australian Bureau of Meteorology feed (ACCESS-G) retrieved via Open-Meteo. Non-commercial educational use.",
	}
	database.LogSystemForUser(u, "export.report", "Report snapshot for "+fc.Location.Name+" on "+date)
	supports.WriteJSONResponse(c, "atmos-report-"+supports.SafeFilePart(fc.Location.Name)+"-"+date+".json", report)
}

// ExportHistoryHandler serializes user lookup histories to CSV or JSON formats for personal query auditing.
func ExportHistoryHandler(c *gin.Context) {
	u, ok := requireAuthCtx(c)
	if !ok {
		return
	}
	format := strings.ToLower(c.DefaultQuery("format", "csv"))
	if format == "json" {
		entries := []gin.H{}
		rows, err := database.DB.Query(`SELECT id,location_name,date,source,result,ts FROM forecast_history WHERE user_id = ? ORDER BY id DESC LIMIT 500`, u.ID)
		if err == nil {
			for rows.Next() {
				var id int64
				var loc, date, source, result, ts string
				if rows.Scan(&id, &loc, &date, &source, &result, &ts) == nil {
					entries = append(entries, gin.H{"id": id, "location": loc, "date": date, "source": source, "result": result, "ts": ts})
				}
			}
			rows.Close()
		}
		database.LogSystemForUser(u, "export.history", "History export (JSON)")
		supports.WriteJSONResponse(c, "atmos-history.json", gin.H{"user": u.Email, "exported_at": time.Now(), "entries": entries})
		return
	}
	rows := [][]string{{"timestamp", "location", "date", "result", "source"}}
	if r, err := database.DB.Query(`SELECT location_name,date,source,result,ts FROM forecast_history WHERE user_id = ? ORDER BY id DESC LIMIT 500`, u.ID); err == nil {
		for r.Next() {
			var loc, date, source, result, ts string
			if r.Scan(&loc, &date, &source, &result, &ts) == nil {
				rows = append(rows, []string{ts, loc, date, result, source})
			}
		}
		r.Close()
	}
	database.LogSystemForUser(u, "export.history", "History export (CSV)")
	supports.WriteCSVResponse(c, "atmos-history.csv", rows)
}

// ExportUsersHandler generates an administrative export of user accounts for access reviews and directory backups.
func ExportUsersHandler(c *gin.Context) {
	u, ok := requireAuthCtx(c)
	if !ok {
		return
	}
	if u.Role != RoleAdmin {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "Access denied: this function is reserved for administrator accounts.", Code: "access_denied"})
		return
	}
	rows := [][]string{{"id", "first_name", "last_name", "email", "role", "status", "default_location", "created_at", "last_active_at"}}
	r, err := database.DB.Query(`SELECT id,first_name,last_name,email,role,status,default_location,created_at,COALESCE(last_active_at,'') FROM users ORDER BY id`)
	if err == nil {
		for r.Next() {
			var id int64
			var first, last, email, role, status, defLoc, created, lastActive string
			if r.Scan(&id, &first, &last, &email, &role, &status, &defLoc, &created, &lastActive) == nil {
				rows = append(rows, []string{
					strconv.FormatInt(id, 10), first, last, email, role, status, defLoc, created, lastActive,
				})
			}
		}
		r.Close()
	}
	database.LogSystemForUser(u, "export.users", "User list exported as CSV")
	supports.WriteCSVResponse(c, "atmos-users.csv", rows)
}

// ---------------------------------------------------------------------------
// Small helpers shared by the export functions
// ---------------------------------------------------------------------------

func pickDaily(fc *ForecastResponse, date string) *DailyForecast {
	for i := range fc.Daily {
		if fc.Daily[i].Date == date {
			return &fc.Daily[i]
		}
	}
	if len(fc.Daily) > 0 {
		return &fc.Daily[0]
	}
	return nil
}

func topScores(scores []ActivityScore, n int) []ActivityScore {
	if len(scores) > n {
		return scores[:n]
	}
	return scores
}

func accuracyFor(locationKey string) gin.H {
	out := gin.H{"samples": 0}
	if database.DB == nil {
		return out
	}
	var overall float64
	var count int
	err := database.DB.QueryRow(`SELECT AVG(percent), COUNT(*) FROM accuracy_samples WHERE location_key = ?`, locationKey).Scan(&overall, &count)
	if err == nil && count > 0 {
		out["samples"] = count
		out["overall_pct"] = weather.Round1(overall)
	}
	return out
}
