package database

import (
	"fmt"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// models.go — every struct / DTO used across the ATMOS backend, plus the
// shared low-level helpers (time/bool/null conversion) and the well-known
// Australian locations used by seeds, map markers and fallbacks.
// Presentation layer (Gin handlers), business services and data layer all
// share these types so the three-layer architecture stays consistent.
// ---------------------------------------------------------------------------

// Role and status constants.
const (
	RoleAdmin     = "admin"
	RoleStandard  = "standard"
	StatusActive  = "active"
	StatusSuspend = "suspended"
	StatusDeact   = "deactivated"
)

// User is the account record exposed to the API (never contains the hash).
type User struct {
	ID              int64      `json:"id"`
	FirstName       string     `json:"first_name"`
	LastName        string     `json:"last_name"`
	Email           string     `json:"email"`
	Role            string     `json:"role"`
	Status          string     `json:"status"`
	DefaultLocation string     `json:"default_location"`
	CreatedAt       time.Time  `json:"created_at"`
	LastActiveAt    *time.Time `json:"last_active_at,omitempty"`
}

// Preferences holds per-user UI choices persisted in user_preferences.
type Preferences struct {
	Theme       string `json:"theme"`        // dark | light
	ClockFormat string `json:"clock_format"` // 12h | 24h
}

// UserWithPrefs bundles the login/me payload.
type UserWithPrefs struct {
	User
	Preferences Preferences `json:"preferences"`
}

// RegisterRequest is the create-account payload.
type RegisterRequest struct {
	FirstName       string  `json:"first_name" binding:"required"`
	LastName        string  `json:"last_name" binding:"required"`
	Email           string  `json:"email" binding:"required,email"`
	Password        string  `json:"password" binding:"required,min=6"`
	DefaultLocation string  `json:"default_location" binding:"required"`
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
	Timezone        string  `json:"timezone"`
	AdminArea       string  `json:"admin_area"`
}

// LoginRequest is the sign-in payload.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// AuthResponse is returned by register/login (and admin log-in-as).
type AuthResponse struct {
	Token string        `json:"token"`
	User  UserWithPrefs `json:"user"`
}

// UpdateAccountRequest changes the signed-in user's own account.
type UpdateAccountRequest struct {
	FirstName       *string `json:"first_name"`
	LastName        *string `json:"last_name"`
	Email           *string `json:"email"`
	CurrentPassword string  `json:"current_password"`
	Password        *string `json:"password"`
	DefaultLocation *string `json:"default_location"`
}

// PreferencesRequest updates theme / clock format.
type PreferencesRequest struct {
	Theme       *string `json:"theme"`
	ClockFormat *string `json:"clock_format"`
}

// ErrorResponse is the single error shape used by every handler.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// LocationRef identifies an Australian place (search result or favourite).
type LocationRef struct {
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timezone  string  `json:"timezone"`
	AdminArea string  `json:"admin_area"`
	Postcode  string  `json:"postcode,omitempty"`
}

// FavouriteLocation is a saved location row.
type FavouriteLocation struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Name      string    `json:"name"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Timezone  string    `json:"timezone"`
	AdminArea string    `json:"admin_area"`
	SavedAt   time.Time `json:"saved_at"`
}

// GeoPlace is one result of the Open-Meteo geocoding API (restricted to AU).
type GeoPlace struct {
	Name      string  `json:"name"`
	AdminArea string  `json:"admin_area"`
	Country   string  `json:"country"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timezone  string  `json:"timezone"`
	Postcode  string  `json:"postcode,omitempty"`
}

// CurrentWeather is the "now" block of a BOM forecast.
type CurrentWeather struct {
	Time         string  `json:"time"`
	TempC        float64 `json:"temp_c"`
	FeelsLikeC   float64 `json:"feels_like_c"`
	Humidity     int     `json:"humidity"`
	PrecipMM     float64 `json:"precip_mm"`
	PrecipProb   int     `json:"precip_probability"`
	WindSpeedKmh float64 `json:"wind_speed_kmh"`
	WindGustKmh  float64 `json:"wind_gust_kmh"`
	WindDirDeg   int     `json:"wind_direction_deg"`
	WindDir      string  `json:"wind_dir"`
	PressureHPa  float64 `json:"surface_pressure_hpa"`
	CloudCover   int     `json:"cloud_cover"`
	VisibilityKm float64 `json:"visibility_km"`
	UVIndex      float64 `json:"uv_index"`
	DewPointC    float64 `json:"dew_point_c"`
	WeatherCode  int     `json:"weather_code"`
	Condition    string  `json:"condition"`
	IsDay        bool    `json:"is_day"`
	Sunrise      string  `json:"sunrise"`
	Sunset       string  `json:"sunset"`
}

// HourlyPoint is one hour of the forecast used by charts and the rubric.
type HourlyPoint struct {
	Time         string  `json:"time"`
	TempC        float64 `json:"temp_c"`
	FeelsLikeC   float64 `json:"feels_like_c"`
	Humidity     int     `json:"humidity"`
	PrecipProb   int     `json:"precip_probability"`
	PrecipMM     float64 `json:"precip_mm"`
	WindSpeedKmh float64 `json:"wind_speed_kmh"`
	WindGustKmh  float64 `json:"wind_gust_kmh"`
	WindDirDeg   int     `json:"wind_direction_deg"`
	WindDir      string  `json:"wind_dir"`
	PressureHPa  float64 `json:"surface_pressure_hpa"`
	CloudCover   int     `json:"cloud_cover"`
	VisibilityKm float64 `json:"visibility_km"`
	UVIndex      float64 `json:"uv_index"`
	DewPointC    float64 `json:"dew_point_c"`
	WeatherCode  int     `json:"weather_code"`
	Condition    string  `json:"condition"`
	IsDay        bool    `json:"is_day"`
}

// DailyForecast is one day of the forecast.
type DailyForecast struct {
	Date           string  `json:"date"`
	TempMinC       float64 `json:"temp_min_c"`
	TempMaxC       float64 `json:"temp_max_c"`
	ApparentMinC   float64 `json:"apparent_min_c"`
	ApparentMaxC   float64 `json:"apparent_max_c"`
	HumidityAvg    int     `json:"humidity_avg"`
	PrecipSumMM    float64 `json:"precip_sum_mm"`
	PrecipProbMax  int     `json:"precip_probability_max"`
	WindMaxKmh     float64 `json:"wind_max_kmh"`
	WindGustMaxKmh float64 `json:"wind_gust_max_kmh"`
	WindDirDeg     int     `json:"wind_direction_deg"`
	WindDir        string  `json:"wind_dir"`
	PressureAvgHPa float64 `json:"pressure_avg_hpa"`
	CloudCoverAvg  int     `json:"cloud_cover_avg"`
	VisibilityKm   float64 `json:"visibility_km"`
	UVIndexMax     float64 `json:"uv_index_max"`
	DewPointAvgC   float64 `json:"dew_point_avg"`
	Sunrise        string  `json:"sunrise"`
	Sunset         string  `json:"sunset"`
	WeatherCode    int     `json:"weather_code"`
	Condition      string  `json:"condition"`
}

// ForecastResponse is the payload served by GET /api/weather/forecast.
type ForecastResponse struct {
	Location  LocationRef `json:"location"`
	Timezone  string      `json:"timezone"`
	Source    string      `json:"source"`
	Licence   string      `json:"licence"`
	SourceURL string      `json:"source_url"`
	// SourceDegraded is true when the dedicated BOM/ACCESS-G feed had no data
	// and the fallback Open-Meteo endpoint supplied the values.
	SourceDegraded bool            `json:"source_degraded"`
	FetchedAt      time.Time       `json:"fetched_at"`
	FromCache      bool            `json:"from_cache"`
	LatencyMS      int64           `json:"latency_ms"`
	Current        *CurrentWeather `json:"current,omitempty"`
	Hourly         []HourlyPoint   `json:"hourly"`
	Daily          []DailyForecast `json:"daily"`
}

// MarkerPayload powers the map page: many locations fetched concurrently.
type MarkerPayload struct {
	Locations    []MarkerLocation `json:"locations"`
	ConcurrentMS int64            `json:"concurrent_ms"`
	SequentialMS int64            `json:"sequential_ms"`
	Count        int              `json:"count"`
}

// MarkerLocation is one plotted city with live values.
type MarkerLocation struct {
	Name      string   `json:"name"`
	AdminArea string   `json:"admin_area"`
	Latitude  float64  `json:"latitude"`
	Longitude float64  `json:"longitude"`
	Timezone  string   `json:"timezone"`
	Saved     bool     `json:"saved"`
	TempC     *float64 `json:"temp_c,omitempty"`
	Condition *string  `json:"condition,omitempty"`
	Icon      *string  `json:"icon,omitempty"`
	FromCache bool     `json:"from_cache"`
}

// ActivityDef is a supported outdoor activity of the intelligence rubric.
type ActivityDef struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Emoji string `json:"emoji"`
}

// ScoreFactor documents one deterministic bonus/penalty with the exact number used.
type ScoreFactor struct {
	Factor  string  `json:"factor"`
	Reading string  `json:"reading"`
	Points  float64 `json:"points"`
	Reason  string  `json:"reason"`
}

// ActivityScore is the detailed suitability result for one activity.
type ActivityScore struct {
	ActivityKey string        `json:"activity_key"`
	Activity    string        `json:"activity"`
	Emoji       string        `json:"emoji"`
	Score       int           `json:"score"`
	Rating      string        `json:"rating"`
	Tag         string        `json:"tag"`
	Title       string        `json:"title"`
	Summary     string        `json:"summary"`
	Factors     []ScoreFactor `json:"factors"`
	Advice      []string      `json:"advice"`
	BestWindow  string        `json:"best_window"`
	BestHours   []string      `json:"best_hours"`
	Date        string        `json:"date"`
	Location    string        `json:"location"`
}

// IntelligenceResponse returns every activity for one location/date.
type IntelligenceResponse struct {
	Location   string          `json:"location"`
	Date       string          `json:"date"`
	Activities []ActivityScore `json:"activities"`
}

// WeatherAlert is a threshold-based alert derived from the live forecast.
type WeatherAlert struct {
	ID             int64     `json:"id"`
	Type           string    `json:"type"`
	Severity       string    `json:"severity"`
	Title          string    `json:"title"`
	Message        string    `json:"message"`
	Trigger        string    `json:"trigger"`
	ActualValue    string    `json:"actual_value"`
	ThresholdValue string    `json:"threshold_value"`
	Location       string    `json:"location"`
	Window         string    `json:"window"`
	Precautions    string    `json:"precautions"`
	Dismissed      bool      `json:"dismissed"`
	CreatedAt      time.Time `json:"created_at"`
}

// Announcement is an admin-published system notice.
type Announcement struct {
	ID        int64      `json:"id"`
	Title     string     `json:"title"`
	Message   string     `json:"message"`
	Author    string     `json:"author"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Active    bool       `json:"active"`
	Dismissed bool       `json:"dismissed"`
	State     string     `json:"state"`
}

// Thresholds is the admin-managed alert policy (3.2.2.20) plus retention (3.2.3.11).
type Thresholds struct {
	HighTempC     float64 `json:"high_temp_c"`
	LowTempC      float64 `json:"low_temp_c"`
	RainPct       int     `json:"rain_pct"`
	WindLevel     string  `json:"wind_level"` // strong | gale | storm
	UVIndex       float64 `json:"uv_index"`
	PushEnabled   bool    `json:"push_enabled"`
	RetentionDays int     `json:"retention_days"`
	KeepCache     bool    `json:"keep_cache"`
	RetainHistory bool    `json:"retain_history"`
}

// SystemLog is one structured runtime event stored in system_logs.
type SystemLog struct {
	ID     int64     `json:"id"`
	TS     time.Time `json:"ts"`
	Actor  string    `json:"actor"`
	Role   string    `json:"role"`
	Action string    `json:"action"`
	Detail string    `json:"detail"`
}

// HistoryEntry is one persisted forecast/query record (3.2.2.7).
type HistoryEntry struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	LocationName string    `json:"location_name"`
	Date         string    `json:"date"`
	Source       string    `json:"source"`
	FromCache    bool      `json:"from_cache"`
	Result       string    `json:"result"`
	Query        string    `json:"query"`
	TS           time.Time `json:"ts"`
}

// AccuracySample stores one forecast-vs-observation comparison (3.2.2.14).
type AccuracySample struct {
	ID       int64     `json:"id"`
	Location string    `json:"location"`
	Date     string    `json:"date"`
	Metric   string    `json:"metric"`
	Forecast float64   `json:"forecast"`
	Observed float64   `json:"observed"`
	Percent  float64   `json:"percent"`
	TS       time.Time `json:"ts"`
}

// AccuracyReport aggregates accuracy per metric.
type AccuracyReport struct {
	Location string             `json:"location"`
	Samples  int                `json:"samples"`
	Overall  float64            `json:"overall"`
	Metrics  map[string]float64 `json:"metrics"`
	Recent   []AccuracySample   `json:"recent"`
}

// SourceCheck is the admin "verify source" result (3.2.2.19).
type SourceCheck struct {
	Healthy   bool      `json:"healthy"`
	Endpoint  string    `json:"endpoint"`
	LatencyMS int64     `json:"latency_ms"`
	Message   string    `json:"message"`
	CheckedAt time.Time `json:"checked_at"`
	// PrimaryOk reports whether the dedicated BOM/ACCESS-G endpoint returned
	// usable values; FallbackOk whether the general Open-Meteo endpoint did.
	// Serving records the source actually used for the probe location.
	PrimaryOk    bool   `json:"primary_ok"`
	PrimaryNote  string `json:"primary_note"`
	FallbackOk   bool   `json:"fallback_ok"`
	FallbackNote string `json:"fallback_note"`
	Serving      string `json:"serving"`
	SourceNote   string `json:"source_note"`
}

// AdminMetrics carries latency / cache statistics for the admin console,
// including the report novelty: concurrent vs sequential retrieval benefit.
type AdminMetrics struct {
	ActiveUsers      int     `json:"active_users"`
	TotalUsers       int     `json:"total_users"`
	LocationsFetched int     `json:"locations_fetched"`
	ConcurrentMS     int64   `json:"concurrent_ms"`
	SequentialMS     int64   `json:"sequential_ms"`
	SpeedupFactor    float64 `json:"speedup_factor"`
	CacheCoveragePct float64 `json:"cache_coverage_pct"`
	APISuccessPct    float64 `json:"api_success_pct"`
	AvgLatencyMS     int64   `json:"avg_latency_ms"`
	AlertsActive     int     `json:"alerts_active"`
	CacheEntries     int     `json:"cache_entries"`
	HistoryRows      int     `json:"history_rows"`
}

// TownImages is the response of the hero photo endpoint (2.2.3).
type TownImages struct {
	Town     string   `json:"town"`
	Provider string   `json:"provider"`
	Images   []string `json:"images"`
	Cached   bool     `json:"cached"`
	Credit   string   `json:"credit"`
}

// NotificationSummary feeds the bell badge (3.5).
type NotificationSummary struct {
	Unread       int `json:"unread"`
	WeatherCount int `json:"weather_count"`
	SystemCount  int `json:"system_count"`
}

// ---------------------------------------------------------------------------
// Well-known Australian locations (used by seeds, map markers, fallbacks)
// ---------------------------------------------------------------------------

var (
	Melbourne = LocationRef{Name: "Melbourne, VIC", Latitude: -37.8136, Longitude: 144.9631, Timezone: "Australia/Melbourne", AdminArea: "VIC"}
	Sydney    = LocationRef{Name: "Sydney, NSW", Latitude: -33.8688, Longitude: 151.2093, Timezone: "Australia/Sydney", AdminArea: "NSW"}
	Brisbane  = LocationRef{Name: "Brisbane, QLD", Latitude: -27.4698, Longitude: 153.0251, Timezone: "Australia/Brisbane", AdminArea: "QLD"}
	Perth     = LocationRef{Name: "Perth, WA", Latitude: -31.9523, Longitude: 115.8613, Timezone: "Australia/Perth", AdminArea: "WA"}
	Hobart    = LocationRef{Name: "Hobart, TAS", Latitude: -42.8821, Longitude: 147.3272, Timezone: "Australia/Hobart", AdminArea: "TAS"}
	Adelaide  = LocationRef{Name: "Adelaide, SA", Latitude: -34.9285, Longitude: 138.6007, Timezone: "Australia/Adelaide", AdminArea: "SA"}
	Darwin    = LocationRef{Name: "Darwin, NT", Latitude: -12.4634, Longitude: 130.8456, Timezone: "Australia/Darwin", AdminArea: "NT"}
	Canberra  = LocationRef{Name: "Canberra, ACT", Latitude: -35.2809, Longitude: 149.1300, Timezone: "Australia/Sydney", AdminArea: "ACT"}
)

// MajorCities is the fixed marker set for the Map page.
func MajorCities() []LocationRef {
	return []LocationRef{Melbourne, Sydney, Brisbane, Perth, Hobart, Adelaide, Darwin, Canberra}
}

// DefaultLocations powers the create-account dropdown and search datalist.
func DefaultLocations() []LocationRef {
	return []LocationRef{Melbourne, Sydney, Brisbane, Perth, Hobart, Adelaide, Darwin, Canberra}
}

// MatchDefaultLocation maps the fixed register choices to exact coordinates.
func MatchDefaultLocation(name string) LocationRef {
	for _, l := range DefaultLocations() {
		if strings.EqualFold(l.Name, name) || strings.HasPrefix(strings.ToLower(name), strings.ToLower(strings.Split(l.Name, ",")[0])) {
			return l
		}
	}
	return LocationRef{}
}

// CacheKey derives the stable location key stored in the database.
func CacheKey(loc LocationRef) string {
	return fmt.Sprintf("%.3f|%.3f", loc.Latitude, loc.Longitude)
}

// ---------------------------------------------------------------------------
// Small shared helpers
// ---------------------------------------------------------------------------

// SQLNullString scans NULL text columns into a Go string + validity flag.
type SQLNullString struct {
	String string
	Valid  bool
}

// Scan implements sql.Scanner for NULL-able text columns.
func (n *SQLNullString) Scan(value interface{}) error {
	if value == nil {
		n.String, n.Valid = "", false
		return nil
	}
	switch v := value.(type) {
	case []byte:
		n.String, n.Valid = string(v), true
	case string:
		n.String, n.Valid = v, true
	default:
		n.String, n.Valid = "", false
	}
	return nil
}

// ParseTime parses the timestamp layouts used by SQLite columns.
func ParseTime(s string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05Z07:00", "2006-01-02 15:04:05-07:00"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Now()
}

// BoolToInt converts a boolean into the 0/1 representation used by SQLite.
func BoolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// LocTime resolves the timezone of a forecast payload (fallback: AEST).
func LocTime(fc *ForecastResponse) *time.Location {
	if l, err := time.LoadLocation(fc.Timezone); err == nil {
		return l
	}
	return time.FixedZone("AEST", 10*3600)
}
