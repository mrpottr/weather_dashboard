package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

/*
Package platform integrates external media providers to fetch scenic hero photography.
Enforcing string-match heuristics and multi-tier fallbacks prevents irrelevant stock imagery and gracefully degrades to public-domain media.
*/

const imagesCacheTTL = 7 * 24 * time.Hour

// ImagesCacheTTL provides the maximum validity window for cached image URLs to minimize external API consumption.
func ImagesCacheTTL() time.Duration { return imagesCacheTTL }

var httpImages = &http.Client{Timeout: 10 * time.Second}

// LookupTownImages queries commercial photographic providers prioritizing Pexels and Unsplash before falling back.
func LookupTownImages(ctx context.Context, town string) ([]string, string, string, error) {
	if key := os.Getenv("PEXELS_API_KEY"); key != "" {
		if imgs, err := pexelsSearch(ctx, town, key); err == nil && len(imgs) > 0 {
			return imgs, "Pexels", "Photos via Pexels (free licence)", nil
		}
	}
	if key := os.Getenv("UNSPLASH_API_KEY"); key != "" {
		if imgs, err := unsplashSearch(ctx, town, key); err == nil && len(imgs) > 0 {
			return imgs, "Unsplash", "Photos via Unsplash (Unsplash licence)", nil
		}
	}
	return nil, "", "", fmt.Errorf("no photo API key configured")
}

// pexelsSearch executes targeted skyline queries against Pexels, enforcing town-name validation in metadata to discard irrelevant assets.
func pexelsSearch(ctx context.Context, town, key string) ([]string, error) {
	q := url.Values{}
	q.Set("query", town+" city skyline")
	q.Set("per_page", "8")
	q.Set("orientation", "landscape")
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.pexels.com/v1/search?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", key)
	resp, err := httpImages.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var payload struct {
		Photos []struct {
			Alt string `json:"alt"`
			Src struct {
				Large2X string `json:"large2x"`
				Large   string `json:"large"`
			} `json:"src"`
		} `json:"photos"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	out := []string{}
	needle := strings.ToLower(town)
	for _, p := range payload.Photos {
		alt := strings.ToLower(p.Alt)
		// Relevance: town name required in the description when one exists.
		if alt != "" && !strings.Contains(alt, needle) && !strings.Contains(needle, alt) {
			continue
		}
		if p.Src.Large != "" {
			out = append(out, p.Src.Large)
		}
		if len(out) >= 6 {
			break
		}
	}
	return out, nil
}

// unsplashSearch queries the Unsplash API applying case-insensitive placename substring filters to verify visual relevance.
func unsplashSearch(ctx context.Context, town, key string) ([]string, error) {
	q := url.Values{}
	q.Set("query", town+" city")
	q.Set("per_page", "8")
	q.Set("orientation", "landscape")
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.unsplash.com/search/photos?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Client-ID "+key)
	resp, err := httpImages.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var payload struct {
		Results []struct {
			AltDescription string `json:"alt_description"`
			Description    string `json:"description"`
			URLs           struct {
				Regular string `json:"regular"`
			} `json:"urls"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	out := []string{}
	needle := strings.ToLower(town)
	for _, r := range payload.Results {
		text := strings.ToLower(r.AltDescription + " " + r.Description)
		if text != "" && !strings.Contains(text, needle) {
			continue
		}
		if r.URLs.Regular != "" {
			out = append(out, r.URLs.Regular)
		}
		if len(out) >= 6 {
			break
		}
	}
	return out, nil
}

// WikimediaTownImages searches Wikimedia Commons using tiered terms to ensure small townships without dedicated skyline entries still resolve.
func WikimediaTownImages(ctx context.Context, query, fallbackQuery string) ([]string, string, error) {
	tried := []string{}
	for _, q0 := range []string{query, fallbackQuery} {
		if q0 == "" || containsString(tried, q0) {
			continue
		}
		tried = append(tried, q0)
		imgs, err := wikimediaSearch(ctx, q0, query)
		if err == nil && len(imgs) > 0 {
			return imgs, "Photos via Wikimedia Commons (CC BY-SA where applicable)", nil
		}
	}
	return nil, "Photos via Wikimedia Commons (CC BY-SA where applicable)", nil
}

// containsString verifies query membership to avoid dispatching redundant HTTP requests for identical search keys.
func containsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// rawText normalizes heterogeneous JSON types from media metadata fields into clean strings.
func rawText(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if len(s) > 1 && s[0] == '"' {
		var out string
		if json.Unmarshal(raw, &out) == nil {
			return out
		}
	}
	return s
}

// wikimediaSearch queries Wikimedia API and discards non-photographic vector/animation files to guarantee aesthetic consistency.
func wikimediaSearch(ctx context.Context, term, town string) ([]string, error) {
	q := url.Values{}
	q.Set("action", "query")
	q.Set("generator", "search")
	q.Set("gsrsearch", term)
	q.Set("gsrnamespace", "6")
	q.Set("gsrlimit", "10")
	q.Set("prop", "imageinfo")
	q.Set("iiprop", "url|extmetadata")
	q.Set("iiurlwidth", "1920")
	q.Set("format", "json")
	req, err := http.NewRequestWithContext(ctx, "GET", "https://commons.wikimedia.org/w/api.php?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ATMOS-WeatherDashboard/1.0 (education project; data Wikimedia Commons)")
	resp, err := httpImages.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, err
	}
	var payload struct {
		Query struct {
			Pages map[string]struct {
				Title     string `json:"title"`
				ImageInfo []struct {
					ThumbURL string `json:"thumburl"`
					URL      string `json:"url"`
					// extmetadata values are usually strings but can be numbers
					// (e.g. CommonsMetadataExtension), so decode them loosely.
					ExtMeta map[string]struct {
						Value json.RawMessage `json:"value"`
					} `json:"extmetadata"`
				} `json:"imageinfo"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	out := []string{}
	needle := strings.ToLower(town)
	for _, p := range payload.Query.Pages {
		if len(p.ImageInfo) == 0 {
			continue
		}
		info := p.ImageInfo[0]
		text := strings.ToLower(p.Title)
		if desc, ok := info.ExtMeta["ImageDescription"]; ok {
			text += " " + strings.ToLower(rawText(desc.Value))
		}
		if needle != "" && !strings.Contains(text, needle) {
			continue
		}
		u := info.ThumbURL
		if u == "" {
			u = info.URL
		}
		if u == "" {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(u), ".svg") && !strings.HasSuffix(strings.ToLower(u), ".gif") {
			out = append(out, u)
		}
		if len(out) >= 6 {
			break
		}
	}
	return out, nil
}
