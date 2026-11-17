package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"weather-forecast-database"

	"weather-forecast-system/internal/weather"
)

/*
Package handlers manages geographic bookmarks, address geocoding, and map marker aggregation.
Centralizing location handling ensures coordinates and timezones remain standardized across personal lists and spatial views.
*/

// ListLocationsHandler fetches bookmarks matching user ownership filters to populate navigation sidebars and quick selectors.
func ListLocationsHandler(c *gin.Context) {
	u, ok := requireAuthCtx(c)
	if !ok {
		return
	}
	q := "%" + strings.ToLower(strings.TrimSpace(c.Query("q"))) + "%"
	rows, err := database.DB.Query(`SELECT id,user_id,name,latitude,longitude,timezone,admin_area,saved_at
		FROM locations WHERE user_id = ? AND (LOWER(name) LIKE ? OR LOWER(admin_area) LIKE ?)
		ORDER BY saved_at DESC`, u.ID, q, q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Could not load saved locations.", Code: "server_error"})
		return
	}
	defer rows.Close()
	out := make([]FavouriteLocation, 0)
	for rows.Next() {
		var l FavouriteLocation
		var saved string
		if err := rows.Scan(&l.ID, &l.UserID, &l.Name, &l.Latitude, &l.Longitude, &l.Timezone, &l.AdminArea, &saved); err != nil {
			continue
		}
		l.SavedAt = database.ParseTime(saved)
		out = append(out, l)
	}
	c.JSON(http.StatusOK, gin.H{"locations": out})
}

// CreateLocationHandler saves user favorite locations, performing fallback geocoding when coordinates are omitted.
func CreateLocationHandler(c *gin.Context) {
	u, ok := requireAuthCtx(c)
	if !ok {
		return
	}
	var body struct {
		Name      string  `json:"name" binding:"required"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Timezone  string  `json:"timezone"`
		AdminArea string  `json:"admin_area"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "A location name and coordinates are required.", Code: "validation"})
		return
	}
	// Fallback geocoding automatically resolves complete coordinate and boundary metadata when the client supplies only a placename.
	if body.Latitude == 0 && body.Longitude == 0 {
		places, err := weather.GeocodeQuery(c.Request.Context(), body.Name)
		if err != nil || len(places) == 0 {
			c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: "That location could not be found inside Australia.", Code: "not_found"})
			return
		}
		p := places[0]
		body.Latitude, body.Longitude = p.Latitude, p.Longitude
		body.Timezone, body.AdminArea = p.Timezone, p.AdminArea
		body.Name = p.Name + weather.StateSuffix(p.AdminArea)
	}
	if body.Timezone == "" {
		body.Timezone = weather.ResolveZone(body.Latitude, body.Longitude)
	}
	res, err := database.DB.Exec(`INSERT OR IGNORE INTO locations(user_id,name,latitude,longitude,timezone,admin_area)
		VALUES(?,?,?,?,?,?)`, u.ID, body.Name, body.Latitude, body.Longitude, body.Timezone, body.AdminArea)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Could not save the location.", Code: "server_error"})
		return
	}
	id, _ := res.LastInsertId()
	database.LogSystemForUser(u, "location.save", "Saved "+body.Name)
	c.JSON(http.StatusCreated, gin.H{"id": id, "name": body.Name, "latitude": body.Latitude, "longitude": body.Longitude,
		"timezone": body.Timezone, "admin_area": body.AdminArea})
}

// DeleteLocationHandler deletes a saved location bookmark while strictly scoping queries by user ID to prevent unauthorized deletions.
func DeleteLocationHandler(c *gin.Context) {
	u, ok := requireAuthCtx(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid location id.", Code: "validation"})
		return
	}
	// Scoping deletion by user_id ensures users cannot mutate bookmarks belonging to other accounts.
	res, err := database.DB.Exec(`DELETE FROM locations WHERE id = ? AND user_id = ?`, id, u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Could not remove the location.", Code: "server_error"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "That saved location was not found.", Code: "not_found"})
		return
	}
	database.LogSystemForUser(u, "location.delete", "Removed location #"+strconv.FormatInt(id, 10))
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

// SearchLocationsHandler proxies forward geocoding requests to external providers and records telemetry for autocomplete optimization.
func SearchLocationsHandler(c *gin.Context) {
	u := currentUser(c)
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(http.StatusOK, gin.H{"results": []GeoPlace{}})
		return
	}
	places, err := weather.GeocodeQuery(c.Request.Context(), q)
	if err != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "Location search is temporarily unavailable: " + err.Error(), Code: "upstream_error"})
		return
	}
	if u != nil {
		_, _ = database.DB.Exec(`INSERT INTO query_history(user_id,query_text,result_count) VALUES(?,?,?)`, u.ID, q, len(places))
	}
	c.JSON(http.StatusOK, gin.H{"results": places})
}

// MapMarkersHandler compiles national observation data across default cities and user pins via concurrent goroutines.
func MapMarkersHandler(c *gin.Context) {
	u := currentUser(c)
	saved := map[string]bool{}
	locs := database.MajorCities()
	if u != nil {
		/*
		Eagerly draining database rows before network requests avoids holding SQLite connection locks during remote HTTP calls.
		*/
		type savedLoc struct {
			Name, Timezone, AdminArea string
			Lat, Lon                  float64
		}
		rowsList := []savedLoc{}
		if rows, err := database.DB.Query(`SELECT name,latitude,longitude,timezone,admin_area FROM locations WHERE user_id = ?`, u.ID); err == nil {
			for rows.Next() {
				var s savedLoc
				if err := rows.Scan(&s.Name, &s.Lat, &s.Lon, &s.Timezone, &s.AdminArea); err == nil {
					rowsList = append(rowsList, s)
				}
			}
			rows.Close()
		}
		for _, s := range rowsList {
			l := LocationRef{Name: s.Name, Latitude: s.Lat, Longitude: s.Lon, Timezone: s.Timezone, AdminArea: s.AdminArea}
			saved[strings.ToLower(l.Name)] = true
			if !containsLocation(locs, l) {
				locs = append(locs, l)
			}
		}
	}
	results, concurrent, sequential := weather.FetchManyConcurrently(c.Request.Context(), locs, false)
	weather.Metrics.SetConcurrency(concurrent.Milliseconds(), sequential, len(locs))

	payload := MarkerPayload{ConcurrentMS: concurrent.Milliseconds(), SequentialMS: sequential, Count: len(results)}
	for _, r := range results {
		m := MarkerLocation{
			Name: r.Loc.Name, AdminArea: r.Loc.AdminArea, Latitude: r.Loc.Latitude,
			Longitude: r.Loc.Longitude, Timezone: r.Loc.Timezone, Saved: saved[strings.ToLower(r.Loc.Name)],
		}
		if r.Forecast != nil {
			m.FromCache = r.Forecast.FromCache
			if r.Forecast.Current != nil {
				temp := r.Forecast.Current.TempC
				cond := r.Forecast.Current.Condition
				_, icon := weather.CodeToCondition(r.Forecast.Current.WeatherCode)
				m.TempC, m.Condition, m.Icon = &temp, &cond, &icon
			}
		}
		payload.Locations = append(payload.Locations, m)
	}
	c.JSON(http.StatusOK, payload)
}

func containsLocation(list []LocationRef, l LocationRef) bool {
	for _, x := range list {
		if strings.EqualFold(x.Name, l.Name) {
			return true
		}
	}
	return false
}
