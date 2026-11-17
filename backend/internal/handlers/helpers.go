package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"weather-forecast-database"

	"weather-forecast-system/internal/middleware"
	"weather-forecast-system/internal/weather"
)

/*
Package handlers provides shared request extraction, authentication verification, and coordinate normalization routines.
Abstracting these utilities ensures uniform parameter validation and centralized upstream error mapping across all route handlers.
*/

// currentUser extracts the authenticated identity injected by upstream security middleware.
func currentUser(c *gin.Context) *User {
	return middleware.CurrentUser(c)
}

// requireAuthCtx enforces session authentication, halting execution with standardized unauthorized payloads when absent.
func requireAuthCtx(c *gin.Context) (*User, bool) {
	u := currentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Sign in required.", Code: "unauthorized"})
		return nil, false
	}
	return u, true
}

// parseIDParam extracts and validates numeric resource identifiers from route URI parameters.
func parseIDParam(c *gin.Context, name string) (int64, error) {
	var id int64
	_, err := fmt.Sscanf(c.Param(name), "%d", &id)
	return id, err
}

func parseFloat(s string) (float64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

// userLocations retrieves persisted favorites to support personalized alerts and dashboard summaries.
func userLocations(u *User) ([]LocationRef, error) {
	rows, err := database.DB.Query(`SELECT name,latitude,longitude,timezone,admin_area FROM locations WHERE user_id = ? ORDER BY saved_at DESC`, u.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LocationRef{}
	for rows.Next() {
		var l LocationRef
		if err := rows.Scan(&l.Name, &l.Latitude, &l.Longitude, &l.Timezone, &l.AdminArea); err == nil {
			out = append(out, l)
		}
	}
	return out, nil
}

// forecastFromQuery parses location coordinates and retrieves meteorological datasets with automatic timezone resolution.
func forecastFromQuery(c *gin.Context, allowMissingCoords bool) (*ForecastResponse, bool) {
	lat, err1 := parseFloat(c.Query("lat"))
	lon, err2 := parseFloat(c.Query("lon"))
	name := strings.TrimSpace(c.Query("name"))
	if name == "" {
		name = strings.TrimSpace(c.Query("location"))
	}
	if err1 != nil || err2 != nil || (lat == 0 && lon == 0) {
		if !allowMissingCoords {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "A valid Australian location (lat/lon) is required.", Code: "validation"})
			return nil, false
		}
		m := database.MatchDefaultLocation("Melbourne, VIC")
		lat, lon, name = m.Latitude, m.Longitude, m.Name
	}
	loc := LocationRef{Name: name, Latitude: lat, Longitude: lon, Timezone: c.Query("timezone")}
	if loc.Name == "" {
		loc.Name = fmt.Sprintf("%.2f, %.2f", lat, lon)
	}
	if loc.Timezone == "" {
		loc.Timezone = weather.ResolveZone(lat, lon)
	}
	if loc.AdminArea == "" {
		loc.AdminArea = weather.ZoneAdminArea(loc.Timezone)
	}
	refresh := c.Query("refresh") == "1"
	fc, err := weather.FetchForecast(c.Request.Context(), loc, refresh)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: err.Error(), Code: "weather_unavailable"})
		return nil, false
	}
	return fc, true
}
