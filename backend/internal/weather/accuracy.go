package weather

import (
	"log"
	"math"
	"time"

	database "weather-forecast-database"
)

/*
Package weather tracks empirical prediction error by comparing past forecasts against incoming real-time observations.
Persisting daily accuracy samples per location provides quantifiable validation metrics across temperature, wind, and precipitation.
*/

// RecordAccuracy scores historical forecasts against incoming observations before new data is inserted to prevent self-comparison.
func RecordAccuracy(loc LocationRef, fc *ForecastResponse) {
	if fc == nil || fc.Current == nil || database.DB == nil {
		return
	}
	now := time.Now().In(database.LocTime(fc))
	today := now.Format("2006-01-02")
	key := database.CacheKey(loc)
	var payload string
	// Querying the earlier forecast prior to inserting the new payload ensures the comparison baseline is authentic and independent.
	err := database.DB.QueryRow(`SELECT payload FROM forecast_history
		WHERE location_key = ? AND date = ? AND from_cache = 0
		ORDER BY id DESC LIMIT 1`, key, today).Scan(&payload)
	if err != nil {
		return // no earlier forecast to compare with yet
	}
	prev := DecodeForecast([]byte(payload))
	if prev == nil || prev.Current == nil {
		return
	}
	// Aligning by matching clock hour ensures observation values are strictly compared against the corresponding predicted window.
	prevTimes := make([]string, 0, len(prev.Hourly))
	for _, h := range prev.Hourly {
		prevTimes = append(prevTimes, h.Time)
	}
	idx := indexOfWorkingHour(prevTimes, fc.Current.Time)
	if idx < 0 {
		log.Printf("accuracy: no matching hour for %s observation %s (previous forecast for %s)", loc.Name, fc.Current.Time, today)
		return
	}
	observed := map[string]float64{
		"temperature":      fc.Current.TempC,
		"humidity":         float64(fc.Current.Humidity),
		"wind":             fc.Current.WindSpeedKmh,
		"rain_probability": float64(fc.Current.PrecipProb),
	}
	forecasted := map[string]float64{
		"temperature":      prev.Hourly[idx].TempC,
		"humidity":         float64(prev.Hourly[idx].Humidity),
		"wind":             prev.Hourly[idx].WindSpeedKmh,
		"rain_probability": float64(prev.Hourly[idx].PrecipProb),
	}
	recorded := 0
	for metric, o := range observed {
		f := forecasted[metric]
		if math.Abs(o) < 1 && math.Abs(f) < 1 {
			continue
		}
		// Relative error scales agreement linearly towards zero as discrepancies approach or exceed 100%.
		relErr := math.Abs(f-o) / math.Max(math.Abs(o), 1)
		pct := math.Max(0, math.Round((1-relErr)*1000)/10)
		if pct < 0 {
			pct = 0
		}
		var exists int
		_ = database.DB.QueryRow(`SELECT COUNT(*) FROM accuracy_samples WHERE location_key = ? AND date = ? AND metric = ?`,
			key, today, metric).Scan(&exists)
		if exists > 0 {
			continue
		}
		if _, err := database.DB.Exec(`INSERT INTO accuracy_samples(location_key,location_name,date,metric,forecast,observed,percent)
			VALUES(?,?,?,?,?,?,?)`, key, loc.Name, today, metric, Round1(f), Round1(o), pct); err != nil {
			log.Printf("accuracy: could not store %s sample for %s: %v", metric, loc.Name, err)
		} else {
			recorded++
		}
	}
	if recorded > 0 {
		log.Printf("accuracy: recorded %d sample(s) for %s on %s", recorded, loc.Name, today)
	}
}
