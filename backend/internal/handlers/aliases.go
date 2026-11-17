package handlers

import "weather-forecast-database"

/*
Package handlers aliases database domain models to decouple HTTP delivery mechanics from storage schemas.
Re-exporting these entities minimizes boilerplate across handlers while preserving package separation.
*/

type (
	User                 = database.User
	Preferences          = database.Preferences
	UserWithPrefs        = database.UserWithPrefs
	RegisterRequest      = database.RegisterRequest
	LoginRequest         = database.LoginRequest
	AuthResponse         = database.AuthResponse
	UpdateAccountRequest = database.UpdateAccountRequest
	PreferencesRequest   = database.PreferencesRequest
	ErrorResponse        = database.ErrorResponse
	LocationRef          = database.LocationRef
	FavouriteLocation    = database.FavouriteLocation
	GeoPlace             = database.GeoPlace
	MarkerPayload        = database.MarkerPayload
	MarkerLocation       = database.MarkerLocation
	ActivityDef          = database.ActivityDef
	ActivityScore        = database.ActivityScore
	IntelligenceResponse = database.IntelligenceResponse
	WeatherAlert         = database.WeatherAlert
	Announcement         = database.Announcement
	Thresholds           = database.Thresholds
	SystemLog            = database.SystemLog
	AccuracySample       = database.AccuracySample
	AccuracyReport       = database.AccuracyReport
	SourceCheck          = database.SourceCheck
	AdminMetrics         = database.AdminMetrics
	TownImages           = database.TownImages
	NotificationSummary  = database.NotificationSummary
	DailyForecast        = database.DailyForecast
	ForecastResponse     = database.ForecastResponse
)

// Role and account status constants are re-exported to maintain compile-time role verification across handlers.
const (
	RoleAdmin     = database.RoleAdmin
	RoleStandard  = database.RoleStandard
	StatusActive  = database.StatusActive
	StatusSuspend = database.StatusSuspend
	StatusDeact   = database.StatusDeact
)
