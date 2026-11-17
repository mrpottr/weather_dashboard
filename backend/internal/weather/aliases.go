package weather

import "weather-forecast-database"

/*
Package weather maps data models between meteorological processing routines and persistent database storage.
Aliasing these structures avoids circular imports while providing type safety across weather subsystems.
*/

type (
	LocationRef      = database.LocationRef
	ForecastResponse = database.ForecastResponse
	CurrentWeather   = database.CurrentWeather
	HourlyPoint      = database.HourlyPoint
	DailyForecast    = database.DailyForecast
	GeoPlace         = database.GeoPlace
	ActivityScore    = database.ActivityScore
	ScoreFactor      = database.ScoreFactor
	AccuracySample   = database.AccuracySample
)
