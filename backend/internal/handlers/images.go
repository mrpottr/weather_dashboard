package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"weather-forecast-database"
	"weather-forecast-system/internal/platform"
)

/*
Package handlers serves contextual hero imagery for queried geographic locations.
Integrating persistent database caching reduces external photographic API usage and stabilizes dashboard visual presentation.
*/

// TownImagesHandler resolves rotating scenic photography for queried locations using cached records and fallback media providers.
func TownImagesHandler(c *gin.Context) {
	town := strings.TrimSpace(c.Query("name"))
	if town == "" {
		town = strings.TrimSpace(c.Query("location"))
	}
	if town == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "A town name is required.", Code: "validation"})
		return
	}
	// Stripping provincial state qualifiers yields canonical municipality names for broader photo query hits.
	search := town
	if i := strings.Index(search, ","); i > 0 {
		search = strings.TrimSpace(search[:i])
	}
	cacheKey := "town:" + strings.ToLower(search)
	// Database cache lookups avoid redundant network calls and rate limits on third-party image backends.
	var urlsJSON, provider, credit string
	var fetchedAt string
	if err := database.DB.QueryRow(`SELECT urls, provider, credit, fetched_at FROM images_cache WHERE query = ?`, cacheKey).
		Scan(&urlsJSON, &provider, &credit, &fetchedAt); err == nil {
		if time.Since(database.ParseTime(fetchedAt)) < platform.ImagesCacheTTL() {
			var urls []string
			if json.Unmarshal([]byte(urlsJSON), &urls) == nil && len(urls) > 0 {
				c.JSON(http.StatusOK, TownImages{Town: town, Provider: provider, Images: urls, Cached: true, Credit: credit})
				return
			}
		}
	}
	// Primary photographic lookup queries upstream stock providers configured with API keys.
	images, prov, cred, err := platform.LookupTownImages(c.Request.Context(), search)
	if err != nil || len(images) == 0 {
		// Wikimedia Commons provides zero-configuration fallback photography when commercial provider credentials are missing.
		images, cred, err = platform.WikimediaTownImages(c.Request.Context(), search+" skyline", search)
		prov = "Wikimedia Commons"
		if err != nil || len(images) == 0 {
			// Returning an empty array signals the UI to gracefully maintain CSS ambient gradients rather than displaying broken images.
			c.JSON(http.StatusOK, TownImages{Town: town, Provider: "none", Images: []string{}, Cached: false,
				Credit: "No photo source available for this town."})
			return
		}
	}
	payload, _ := json.Marshal(images)
	_, _ = database.DB.Exec(`INSERT INTO images_cache(query,provider,urls,credit,fetched_at) VALUES(?,?,?,?,?)
		ON CONFLICT(query) DO UPDATE SET provider=excluded.provider, urls=excluded.urls, credit=excluded.credit, fetched_at=excluded.fetched_at`,
		cacheKey, prov, string(payload), cred, time.Now().Format(time.RFC3339))
	c.JSON(http.StatusOK, TownImages{Town: town, Provider: prov, Images: images, Cached: false, Credit: cred})
}
