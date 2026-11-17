package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"weather-forecast-database"

	"weather-forecast-system/internal/weather"
)

/*
Package handlers manages hazardous weather detection and notifications against administrative thresholds.
Evaluating live forecasts against user thresholds delivers preemptive hazard advisories with actionable safety guidance.
*/

// windThreshold maps qualitative Beaufort threat bands to quantitative velocity limits for deterministic rule triggering.
func windThreshold(level string) (float64, string) {
	switch strings.ToLower(level) {
	case "gale":
		return 62, "gale-force wind (>=62 km/h)"
	case "storm":
		return 89, "storm-force wind (>=89 km/h)"
	default:
		return 40, "strong wind (>=40 km/h)"
	}
}

// generateWeatherAlerts evaluates multi-day forecasts against safety thresholds to generate actionable hazard notifications.
func generateWeatherAlerts(c *gin.Context, u *User, locs []LocationRef) []WeatherAlert {
	th := database.GetThresholds()
	if !th.PushEnabled {
		return []WeatherAlert{}
	}
	results, _, _ := weather.FetchManyConcurrently(c.Request.Context(), locs, false)
	out := []WeatherAlert{}
	for _, r := range results {
		if r.Forecast == nil {
			continue
		}
		fc := r.Forecast
		now := time.Now().In(database.LocTime(fc))
		for i, d := range fc.Daily {
			if i > 2 { // Capping evaluations at a 72-hour horizon ensures alerts focus on immediate, high-confidence planning windows.
				break
			}
			dateLabel := d.Date
			if i == 0 {
				dateLabel += " (today)"
			} else if i == 1 {
				dateLabel += " (tomorrow)"
			}
			add := func(typ, severity, title, trigger, actual, threshold, precautions string) {
				out = append(out, WeatherAlert{
					Type: typ, Severity: severity, Title: title, Location: fc.Location.Name,
					Message:     trigger + " on " + dateLabel + " in " + fc.Location.Name + ".",
					Trigger:     trigger,
					ActualValue: actual, ThresholdValue: threshold,
					Window:      dateLabel + " " + d.Sunrise + "-" + d.Sunset + " local",
					Precautions: precautions, CreatedAt: now,
				})
			}
			if d.TempMaxC >= th.HighTempC {
				sev := "high"
				if d.TempMaxC >= th.HighTempC+8 {
					sev = "extreme"
				}
				add("high_temperature", sev, "High temperature alert",
					fmt.Sprintf("Maximum temperature %.0f°C reaches the configured high-temperature threshold", d.TempMaxC),
					fmt.Sprintf("%.0f°C maximum", d.TempMaxC),
					fmt.Sprintf("%.0f°C high-temperature threshold", th.HighTempC),
					"Drink water before feeling thirsty, avoid strenuous activity 11:00-15:00, never leave people or pets in parked cars, and check on vulnerable neighbours.")
			}
			if d.TempMinC <= th.LowTempC {
				add("low_temperature", "medium", "Low temperature alert",
					fmt.Sprintf("Minimum temperature %.0f°C reaches the configured low-temperature threshold", d.TempMinC),
					fmt.Sprintf("%.0f°C minimum", d.TempMinC),
					fmt.Sprintf("%.0f°C low-temperature threshold", th.LowTempC),
					"Wrap up in layers, protect plants and pets, allow extra travel time for possible frost or ice, and watch for cold-related discomfort.")
			}
			if d.PrecipProbMax >= th.RainPct || d.PrecipSumMM >= 20 {
				sev := "medium"
				if d.PrecipProbMax >= 90 || d.PrecipSumMM >= 40 {
					sev = "high"
				}
				add("heavy_rain", sev, "Heavy rain alert",
					fmt.Sprintf("Rain probability %d%% (threshold %d%%) with %.1f mm expected", d.PrecipProbMax, th.RainPct, d.PrecipSumMM),
					fmt.Sprintf("%d%% probability, %.1f mm", d.PrecipProbMax, d.PrecipSumMM),
					fmt.Sprintf("%d%% rain threshold", th.RainPct),
					"Carry waterproofs, allow extra travel time, avoid flooded roads and drains, and secure outdoor furniture.")
			}
			if w, label := windThreshold(th.WindLevel); d.WindMaxKmh >= w {
				sev := "medium"
				if th.WindLevel == "gale" || th.WindLevel == "storm" || d.WindGustMaxKmh >= w+25 {
					sev = "high"
				}
				if d.WindGustMaxKmh >= 100 {
					sev = "extreme"
				}
				add("strong_wind", sev, "Wind alert",
					fmt.Sprintf("Wind %.0f km/h (gusts %.0f km/h) reaches the configured %s", d.WindMaxKmh, d.WindGustMaxKmh, label),
					fmt.Sprintf("%.0f km/h sustained, %.0f km/h gusts", d.WindMaxKmh, d.WindGustMaxKmh),
					label,
					"Secure loose items, avoid exposed coastlines and ridgelines, delay boating and tree work, and expect possible power interruptions.")
			}
			if d.WeatherCode >= 95 {
				add("thunderstorm", "high", "Thunderstorm alert",
					"Forecast weather codes indicate thunderstorms (95/96/99)",
					codeLabel(d.WeatherCode),
					"Any thunderstorm code in the forecast",
					"Move to a substantial building or hard-topped vehicle before lightning starts, avoid tall trees and water, and unplug sensitive equipment.")
			}
			if d.UVIndexMax >= th.UVIndex {
				sev := "medium"
				if d.UVIndexMax >= 11 {
					sev = "high"
				}
				cat, _ := weather.UVCategory(d.UVIndexMax)
				add("extreme_uv", sev, "High UV alert",
					fmt.Sprintf("UV index %.0f reaches the configured %s threshold (%s category)", d.UVIndexMax, uvCatShort(th.UVIndex), cat),
					fmt.Sprintf("UV %.0f (%s)", d.UVIndexMax, cat),
					fmt.Sprintf("UV %.0f threshold", th.UVIndex),
					"Use SPF50+ sunscreen, a broad-brim hat and sunglasses; seek shade 10:00-16:00; consider indoor alternatives at peak UV.")
			}
		}
	}
	/*
	Persisting generated alerts preserves user dismissal states across page reloads.
	The upsert key ensures duplicate records are updated in place rather than creating redundant notices.
	*/
	for i := range out {
		a := &out[i]
		var id int64
		var dismissed int
		day := strings.SplitN(a.Window, " ", 2)[0]
		err := database.DB.QueryRow(`INSERT INTO weather_alerts(user_id,location_name,alert_type,date)
			VALUES(?,?,?,?) ON CONFLICT(user_id,location_name,alert_type,date) DO UPDATE SET created_at=CURRENT_TIMESTAMP
			RETURNING id, dismissed`, u.ID, a.Location, a.Type, day).Scan(&id, &dismissed)
		if err != nil {
			// Backward compatibility query accommodates SQLite drivers that do not support RETURNING syntax.
			_ = database.DB.QueryRow(`SELECT id, dismissed FROM weather_alerts WHERE user_id=? AND location_name=? AND alert_type=? AND date=?`,
				u.ID, a.Location, a.Type, day).Scan(&id, &dismissed)
		}
		a.ID = id
		a.Dismissed = dismissed == 1
	}
	return out
}

func codeLabel(code int) string {
	label, _ := weather.CodeToCondition(code)
	return label
}

func uvCatShort(uv float64) string {
	cat, _ := weather.UVCategory(uv)
	return cat
}

// AlertsHandler provides sorted hazard notifications prioritized by severity to streamline risk mitigation for users.
func AlertsHandler(c *gin.Context) {
	u, ok := requireAuthCtx(c)
	if !ok {
		return
	}
	locs, err := userLocations(u)
	if err != nil || len(locs) == 0 {
		locs = []LocationRef{database.MatchDefaultLocation(u.DefaultLocation)}
		if locs[0].Name == "" {
			locs = []LocationRef{database.Melbourne}
		}
	}
	if lat, err1 := parseFloat(c.Query("lat")); err1 == nil && lat != 0 {
		if lon, err2 := parseFloat(c.Query("lon")); err2 == nil && lon != 0 {
			locs = []LocationRef{{
				Name: c.Query("name"), Latitude: lat, Longitude: lon,
				Timezone: c.Query("timezone"), AdminArea: c.Query("admin_area"),
			}}
			if locs[0].Name == "" {
				locs[0].Name = fmt.Sprintf("%.2f, %.2f", lat, lon)
			}
			if locs[0].Timezone == "" {
				locs[0].Timezone = weather.ResolveZone(lat, lon)
			}
		}
	}
	alerts := generateWeatherAlerts(c, u, locs)
	includeDismissed := c.Query("include_dismissed") == "1"
	filtered := make([]WeatherAlert, 0, len(alerts))
	for _, a := range alerts {
		if a.Dismissed && !includeDismissed {
			continue
		}
		filtered = append(filtered, a)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		rank := map[string]int{"extreme": 0, "high": 1, "medium": 2}
		if rank[filtered[i].Severity] != rank[filtered[j].Severity] {
			return rank[filtered[i].Severity] < rank[filtered[j].Severity]
		}
		return filtered[i].Location < filtered[j].Location
	})
	c.JSON(http.StatusOK, gin.H{"alerts": filtered, "generated_at": time.Now(), "source": weather.SourceName})
}

// DismissAlertHandler acknowledges an active alert to prevent duplicate notifications from disrupting the user.
func DismissAlertHandler(c *gin.Context) {
	u, ok := requireAuthCtx(c)
	if !ok {
		return
	}
	id, err := parseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid alert id.", Code: "validation"})
		return
	}
	res, err := database.DB.Exec(`UPDATE weather_alerts SET dismissed = 1 WHERE id = ? AND user_id = ?`, id, u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Could not dismiss the alert.", Code: "server_error"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Alert not found.", Code: "not_found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"dismissed": id})
}

// NotificationsHandler calculates unread badge counts across weather and system alerts to drive notification badges.
func NotificationsHandler(c *gin.Context) {
	u, ok := requireAuthCtx(c)
	if !ok {
		return
	}
	weatherCount := 0
	if locs, err := userLocations(u); err == nil && len(locs) > 0 {
		for _, a := range generateWeatherAlerts(c, u, locs) {
			if !a.Dismissed {
				weatherCount++
			}
		}
	}
	system := 0
	_ = database.DB.QueryRow(`SELECT COUNT(*) FROM announcements a
		WHERE a.active = 1
		AND (a.expires_at IS NULL OR a.expires_at > ?)
		AND NOT EXISTS (SELECT 1 FROM announcement_dismissals d WHERE d.announcement_id = a.id AND d.user_id = ?)`,
		time.Now().Format(time.RFC3339), u.ID).Scan(&system)
	c.JSON(http.StatusOK, NotificationSummary{Unread: weatherCount + system, WeatherCount: weatherCount, SystemCount: system})
}
