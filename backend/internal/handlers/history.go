package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"weather-forecast-database"
	"weather-forecast-system/internal/weather"
)

/*
Package handlers aggregates historical user interactions and meteorological trend observations.
Providing unified timelines and longitudinal statistics allows users to evaluate seasonal trends and system forecasting reliability.
*/

// ListHistoryHandler aggregates search queries and past forecast lookups into a unified chronological audit trail.
func ListHistoryHandler(c *gin.Context) {
	u, ok := requireAuthCtx(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := strings.ToLower(strings.TrimSpace(c.Query("q")))
	type row struct {
		ID       int64  `json:"id"`
		Kind     string `json:"kind"`
		TS       string `json:"ts"`
		Location string `json:"location"`
		Date     string `json:"date"`
		Result   string `json:"result"`
		Source   string `json:"source"`
		Query    string `json:"query"`
	}
	out := []row{}
	rows, err := database.DB.Query(`SELECT id,location_name,date,source,result,ts FROM forecast_history
		WHERE user_id = ? ORDER BY id DESC LIMIT ?`, u.ID, limit)
	if err == nil {
		for rows.Next() {
			var r row
			var ts string
			if err := rows.Scan(&r.ID, &r.Location, &r.Date, &r.Source, &r.Result, &ts); err == nil {
				r.Kind = "forecast"
				r.TS = ts
				if q == "" || strings.Contains(strings.ToLower(r.Location), q) || strings.Contains(strings.ToLower(r.Result), q) {
					out = append(out, r)
				}
			}
		}
		rows.Close()
	}
	qrows, err := database.DB.Query(`SELECT id,query_text,result_count,ts FROM query_history WHERE user_id = ? ORDER BY id DESC LIMIT ?`, u.ID, limit)
	if err == nil {
		for qrows.Next() {
			var r row
			var ts string
			var count int
			if err := qrows.Scan(&r.ID, &r.Query, &count, &ts); err == nil {
				r.Kind = "query"
				r.TS = ts
				r.Result = fmt.Sprintf("%d results", count)
				r.Source = "Location search"
				r.Location = r.Query
				if q == "" || strings.Contains(strings.ToLower(r.Query), q) {
					out = append(out, r)
				}
			}
		}
		qrows.Close()
	}
	c.JSON(http.StatusOK, gin.H{"entries": out, "retention_days": database.GetThresholds().RetentionDays})
}

// HistoryStatsHandler derives rolling temperature averages, rain trajectory percentages, and historical accuracy scores.
func HistoryStatsHandler(c *gin.Context) {
	u := currentUser(c)
	report := gin.H{"mean_temp_c": nil, "rain_trend_pct": nil, "accuracy": nil}
	if u == nil {
		c.JSON(http.StatusOK, report)
		return
	}
	/*
	Aggregating stored forecast payloads calculates rolling mean temperatures without triggering external API calls.
	Sampling up to 30 past entries provides sufficient statistical significance while preserving snappy response times.
	*/
	rows, err := database.DB.Query(`SELECT payload FROM forecast_history WHERE user_id = ? ORDER BY id DESC LIMIT 30`, u.ID)
	if err == nil {
		var sum, n float64
		for rows.Next() {
			var payload string
			if rows.Scan(&payload) != nil {
				continue
			}
			fc := weather.DecodeForecast([]byte(payload))
			if fc == nil || len(fc.Daily) == 0 {
				continue
			}
			for _, d := range fc.Daily {
				sum += (d.TempMinC + d.TempMaxC) / 2
				n++
			}
		}
		rows.Close()
		if n > 0 {
			report["mean_temp_c"] = weather.Round1(sum / n)
		}
	}
	// avgRain computes rolling precipitation probabilities across offset windows to detect weekly shifts in rainfall intensity.
	avgRain := func(offset, count int) *float64 {
		r, err := database.DB.Query(`SELECT payload FROM forecast_history WHERE user_id = ? ORDER BY id DESC LIMIT ? OFFSET ?`, u.ID, count, offset)
		if err != nil {
			return nil
		}
		defer r.Close()
		var sum, n float64
		for r.Next() {
			var payload string
			if r.Scan(&payload) != nil {
				continue
			}
			fc := weather.DecodeForecast([]byte(payload))
			if fc == nil || len(fc.Daily) == 0 {
				continue
			}
			sum += float64(fc.Daily[0].PrecipProbMax)
			n++
		}
		if n == 0 {
			return nil
		}
		v := weather.Round1(sum / n)
		return &v
	}
	if a := avgRain(0, 7); a != nil {
		if b := avgRain(7, 7); b != nil && *b > 0 {
			report["rain_trend_pct"] = weather.Round1((*a - *b) / *b * 100)
		}
	}
	// Historical accuracy metrics quantify empirical forecast deviation across the user's queried locations.
	var acc float64
	var count int
	_ = database.DB.QueryRow(`SELECT AVG(percent), COUNT(*) FROM accuracy_samples WHERE location_key IN
		(SELECT DISTINCT location_key FROM forecast_history WHERE user_id = ?)`, u.ID).Scan(&acc, &count)
	if count > 0 {
		report["accuracy"] = weather.Round1(acc)
		report["accuracy_samples"] = count
	}
	c.JSON(http.StatusOK, report)
}
