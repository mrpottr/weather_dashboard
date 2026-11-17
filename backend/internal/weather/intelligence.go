package weather

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"weather-forecast-database"
)

/*
Package weather implements deterministic activity scoring rubrics and plain-language meteorological advisories.
Deriving recommendations directly from verified physical forecast variables ensures transparent, auditable decision outputs.
*/

// ActivityProfile encapsulates meteorological sensitivity multipliers and temperature bounds for specific outdoor pursuits.
type ActivityProfile struct {
	Key, Name, Emoji          string
	WindMul, RainMul, HeatMul float64
	UVMul, ThunderMul, VisMul float64
	OvercastBonus, ClearBonus float64
	MinTemp, MaxTemp          float64 // comfortable band for this activity
}

// ActivityProfiles defines the active taxonomy of recreational pursuits evaluated by the recommendation engine.
var ActivityProfiles = []ActivityProfile{
	{"cycling", "Cycling", "🚴", 1.2, 1.15, 1.0, 1.1, 1.3, 1.0, 0, 2, 14, 27},
	{"beach-walk", "Beach walk", "🏖️", 1.1, 1.0, 1.1, 1.2, 1.3, 1.0, -2, 1, 18, 34},
	{"running", "Running", "🏃", 1.0, 1.0, 1.35, 1.1, 1.3, 1.0, 0, 2, 8, 26},
	{"hiking", "Hiking", "🥾", 1.15, 1.15, 1.0, 1.1, 1.4, 1.3, -2, 3, 8, 26},
	{"photography", "Photography", "📷", 0.8, 1.4, 0.9, 1.0, 1.2, 1.4, -3, 4, 2, 32},
	{"picnic", "Picnic", "🧺", 1.0, 1.6, 1.0, 1.2, 1.5, 1.0, -4, 3, 14, 30},
	{"fishing", "Fishing", "🎣", 1.1, 1.1, 1.0, 0.9, 1.3, 1.1, 3, 0, 8, 33},
	{"sightseeing", "Sightseeing", "🗺️", 1.0, 1.2, 1.0, 1.1, 1.3, 1.15, -1, 3, 8, 30},
	{"swimming", "Swimming", "🏊", 0.7, 0.8, 0.6, 1.0, 1.4, 1.0, 0, 2, 16, 36},
	{"camping", "Camping", "⛺", 1.15, 1.5, 1.1, 1.0, 1.6, 1.2, -3, 3, 6, 28},
}

// ActivityInputs collates representative daylight and daily maximum meteorological parameters required by the scoring rubric.
type ActivityInputs struct {
	Date          string
	Location      string
	HasData       bool
	TempC         float64 // representative daylight apparent temperature
	Daylight      int     // hours of daylight in the forecast day
	WindMax       float64
	GustMax       float64
	PrecipProb    int
	PrecipSum     float64
	UVMax         float64
	Humidity      int
	Visibility    float64
	CloudCover    int
	PressureAvg   float64
	PressureTrend float64 // hPa over 3 hours, from the hourly series
	Thunder       bool
	Hours         []HourlyPoint
	Sunrise       string
	Sunset        string
	MinTemp       float64
	MaxTemp       float64
}

// BuildInputs synthesizes day-length, pressure trajectories, and daytime temperature averages from raw hourly series.
func BuildInputs(fc *ForecastResponse, date string) ActivityInputs {
	in := ActivityInputs{Date: date, Location: fc.Location.Name}
	if date == "" {
		date = time.Now().In(database.LocTime(fc)).Format("2006-01-02")
		in.Date = date
	}
	var day *DailyForecast
	for i := range fc.Daily {
		if fc.Daily[i].Date == date {
			day = &fc.Daily[i]
			break
		}
	}
	if day == nil && len(fc.Daily) > 0 {
		// When the specified date lacks an exact daily match, defaulting to the earliest available day provides safe fallback data.
		day = &fc.Daily[0]
	}
	if day == nil {
		return in
	}
	in.HasData = true
	in.MinTemp, in.MaxTemp = day.TempMinC, day.TempMaxC
	in.WindMax, in.GustMax = day.WindMaxKmh, day.WindGustMaxKmh
	in.PrecipProb, in.PrecipSum = day.PrecipProbMax, day.PrecipSumMM
	in.UVMax, in.Humidity = day.UVIndexMax, day.HumidityAvg
	in.Visibility, in.CloudCover = day.VisibilityKm, day.CloudCoverAvg
	in.PressureAvg = day.PressureAvgHPa
	in.Sunrise, in.Sunset = day.Sunrise, day.Sunset

	prefix := date + "T"
	var apparentSum float64
	var pressureStart, pressureEnd float64
	var haveStart bool
	for _, h := range fc.Hourly {
		if !strings.HasPrefix(h.Time, prefix) {
			continue
		}
		in.Hours = append(in.Hours, h)
		if h.WeatherCode >= 95 {
			in.Thunder = true
		}
		if h.IsDay {
			in.Daylight++
			apparentSum += h.FeelsLikeC
			if !haveStart {
				pressureStart, haveStart = h.PressureHPa, true
			}
			pressureEnd = h.PressureHPa
		}
	}
	if in.Daylight > 0 {
		in.TempC = Round1(apparentSum / float64(in.Daylight))
	} else {
		in.TempC = Round1((day.ApparentMinC + day.ApparentMaxC) / 2)
	}
	if haveStart && pressureStart > 0 {
		in.PressureTrend = Round1((pressureEnd - pressureStart) / 3 * 3) // hPa over the daylight span, reported per 3h
		if in.Daylight > 3 {
			in.PressureTrend = Round1((pressureEnd - pressureStart) / float64(in.Daylight) * 3)
		}
	}
	return in
}

// ScoreActivity evaluates environmental inputs against activity-specific tolerances, accumulating weighted point adjustments.
func ScoreActivity(in ActivityInputs, p ActivityProfile) ActivityScore {
	out := ActivityScore{
		ActivityKey: p.Key, Activity: p.Name, Emoji: p.Emoji,
		Date: in.Date, Location: in.Location, Score: 100,
	}
	if !in.HasData {
		out.Score = 0
		out.Rating, out.Tag = "No data", "NO DATA"
		out.Title = p.Name + " cannot be rated yet."
		out.Summary = "No forecast data is available for " + in.Date + " in " + in.Location +
			". The score will appear as soon as the BOM forecast (or a cached copy) is available."
		return out
	}
	total := 100.0
	add := func(factor, reading string, points float64, reason string) {
		if points == 0 {
			return
		}
		out.Factors = append(out.Factors, ScoreFactor{Factor: factor, Reading: reading, Points: Round1(points), Reason: reason})
		total += points
	}
	t := in.TempC

	// Apparent daylight temperature scores thermal comfort, penalizing cold or extreme heat beyond profile thresholds.
	heat := 0.0
	switch {
	case t >= 40:
		heat = -28
	case t >= 35:
		heat = -18
	case t >= 30:
		heat = -9
	case t > 24:
		heat = -(t - 24) * 1.4
	case t >= 14:
		heat = 0
	case t >= 8:
		heat = -(14 - t) * 1.2
	case t >= 0:
		heat = -(14-t)*1.4 - 4
	default:
		heat = -22
	}
	heat *= p.HeatMul
	heatReason := "Daylight apparent temperature is inside the comfortable 14-24°C band."
	switch {
	case t >= 35:
		heatReason = "Heat-stress range (>=35°C feels-like): dehydration and heat exhaustion risk rises sharply."
	case t >= 30:
		heatReason = "Warm/heat-stress range (30-34°C feels-like): the body works harder to cool down."
	case t > 24:
		heatReason = fmt.Sprintf("Warmer than the %s comfort ceiling of %.0f°C.", p.Name, p.MaxTemp)
	case t < 14:
		heatReason = fmt.Sprintf("Colder than the %s comfort floor of %.0f°C.", p.Name, p.MinTemp)
	}
	add("Temperature", fmt.Sprintf("%.0f°C feels-like (daylight mean)", t), heat, heatReason)

	// Precipitation probabilities and expected accumulation volumes penalize activities vulnerable to wet gear or slippery ground.
	rain := 0.0
	switch {
	case in.PrecipProb >= 80:
		rain = -24
	case in.PrecipProb >= 60:
		rain = -16
	case in.PrecipProb >= 40:
		rain = -9
	case in.PrecipProb >= 20:
		rain = -4
	default:
		rain = 3
	}
	amount := 0.0
	switch {
	case in.PrecipSum >= 10:
		amount = -12
	case in.PrecipSum >= 5:
		amount = -6
	case in.PrecipSum >= 1:
		amount = -3
	}
	add("Rain probability", fmt.Sprintf("%d%% (day maximum)", in.PrecipProb), rain*p.RainMul,
		"Probability band applied from the deterministic rubric; below 20% is a bonus.")
	add("Rainfall amount", fmt.Sprintf("%.1f mm forecast total", in.PrecipSum), amount*p.RainMul,
		"Wet ground and soaked gear reduce comfort and safety for outdoor activity.")

	// Sustained winds and peak gusts apply Beaufort-aligned safety penalties scaled by activity drag factors.
	wind := 0.0
	windBand := ""
	switch {
	case in.WindMax > 61:
		wind, windBand = -30, "storm-force band (>61 km/h)"
	case in.WindMax >= 51:
		wind, windBand = -20, "gale band (51-61 km/h)"
	case in.WindMax >= 40:
		wind, windBand = -13, "near-gale band (40-50 km/h)"
	case in.WindMax >= 30:
		wind, windBand = -7, "strong-breeze band (30-39 km/h)"
	case in.WindMax >= 12:
		wind, windBand = 0, "moderate band (12-29 km/h)"
	default:
		wind, windBand = 2, "light-air band (<12 km/h)"
	}
	add("Wind speed", fmt.Sprintf("%.0f km/h max, %s", in.WindMax, windBand), wind*p.WindMul,
		"Beaufort-style band for sustained wind; the activity profile multiplies the impact.")
	gust := 0.0
	gustNote := ""
	if in.GustMax >= 80 {
		gust, gustNote = -14, "Gusts are storm-level (>=80 km/h): control and balance are severely affected."
	} else if in.GustMax >= 65 {
		gust, gustNote = -8, "Gusts reach 65 km/h or more: sudden loss of control is possible."
	}
	if gust != 0 {
		add("Wind gusts", fmt.Sprintf("%.0f km/h peak gust", in.GustMax), gust*p.WindMul, gustNote)
	}

	// Maximum daytime UV index incorporates WHO risk bands to adjust recommendations based on sun exposure vulnerability.
	uvCat, uvPenalty := UVCategory(in.UVMax)
	uvReason := fmt.Sprintf("UV %s in the %s category; sun protection is advised for exposed skin.", uvLabel(in.UVMax), uvCat)
	if in.UVMax < 3 {
		uvReason = "UV is in the low category; no sun protection is required."
	}
	add("UV index", fmt.Sprintf("%.0f max · %s", in.UVMax, uvCat), uvPenalty*p.UVMul, uvReason)

	// Relative humidity extremes penalize muggy atmospheres that impede natural evaporative cooling or excessively arid conditions.
	if in.Humidity > 85 {
		add("Humidity", fmt.Sprintf("%d%% daily mean", in.Humidity), -5, "Muggy air (>85%) reduces evaporative cooling and comfort.")
	} else if in.Humidity < 30 {
		add("Humidity", fmt.Sprintf("%d%% daily mean", in.Humidity), -3, "Very dry air (<30%) increases dehydration and eye irritation.")
	}

	// Atmospheric visibility penalties reflect navigational difficulty and compromised situational awareness in fog or haze.
	switch {
	case in.Visibility < 1:
		add("Visibility", fmt.Sprintf("%.1f km", in.Visibility), -12*p.VisMul, "Dense fog (<1 km): navigation and safety are compromised.")
	case in.Visibility < 5:
		add("Visibility", fmt.Sprintf("%.1f km", in.Visibility), -6*p.VisMul, "Reduced visibility (<5 km): expect haze or mist.")
	case in.Visibility >= 20:
		add("Visibility", fmt.Sprintf("%.1f km", in.Visibility), 1*p.VisMul, "Excellent clarity (>=20 km) improves views and navigation.")
	}

	// Thunderstorm detection applies severe deductions to prioritize personal safety during lightning or sudden squall hazards.
	if in.Thunder {
		add("Thunderstorm", "Forecast codes 95/96/99 present", -28*p.ThunderMul,
			"Thunderstorms bring lightning, sudden downpours and gust fronts: shelter is required.")
	}

	// Barometric pressure shifts detect rapid frontal movements and unstable meteorological transitions.
	if in.PressureTrend <= -1.5 && in.PressureAvg < 1010 {
		add("Pressure trend", fmt.Sprintf("%.1f hPa falling (avg %.0f hPa)", in.PressureTrend, in.PressureAvg), -6,
			"Pressure is falling while below 1010 hPa: a weather front is likely approaching.")
	} else if in.PressureTrend >= 1 {
		add("Pressure trend", fmt.Sprintf("%.1f hPa rising (avg %.0f hPa)", in.PressureTrend, in.PressureAvg), 2,
			"Rising pressure indicates improving or stable conditions.")
	}

	// Cloud cover scoring rewards pursuits that benefit from direct sunlight while respecting activities with shade preferences.
	if in.CloudCover > 90 {
		add("Cloud cover", fmt.Sprintf("%d%% mean", in.CloudCover), -3+p.OvercastBonus, "Fully overcast sky; light levels and morale drop for some activities.")
	} else if in.CloudCover < 15 {
		add("Cloud cover", fmt.Sprintf("%d%% mean", in.CloudCover), 2+p.ClearBonus, "Very clear sky; strong light and long views.")
	}

	// Total score is clamped to a standard 0-100 range and sorted by factor impact magnitude for rapid auditing.
	out.Score = int(math.Round(math.Max(0, math.Min(100, total))))
	sort.SliceStable(out.Factors, func(i, j int) bool {
		return math.Abs(out.Factors[i].Points) > math.Abs(out.Factors[j].Points)
	})
	out.Score, out.Rating, out.Tag = rateScore(out.Score)
	out.Title = fmt.Sprintf("%s is looking %s in %s.", p.Name, ratingWord(out.Rating), in.Location)
	out.Summary = buildSummary(in, p, out.Score, out.Rating)
	out.Advice = buildAdvice(in, p)
	out.BestWindow, out.BestHours = bestWindow(in, p)
	return out
}

// rateScore partitions numeric scores into qualitative rating bands for UI tags and status styling.
func rateScore(score int) (int, string, string) {
	switch {
	case score >= 80:
		return score, "Excellent", "GOOD OUTLOOK"
	case score >= 65:
		return score, "Good", "GOOD OUTLOOK"
	case score >= 50:
		return score, "Fair", "CAUTION"
	case score >= 35:
		return score, "Poor", "CAUTION"
	default:
		return score, "Bad", "POOR"
	}
}

func ratingWord(r string) string {
	switch r {
	case "Excellent":
		return "excellent"
	case "Good":
		return "good"
	case "Fair":
		return "mixed"
	case "Poor":
		return "poor"
	default:
		return "unsuitable"
	}
}

// UVCategory classifies UV index values according to Cancer Council health risk bands and determines penalty points.
func UVCategory(uv float64) (string, float64) {
	switch {
	case uv >= 11:
		return "extreme", -11
	case uv >= 8:
		return "very high", -7
	case uv >= 6:
		return "high", -3
	case uv >= 3:
		return "moderate", 0
	default:
		return "low", 0
	}
}

func uvLabel(uv float64) string { return fmt.Sprintf("%.0f", uv) }

// buildSummary synthesizes evaluated meteorological observations into an audit-friendly plain-language rationale.
func buildSummary(in ActivityInputs, p ActivityProfile, score int, rating string) string {
	uvCat, _ := UVCategory(in.UVMax)
	parts := []string{
		fmt.Sprintf("On %s in %s the forecast daylight feels-like temperature averages %.0f°C (day %.0f-%.0f°C)",
			in.Date, in.Location, in.TempC, in.MinTemp, in.MaxTemp),
		fmt.Sprintf("wind peaks at %.0f km/h with gusts to %.0f km/h", in.WindMax, in.GustMax),
		fmt.Sprintf("rain probability is %d%% with %.1f mm expected", in.PrecipProb, in.PrecipSum),
		fmt.Sprintf("UV reaches %.0f (%s)", in.UVMax, uvCat),
		fmt.Sprintf("humidity averages %d%%, cloud %d%% and visibility %.0f km", in.Humidity, in.CloudCover, in.Visibility),
	}
	pressureNote := fmt.Sprintf("surface pressure averages %.0f hPa", in.PressureAvg)
	if in.PressureTrend != 0 {
		pressureNote += fmt.Sprintf(" and is trending %.1f hPa per 3 hours", in.PressureTrend)
	}
	parts = append(parts, pressureNote)
	body := strings.Join(parts, ", ") + ". "
	switch rating {
	case "Excellent", "Good":
		body += fmt.Sprintf("Those values sit inside the comfortable band for %s, so the rubric awards a score of %d/100.", p.Name, score)
	case "Fair":
		body += fmt.Sprintf("Some values fall outside the comfortable band for %s, giving a score of %d/100 — plan around the weaker factors.", p.Name, score)
	default:
		body += fmt.Sprintf("Several values are outside the safe band for %s, giving a score of %d/100 — reschedule if you can.", p.Name, score)
	}
	if in.Thunder {
		body += " Thunderstorm codes are present in the hourly forecast, which is the single largest penalty in the rubric."
	}
	return body
}

// buildAdvice produces actionable safety precautions and clothing guidance derived from active meteorological risk factors.
func buildAdvice(in ActivityInputs, p ActivityProfile) []string {
	advice := []string{}
	if in.UVMax >= 6 {
		cat, _ := UVCategory(in.UVMax)
		advice = append(advice, fmt.Sprintf("Sun protection: UV %.0f (%s). Use SPF50+, a hat and sunglasses, especially 10:00-16:00.", in.UVMax, cat))
	}
	switch {
	case in.TempC >= 35:
		advice = append(advice, fmt.Sprintf("Heat safety: feels-like %.0f°C. Avoid exertion in the afternoon, take shade every 30 minutes and drink water before you feel thirsty.", in.TempC))
	case in.TempC >= 30:
		advice = append(advice, fmt.Sprintf("Hydration: feels-like %.0f°C. Carry at least 500 ml per hour and use light, breathable clothing.", in.TempC))
	case in.TempC < 8:
		advice = append(advice, fmt.Sprintf("Cold protection: feels-like %.0f°C. Dress in warm layers with wind protection and cover extremities.", in.TempC))
	case in.TempC < 14:
		advice = append(advice, fmt.Sprintf("Dress warmly: feels-like %.0f°C. A mid-layer plus a wind shell will keep you comfortable.", in.TempC))
	default:
		advice = append(advice, fmt.Sprintf("Clothing: a light layer matches the %.0f°C feels-like temperature.", in.TempC))
	}
	if in.PrecipProb >= 50 || in.PrecipSum >= 5 {
		advice = append(advice, fmt.Sprintf("Rain cover: %d%% probability and %.1f mm expected — take waterproofs and protect electronics.", in.PrecipProb, in.PrecipSum))
	}
	if in.WindMax >= 40 {
		advice = append(advice, fmt.Sprintf("Wind warning: gusts to %.0f km/h. Secure loose gear and avoid exposed ridgelines, coastlines or tall structures.", in.GustMax))
	}
	if in.Thunder {
		advice = append(advice, "Safety: thunderstorms are forecast. Move to a substantial building or hard-topped vehicle at the first rumble of thunder.")
	}
	if in.Humidity > 85 {
		advice = append(advice, fmt.Sprintf("Muggy air at %d%% humidity — moisture-wicking clothing will reduce chafing and overheating.", in.Humidity))
	}
	if in.Visibility < 5 {
		advice = append(advice, fmt.Sprintf("Low visibility at %.1f km — slow down, use lights and keep extra distance.", in.Visibility))
	}
	advice = append(advice, fmt.Sprintf("Best time of day: %s (local time for %s).", bestWindowLabel(in, p), in.Location))
	return advice
}

// bestWindow scans hourly forecasts to identify the longest continuous block meeting environmental comfort limits.
func bestWindow(in ActivityInputs, p ActivityProfile) (string, []string) {
	if len(in.Hours) == 0 {
		return "No hourly data for this date", nil
	}
	good := func(h HourlyPoint) bool {
		if h.WeatherCode >= 95 {
			return false
		}
		if !h.IsDay {
			return false
		}
		if h.PrecipProb > 40 || h.PrecipMM > 0.4 {
			return false
		}
		if h.WindSpeedKmh > 40 {
			return false
		}
		if h.FeelsLikeC < p.MinTemp || h.FeelsLikeC > p.MaxTemp {
			return false
		}
		if h.UVIndex >= 8 && p.Key != "swimming" {
			return false
		}
		return true
	}
	var bestStart, bestEnd, curStart, curLen, bestLen int = -1, -1, -1, 0, 0
	for i, h := range in.Hours {
		if good(h) {
			if curStart < 0 {
				curStart = i
				curLen = 0
			}
			curLen++
			if curLen > bestLen {
				bestLen, bestStart, bestEnd = curLen, curStart, i
			}
		} else {
			curStart, curLen = -1, 0
		}
	}
	if bestStart < 0 || bestLen < 1 {
		return "No comfortable outdoor window on this day", nil
	}
	startH := hourLabel(in.Hours[bestStart].Time)
	endH := hourLabel(in.Hours[bestEnd].Time)
	hours := make([]string, 0, bestLen)
	for i := bestStart; i <= bestEnd; i++ {
		hours = append(hours, hourLabel(in.Hours[i].Time))
	}
	return startH + "-" + endH, hours
}

func bestWindowLabel(in ActivityInputs, p ActivityProfile) string {
	w, _ := bestWindow(in, p)
	return w
}

// hourLabel trims ISO-8601 timestamps to concise clock times for dense schedule displays.
func hourLabel(iso string) string {
	if len(iso) < 16 {
		return iso
	}
	return iso[11:16]
}
