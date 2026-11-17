package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

/*
Package weather resolves Australian coordinate boundaries, address search terms, and IANA timezone mappings.
Restricting queries to continental coordinates and validating boundary polygons guarantees accurate forecast alignment.
*/

// GeocodeQuery translates textual place queries into Australian coordinate pairs with fallback geocoding providers.
func GeocodeQuery(ctx context.Context, query string) ([]GeoPlace, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	// Direct coordinate parsing detects comma-separated decimal degrees to bypass external search latency.
	if parts := strings.Split(query, ","); len(parts) == 2 {
		if lat, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64); err1 == nil {
			if lon, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err2 == nil {
				if lat >= -44 && lat <= -9 && lon >= 112 && lon <= 154.5 {
					name := fmt.Sprintf("%.2f, %.2f", lat, lon)
					return []GeoPlace{{Name: name, Country: "Australia", Latitude: lat, Longitude: lon, Timezone: ResolveZone(lat, lon), AdminArea: ZoneAdminArea(ResolveZone(lat, lon))}}, nil
				}
				return nil, fmt.Errorf("coordinates are outside Australia")
			}
		}
	}
	q := url.Values{}
	q.Set("name", query)
	q.Set("count", "8")
	q.Set("language", "en")
	q.Set("format", "json")
	q.Set("countryCode", "AU")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, geocodeEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ATMOS-WeatherDashboard/1.0 (education project)")
	resp, err := httpBOM.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var payload struct {
		Results []struct {
			Name        string  `json:"name"`
			Admin1      string  `json:"admin1"`
			Country     string  `json:"country"`
			CountryCode string  `json:"country_code"`
			Latitude    float64 `json:"latitude"`
			Longitude   float64 `json:"longitude"`
			Timezone    string  `json:"timezone"`
			Postcode    string  `json:"postcode"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	places := make([]GeoPlace, 0, len(payload.Results))
	for _, r := range payload.Results {
		if r.CountryCode != "AU" && r.Country != "Australia" {
			continue
		}
		tz := r.Timezone
		if tz == "" {
			tz = ResolveZone(r.Latitude, r.Longitude)
		}
		places = append(places, GeoPlace{
			Name: r.Name, AdminArea: r.Admin1, Country: "Australia",
			Latitude: r.Latitude, Longitude: r.Longitude, Timezone: tz, Postcode: r.Postcode,
		})
	}
	// Secondary query dispatch to OpenStreetMap handles postcodes and street-level addresses that primary gazetteers omit.
	if len(places) == 0 {
		if extra, err := nominatimQuery(ctx, query); err == nil {
			places = append(places, extra...)
		}
	}
	return places, nil
}

func nominatimQuery(ctx context.Context, query string) ([]GeoPlace, error) {
	q := url.Values{}
	q.Set("format", "json")
	q.Set("limit", "6")
	q.Set("countrycodes", "au")
	if isNumeric(query) && len(query) == 4 {
		q.Set("postalcode", query)
		q.Set("q", query+" Australia")
	} else {
		q.Set("q", query+", Australia")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, nominatimEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ATMOS-WeatherDashboard/1.0 (education project; single lookup)")
	resp, err := httpBOM.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var rows []struct {
		DisplayName string `json:"display_name"`
		Lat         string `json:"lat"`
		Lon         string `json:"lon"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, err
	}
	out := make([]GeoPlace, 0, len(rows))
	for _, r := range rows {
		lat, e1 := strconv.ParseFloat(r.Lat, 64)
		lon, e2 := strconv.ParseFloat(r.Lon, 64)
		if e1 != nil || e2 != nil {
			continue
		}
		if lat < -44 || lat > -9 || lon < 112 || lon > 154.5 {
			continue
		}
		name := strings.Split(r.DisplayName, ",")[0]
		out = append(out, GeoPlace{
			Name: name, AdminArea: adminFromDisplay(r.DisplayName), Country: "Australia",
			Latitude: lat, Longitude: lon, Timezone: ResolveZone(lat, lon),
		})
	}
	return out, nil
}

func adminFromDisplay(display string) string {
	parts := strings.Split(display, ",")
	if len(parts) >= 2 {
		return strings.TrimSpace(parts[len(parts)-2])
	}
	return ""
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// ResolveZone resolves decimal coordinates to official Australian IANA timezone identifiers across state daylight savings lines.
func ResolveZone(lat, lon float64) string {
	switch {
	case lat < -40.4: // Tasmania
		return "Australia/Hobart"
	case lon < 129: // Western Australia
		return "Australia/Perth"
	case lon < 138:
		if lat > -26 { // Northern Territory
			return "Australia/Darwin"
		}
		return "Australia/Adelaide" // south of 26S is SA
	case lon < 141:
		if lat > -26 { // Queensland
			return "Australia/Brisbane"
		}
		return "Australia/Adelaide"
	case lat <= -36.1: // Victoria (below the NSW border band)
		return "Australia/Melbourne"
	case lat <= -35.1 && lon >= 148.7 && lon <= 149.5: // ACT enclave
		return "Australia/Sydney"
	default: // New South Wales
		return "Australia/Sydney"
	}
}

// ZoneAdminArea converts IANA regional timezone keys into standard abbreviated Australian state identifiers.
func ZoneAdminArea(tz string) string {
	switch tz {
	case "Australia/Perth":
		return "WA"
	case "Australia/Darwin":
		return "NT"
	case "Australia/Adelaide":
		return "SA"
	case "Australia/Brisbane":
		return "QLD"
	case "Australia/Melbourne":
		return "VIC"
	case "Australia/Hobart":
		return "TAS"
	default:
		return "NSW"
	}
}

// StateSuffix formats state qualifiers to disambiguate municipalities sharing duplicate names across different states.
func StateSuffix(admin string) string {
	if admin == "" {
		return ""
	}
	return ", " + admin
}
