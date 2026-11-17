// Package weather retrieves, normalizes, and caches Australian meteorological forecasts.
// It queries the Open-Meteo ACCESS-G feed with automated fallback to global models and persists responses across memory and SQLite.
package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"weather-forecast-database"
)

const (
	BOMEndpoint       = "https://api.open-meteo.com/v1/bom"
	NodeEndpoint      = "https://api.open-meteo.com/v1/forecast"
	geocodeEndpoint   = "https://geocoding-api.open-meteo.com/v1/search"
	nominatimEndpoint = "https://nominatim.openstreetmap.org/search"
	cacheTTL          = 10 * time.Minute
	upstreamTimeout   = 12 * time.Second
	// SourceName attributes the primary Australian ACCESS-G numerical model feed.
	SourceName = "BOM / ACCESS-G via Open-Meteo"
	// SourceLicence provides mandatory attribution for upstream redistribution compliance.
	SourceLicence = "Non-commercial data: Australian Bureau of Meteorology (ACCESS-G) via Open-Meteo. Open-Meteo CC BY 4.0."
	// fallbackSourceName identifies blended models used when ACCESS-G experiences upstream outages.
	fallbackSourceName    = "Open-Meteo (BOM ACCESS-G feed unavailable)"
	fallbackSourceLicence = "Open-Meteo CC BY 4.0 (model blend). The dedicated BOM ACCESS-G feed returned no data at request time."
)

// httpBOM provides a connection-pooled client with strict deadlines to prevent hung worker goroutines.
var httpBOM = &http.Client{Timeout: upstreamTimeout}

type omResponse struct {
	Timezone string    `json:"timezone"`
	Current  omCurrent `json:"current"`
	Hourly   omHourly  `json:"hourly"`
	Daily    omDaily   `json:"daily"`
}

type omCurrent struct {
	Time     string   `json:"time"`
	Temp     *float64 `json:"temperature_2m"`
	Humidity *float64 `json:"relative_humidity_2m"`
	Apparent *float64 `json:"apparent_temperature"`
	Precip   *float64 `json:"precipitation"`
	Code     *int     `json:"weather_code"`
	Cloud    *float64 `json:"cloud_cover"`
	Pressure *float64 `json:"surface_pressure"`
	Wind     *float64 `json:"wind_speed_10m"`
	Gust     *float64 `json:"wind_gusts_10m"`
	Dir      *float64 `json:"wind_direction_10m"`
	IsDay    *int     `json:"is_day"`
}

type omHourly struct {
	Time       []string   `json:"time"`
	Temp       []*float64 `json:"temperature_2m"`
	Apparent   []*float64 `json:"apparent_temperature"`
	Humidity   []*float64 `json:"relative_humidity_2m"`
	PrecipProb []*float64 `json:"precipitation_probability"`
	Precip     []*float64 `json:"precipitation"`
	Code       []*int     `json:"weather_code"`
	Cloud      []*float64 `json:"cloud_cover"`
	Pressure   []*float64 `json:"surface_pressure"`
	Visibility []*float64 `json:"visibility"`
	Wind       []*float64 `json:"wind_speed_10m"`
	Gust       []*float64 `json:"wind_gusts_10m"`
	Dir        []*float64 `json:"wind_direction_10m"`
	Dew        []*float64 `json:"dew_point_2m"`
	UV         []*float64 `json:"uv_index"`
	IsDay      []*int     `json:"is_day"`
}

type omDaily struct {
	Time          []string   `json:"time"`
	TMin          []*float64 `json:"temperature_2m_min"`
	TMax          []*float64 `json:"temperature_2m_max"`
	AppMin        []*float64 `json:"apparent_temperature_min"`
	AppMax        []*float64 `json:"apparent_temperature_max"`
	Sunrise       []string   `json:"sunrise"`
	Sunset        []string   `json:"sunset"`
	PrecipSum     []*float64 `json:"precipitation_sum"`
	PrecipProbMax []*float64 `json:"precipitation_probability_max"`
	WindMax       []*float64 `json:"wind_speed_10m_max"`
	WindGustMax   []*float64 `json:"wind_gusts_10m_max"`
	WindDirDom    []*float64 `json:"wind_direction_10m_dominant"`
	UVMax         []*float64 `json:"uv_index_max"`
	Code          []*int     `json:"weather_code"`
}

// OMHasData detects empty or null upstream payloads to trigger seamless fallback switching.
// This prevents HTTP 200 responses with absent sensor arrays from corrupting client state.
func OMHasData(raw *omResponse) bool {
	if raw == nil {
		return false
	}
	if raw.Current.Temp != nil {
		return true
	}
	for _, v := range raw.Hourly.Temp {
		if v != nil {
			return true
		}
	}
	for _, v := range raw.Daily.TMax {
		if v != nil {
			return true
		}
	}
	return false
}

type fetchStat struct {
	Location  string    `json:"location"`
	LatencyMS int64     `json:"latency_ms"`
	At        time.Time `json:"at"`
	OK        bool      `json:"ok"`
	FromCache bool      `json:"from_cache"`
}

type sourceMetrics struct {
	mu           sync.Mutex
	lastByLoc    map[string]int64
	recent       []fetchStat
	attempts     int
	liveOK       int
	cacheHits    int
	failures     int
	lastConcMS   int64
	lastSeqMS    int64
	lastLocCount int
}

// Metrics tracks retrieval health and concurrency performance metrics for diagnostics.
var Metrics = &sourceMetrics{lastByLoc: map[string]int64{}}

func (m *sourceMetrics) record(s fetchStat) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attempts++
	if s.OK && s.FromCache {
		m.cacheHits++
	}
	if s.OK && !s.FromCache {
		m.liveOK++
		m.lastByLoc[s.Location] = s.LatencyMS
	}
	if !s.OK {
		m.failures++
	}
	m.recent = append(m.recent, s)
	if len(m.recent) > 200 {
		m.recent = m.recent[len(m.recent)-200:]
	}
}

// Snapshot aggregates rolling latency and availability ratios for administrator dashboards.
func (m *sourceMetrics) Snapshot() (avg, successPct, cachePct float64, n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var sum int64
	okCount := 0
	for _, s := range m.recent {
		if s.OK {
			okCount++
			if s.FromCache {
				cachePct++
			} else {
				sum += s.LatencyMS
			}
		}
	}
	if okCount > 0 {
		avg = float64(sum) / float64(okCount)
		cachePct = cachePct / float64(okCount) * 100
	}
	if m.attempts > 0 {
		successPct = float64(m.liveOK+m.cacheHits) / float64(m.attempts) * 100
	}
	return avg, successPct, cachePct, len(m.recent)
}

// SetConcurrency updates concurrent and sequential benchmarks from multi-location updates.
func (m *sourceMetrics) SetConcurrency(conc, seq int64, locs int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastConcMS, m.lastSeqMS, m.lastLocCount = conc, seq, locs
}

// Concurrency exposes latest parallel batch benchmark numbers for throughput audits.
func (m *sourceMetrics) Concurrency() (int64, int64, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastConcMS, m.lastSeqMS, m.lastLocCount
}

type memCacheEntry struct {
	payload []byte
	at      time.Time
}

var (
	cacheMu  sync.RWMutex
	cacheMem = map[string]memCacheEntry{}
)

// cacheGet retrieves cached responses from memory or SQLite to preserve availability across restarts.
func cacheGet(key string) (payload []byte, at time.Time, fresh bool, ok bool) {
	cacheMu.RLock()
	e, found := cacheMem[key]
	cacheMu.RUnlock()
	if found && time.Since(e.at) < cacheTTL {
		return e.payload, e.at, true, true
	}
	// Fall back to SQLite storage when memory entries are absent or expired.
	var p string
	var fetchedAt time.Time
	err := database.DB.QueryRow(`SELECT payload, fetched_at FROM weather_cache WHERE cache_key = ?`, key).Scan(&p, &fetchedAt)
	if err != nil {
		return nil, time.Time{}, false, false
	}
	stale := []byte(p)
	cacheMu.Lock()
	cacheMem[key] = memCacheEntry{payload: stale, at: fetchedAt}
	cacheMu.Unlock()
	return stale, fetchedAt, time.Since(fetchedAt) < cacheTTL, true
}

// cachePut synchronizes updated payloads into memory and persistent database storage.
func cachePut(key, locationName string, payload []byte, at time.Time) {
	cacheMu.Lock()
	cacheMem[key] = memCacheEntry{payload: payload, at: at}
	cacheMu.Unlock()
	expires := at.Add(cacheTTL)
	_, err := database.DB.Exec(`INSERT INTO weather_cache(cache_key,location_name,payload,fetched_at,expires_at)
		VALUES(?,?,?,?,?)
		ON CONFLICT(cache_key) DO UPDATE SET payload=excluded.payload, fetched_at=excluded.fetched_at,
		expires_at=excluded.expires_at, hits=weather_cache.hits+1`,
		key, locationName, string(payload), at.Format(time.RFC3339), expires.Format(time.RFC3339))
	if err != nil {
		log.Printf("cache write: %v", err)
	}
}

// FetchForecast resolves weather forecasts via cache validation with fallback to live HTTP queries.
// If upstream providers are unresponsive, it returns the most recent cached snapshot marked as degraded.
func FetchForecast(ctx context.Context, loc LocationRef, refresh bool) (*ForecastResponse, error) {
	key := database.CacheKey(loc)
	if !refresh {
		if payload, at, fresh, ok := cacheGet(key); ok && fresh {
			fc := DecodeForecast(payload)
			if fc != nil {
				fc.FromCache = false
				fc.FetchedAt = at
				Metrics.record(fetchStat{Location: loc.Name, LatencyMS: 0, At: time.Now(), OK: true})
				return fc, nil
			}
		}
	}
	start := time.Now()
	fc, err := liveBOMFetch(ctx, loc)
	latency := time.Since(start).Milliseconds()
	if err == nil {
		fc.LatencyMS = latency
		fc.FetchedAt = time.Now()
		Metrics.record(fetchStat{Location: loc.Name, LatencyMS: latency, At: time.Now(), OK: true})
		if payload, mErr := json.Marshal(fc); mErr == nil {
			cachePut(key, loc.Name, payload, fc.FetchedAt)
		}
		return fc, nil
	}
	// Upstream query failed; degrade gracefully by serving cached historical payload.
	if payload, at, _, ok := cacheGet(key); ok {
		fc := DecodeForecast(payload)
		if fc != nil {
			fc.FromCache = true
			fc.FetchedAt = at
			fc.LatencyMS = latency
			Metrics.record(fetchStat{Location: loc.Name, LatencyMS: latency, At: time.Now(), OK: true, FromCache: true})
			return fc, nil
		}
	}
	Metrics.record(fetchStat{Location: loc.Name, LatencyMS: latency, At: time.Now(), OK: false})
	return nil, fmt.Errorf("weather source unavailable and no cached data exists for %s: %v", loc.Name, err)
}

// DecodeForecast deserializes binary cache payloads into typed forecast representations.
func DecodeForecast(payload []byte) *ForecastResponse {
	var fc ForecastResponse
	if err := json.Unmarshal(payload, &fc); err != nil {
		return nil
	}
	return &fc
}

// BOMQuery formats HTTP parameters required by Open-Meteo ACCESS-G and blended endpoints.
func BOMQuery(loc LocationRef) url.Values {
	q := url.Values{}
	q.Set("latitude", fmt.Sprintf("%.4f", loc.Latitude))
	q.Set("longitude", fmt.Sprintf("%.4f", loc.Longitude))
	q.Set("timezone", loc.Timezone)
	q.Set("forecast_days", "7")
	q.Set("current", "temperature_2m,relative_humidity_2m,apparent_temperature,is_day,precipitation,weather_code,cloud_cover,surface_pressure,wind_speed_10m,wind_gusts_10m,wind_direction_10m")
	q.Set("hourly", "temperature_2m,apparent_temperature,relative_humidity_2m,precipitation_probability,precipitation,weather_code,cloud_cover,surface_pressure,visibility,wind_speed_10m,wind_gusts_10m,wind_direction_10m,dew_point_2m,uv_index,is_day")
	q.Set("daily", "temperature_2m_min,temperature_2m_max,apparent_temperature_min,apparent_temperature_max,sunrise,sunset,precipitation_sum,precipitation_probability_max,wind_speed_10m_max,wind_gusts_10m_max,wind_direction_10m_dominant,uv_index_max,weather_code")
	q.Set("wind_speed_unit", "kmh")
	return q
}

// GetOpenMeteo executes an outbound GET request with educational identification headers.
func GetOpenMeteo(ctx context.Context, endpoint string, q url.Values) (*omResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ATMOS-WeatherDashboard/1.0 (education project; data BOM via Open-Meteo)")
	resp, err := httpBOM.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 400))
		return nil, fmt.Errorf("endpoint status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var raw omResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &raw, nil
}

// liveBOMFetch attempts retrieval from Australian ACCESS-G before failing over to blended models.
func liveBOMFetch(ctx context.Context, loc LocationRef) (*ForecastResponse, error) {
	q := BOMQuery(loc)
	raw, err := GetOpenMeteo(ctx, BOMEndpoint, q)
	if err == nil && OMHasData(raw) {
		return buildForecast(loc, raw, SourceName, SourceLicence, BOMEndpoint, false), nil
	}
	primary := err
	if primary == nil {
		primary = fmt.Errorf("BOM ACCESS-G feed returned no usable values")
	}
	raw2, err2 := GetOpenMeteo(ctx, NodeEndpoint, q)
	if err2 == nil && OMHasData(raw2) {
		log.Printf("weather source: %s; serving fallback forecast for %s", primary, loc.Name)
		return buildForecast(loc, raw2, fallbackSourceName, fallbackSourceLicence, NodeEndpoint, true), nil
	}
	if err2 != nil {
		return nil, fmt.Errorf("BOM feed: %v; fallback feed: %v", primary, err2)
	}
	return nil, fmt.Errorf("no usable forecast data: %v", primary)
}

// buildForecast transforms upstream series into standardized client domain models.
func buildForecast(loc LocationRef, raw *omResponse, source, licence, sourceURL string, degraded bool) *ForecastResponse {
	out := &ForecastResponse{
		Location:       loc,
		Timezone:       raw.Timezone,
		Source:         source,
		Licence:        licence,
		SourceURL:      sourceURL,
		SourceDegraded: degraded,
	}
	if out.Timezone == "" {
		out.Timezone = loc.Timezone
	}
	if len(raw.Hourly.Time) > 0 {
		out.Hourly = make([]HourlyPoint, 0, len(raw.Hourly.Time))
		for i, t := range raw.Hourly.Time {
			code := atInt(raw.Hourly.Code, i)
			cond, _ := CodeToCondition(code)
			out.Hourly = append(out.Hourly, HourlyPoint{
				Time:         t,
				TempC:        atFloat(raw.Hourly.Temp, i),
				FeelsLikeC:   atFloat(raw.Hourly.Apparent, i),
				Humidity:     int(math.Round(atFloat(raw.Hourly.Humidity, i))),
				PrecipProb:   int(math.Round(atFloat(raw.Hourly.PrecipProb, i))),
				PrecipMM:     Round1(atFloat(raw.Hourly.Precip, i)),
				WindSpeedKmh: Round1(atFloat(raw.Hourly.Wind, i)),
				WindGustKmh:  Round1(atFloat(raw.Hourly.Gust, i)),
				WindDirDeg:   int(math.Round(atFloat(raw.Hourly.Dir, i))),
				WindDir:      windDirText(int(math.Round(atFloat(raw.Hourly.Dir, i)))),
				PressureHPa:  Round1(atFloat(raw.Hourly.Pressure, i)),
				CloudCover:   int(math.Round(atFloat(raw.Hourly.Cloud, i))),
				VisibilityKm: Round1(atFloat(raw.Hourly.Visibility, i) / 1000), // metres -> km
				UVIndex:      Round1(atFloat(raw.Hourly.UV, i)),
				DewPointC:    Round1(atFloat(raw.Hourly.Dew, i)),
				WeatherCode:  code,
				Condition:    cond,
				IsDay:        atInt(raw.Hourly.IsDay, i) == 1,
			})
		}
	}
	// Daily rows with aggregates computed from the matching hourly hours.
	if len(raw.Daily.Time) > 0 {
		out.Daily = make([]DailyForecast, 0, len(raw.Daily.Time))
		for i, date := range raw.Daily.Time {
			code := atInt(raw.Daily.Code, i)
			cond, _ := CodeToCondition(code)
			agg := aggregateDay(out.Hourly, date)
			out.Daily = append(out.Daily, DailyForecast{
				Date:           date,
				TempMinC:       Round1(atFloat(raw.Daily.TMin, i)),
				TempMaxC:       Round1(atFloat(raw.Daily.TMax, i)),
				ApparentMinC:   Round1(atFloat(raw.Daily.AppMin, i)),
				ApparentMaxC:   Round1(atFloat(raw.Daily.AppMax, i)),
				HumidityAvg:    agg.humidity,
				PrecipSumMM:    Round1(atFloat(raw.Daily.PrecipSum, i)),
				PrecipProbMax:  int(math.Round(atFloat(raw.Daily.PrecipProbMax, i))),
				WindMaxKmh:     Round1(atFloat(raw.Daily.WindMax, i)),
				WindGustMaxKmh: Round1(atFloat(raw.Daily.WindGustMax, i)),
				WindDirDeg:     int(math.Round(atFloat(raw.Daily.WindDirDom, i))),
				WindDir:        windDirText(int(math.Round(atFloat(raw.Daily.WindDirDom, i)))),
				PressureAvgHPa: agg.pressure,
				CloudCoverAvg:  agg.cloud,
				VisibilityKm:   agg.visibility,
				UVIndexMax:     Round1(atFloat(raw.Daily.UVMax, i)),
				DewPointAvgC:   agg.dew,
				Sunrise:        atString(raw.Daily.Sunrise, i),
				Sunset:         atString(raw.Daily.Sunset, i),
				WeatherCode:    code,
				Condition:      cond,
			})
		}
	}
	// Current block: upstream "current" plus the matching hourly index so
	// UV / visibility / dew point are real values for this hour.
	if raw.Current.Time != "" {
		code := iv(raw.Current.Code)
		cur := &CurrentWeather{
			Time:         raw.Current.Time,
			TempC:        Round1(dv(raw.Current.Temp)),
			FeelsLikeC:   Round1(dv(raw.Current.Apparent)),
			Humidity:     int(math.Round(dv(raw.Current.Humidity))),
			PrecipMM:     Round1(dv(raw.Current.Precip)),
			WindSpeedKmh: Round1(dv(raw.Current.Wind)),
			WindGustKmh:  Round1(dv(raw.Current.Gust)),
			WindDirDeg:   int(math.Round(dv(raw.Current.Dir))),
			WindDir:      windDirText(int(math.Round(dv(raw.Current.Dir)))),
			PressureHPa:  Round1(dv(raw.Current.Pressure)),
			CloudCover:   int(math.Round(dv(raw.Current.Cloud))),
			WeatherCode:  code,
			IsDay:        iv(raw.Current.IsDay) == 1,
		}
		cur.Condition, _ = CodeToCondition(code)
		if idx := indexOfWorkingHour(raw.Hourly.Time, raw.Current.Time); idx >= 0 {
			cur.UVIndex = Round1(atFloat(raw.Hourly.UV, idx))
			cur.VisibilityKm = Round1(atFloat(raw.Hourly.Visibility, idx) / 1000)
			cur.DewPointC = Round1(atFloat(raw.Hourly.Dew, idx))
			if p, ok := atFloatOK(raw.Hourly.PrecipProb, idx); ok {
				cur.PrecipProb = int(math.Round(p))
			}
		}
		if len(out.Daily) > 0 {
			cur.Sunrise = out.Daily[0].Sunrise
			cur.Sunset = out.Daily[0].Sunset
		}
		out.Current = cur
	}
	return out
}

type dayAgg struct {
	humidity   int
	pressure   float64
	cloud      int
	visibility float64
	dew        float64
}

// aggregateDay averages the hourly series that belong to the given date.
func aggregateDay(hourly []HourlyPoint, date string) dayAgg {
	var hum, cloud, n int
	var pres, vis, dew float64
	prefix := date + "T"
	for _, h := range hourly {
		if !strings.HasPrefix(h.Time, prefix) {
			continue
		}
		n++
		hum += h.Humidity
		cloud += h.CloudCover
		pres += h.PressureHPa
		vis += h.VisibilityKm
		dew += h.DewPointC
	}
	if n == 0 {
		return dayAgg{}
	}
	return dayAgg{
		humidity:   hum / n,
		pressure:   Round1(pres / float64(n)),
		cloud:      cloud / n,
		visibility: Round1(vis / float64(n)),
		dew:        Round1(dew / float64(n)),
	}
}

func indexOfWorkingHour(times []string, current string) int {
	if len(current) < 13 || len(times) == 0 {
		return -1
	}
	prefix := current[:13] // YYYY-MM-DDTH
	for i, t := range times {
		if strings.HasPrefix(t, prefix) {
			return i
		}
	}
	return -1
}

// FetchResult pairs a requested location with its forecast outcome and latency metrics.
type FetchResult struct {
	Loc       LocationRef
	Forecast  *ForecastResponse
	Err       error
	LatencyMS int64
}

// FetchManyConcurrently fetches multiple locations using worker goroutines to minimize wall-clock latency.
// It measures both concurrent duration and cumulative sequential latency for system throughput benchmarks.
func FetchManyConcurrently(ctx context.Context, locs []LocationRef, refresh bool) ([]FetchResult, time.Duration, int64) {
	started := time.Now()
	jobs := make(chan LocationRef)
	results := make(chan FetchResult, len(locs))
	workers := len(locs)
	if workers > 8 {
		workers = 8
	}
	if workers < 1 {
		workers = 1
	}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for loc := range jobs {
				start := time.Now()
				fc, err := FetchForecast(ctx, loc, refresh)
				res := FetchResult{Loc: loc, Forecast: fc, Err: err, LatencyMS: time.Since(start).Milliseconds()}
				results <- res
			}
		}()
	}
	go func() {
		for _, loc := range locs {
			select {
			case jobs <- loc:
			case <-ctx.Done():
			}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()
	out := make([]FetchResult, 0, len(locs))
	var seq int64
	for r := range results {
		if r.Forecast != nil {
			seq += r.LatencyMS
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Loc.Name < out[j].Loc.Name })
	return out, time.Since(started), seq
}

// CodeToCondition translates standard WMO meteorological codes into human-readable descriptions and emoji indicators.
func CodeToCondition(code int) (string, string) {
	switch code {
	case 0:
		return "Clear", "☀️"
	case 1:
		return "Mainly clear", "🌤️"
	case 2:
		return "Partly cloudy", "⛅"
	case 3:
		return "Overcast", "☁️"
	case 45:
		return "Fog", "🌫️"
	case 48:
		return "Rime fog", "🌫️"
	case 51:
		return "Light drizzle", "🌦️"
	case 53:
		return "Drizzle", "🌦️"
	case 55:
		return "Dense drizzle", "🌧️"
	case 56, 57:
		return "Freezing drizzle", "🌧️"
	case 61:
		return "Slight rain", "🌦️"
	case 63:
		return "Rain", "🌧️"
	case 65:
		return "Heavy rain", "🌧️"
	case 66, 67:
		return "Freezing rain", "🌧️"
	case 71:
		return "Light snow", "🌨️"
	case 73:
		return "Snow", "🌨️"
	case 75:
		return "Heavy snow", "🌨️"
	case 77:
		return "Snow grains", "🌨️"
	case 80:
		return "Rain showers", "🌦️"
	case 81:
		return "Showers", "🌧️"
	case 82:
		return "Violent showers", "🌧️"
	case 85:
		return "Snow showers", "🌨️"
	case 86:
		return "Heavy snow showers", "🌨️"
	case 95:
		return "Thunderstorm", "⛈️"
	case 96, 99:
		return "Thunderstorm with hail", "⛈️"
	default:
		return "Unknown", "🌡️"
	}
}

// windDirText maps compass degree azimuths to standard 16-point nautical cardinal notations.
func windDirText(deg int) string {
	if deg < 0 || deg > 360 {
		deg = (deg + 360) % 360
	}
	pts := []string{"N", "NE", "NE", "ENE", "E", "ESE", "SE", "SSE", "S", "SSW", "SW", "WSW", "W", "WNW", "NW", "NNW"}
	idx := int(math.Round(float64(deg)/22.5)) % 16
	return pts[idx]
}

// dv safely dereferences nullable upstream float pointers to guard against nil pointer panics.
func dv(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

// iv safely dereferences nullable upstream integer pointers to guard against nil pointer panics.
func iv(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func atFloat(s []*float64, i int) float64 {
	if i < 0 || i >= len(s) {
		return 0
	}
	return dv(s[i])
}

func atFloatOK(s []*float64, i int) (float64, bool) {
	if i < 0 || i >= len(s) || s[i] == nil {
		return 0, false
	}
	return *s[i], true
}

func atInt(s []*int, i int) int {
	if i < 0 || i >= len(s) {
		return 0
	}
	return iv(s[i])
}

func atString(s []string, i int) string {
	if i < 0 || i >= len(s) {
		return ""
	}
	return s[i]
}

// Round1 normalizes numeric precision to one decimal place for consistent UI formatting.
func Round1(v float64) float64 {
	return math.Round(v*10) / 10
}
