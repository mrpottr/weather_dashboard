// Package database provides historical forecast logging and file mirroring mechanisms.
// It persists query executions to SQLite while streaming serialized CSV and JSON snapshots for external audits.
package database

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// DataDir specifies the root directory for generated CSV and JSON operational export files.
var DataDir = "data"

func historyDir() string {
	return DataDir
}

// RecordForecastHistory logs forecast lookups into SQLite and updates corresponding CSV and JSON exports.
// This dual persistence guarantees both relational queryability and external data portability.
func RecordForecastHistory(u *User, fc *ForecastResponse, date string) {
	if fc == nil || DB == nil {
		return
	}
	if date == "" {
		date = time.Now().In(LocTime(fc)).Format("2006-01-02")
	}
	source := fc.Source
	if fc.FromCache {
		source = "Local cache (BOM unavailable)"
	}
	payload, _ := json.Marshal(fc)
	var uid interface{}
	if u != nil {
		uid = u.ID
	}
	result := "retrieved"
	if fc.FromCache {
		result = "cached"
	}
	_, err := DB.Exec(`INSERT INTO forecast_history(user_id,location_name,location_key,date,source,from_cache,result,payload)
		VALUES(?,?,?,?,?,?,?,?)`,
		uid, fc.Location.Name, CacheKey(fc.Location), date, source, BoolToInt(fc.FromCache), result, string(payload))
	if err != nil {
		return
	}
	// Append the new observation row to the persistent CSV export log.
	if f, err := os.OpenFile(filepath.Join(historyDir(), "forecast_history.csv"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		stat, _ := f.Stat()
		w := csv.NewWriter(f)
		if stat != nil && stat.Size() == 0 {
			_ = w.Write([]string{"timestamp", "location", "date", "result", "source", "temp_min_c", "temp_max_c", "rain_probability"})
		}
		minT, maxT, rain := "", "", ""
		if len(fc.Daily) > 0 {
			for _, d := range fc.Daily {
				if d.Date == date {
					minT = fmt.Sprintf("%.1f", d.TempMinC)
					maxT = fmt.Sprintf("%.1f", d.TempMaxC)
					rain = strconv.Itoa(d.PrecipProbMax)
					break
				}
			}
		}
		_ = w.Write([]string{time.Now().Format("2006-01-02 15:04:05"), fc.Location.Name, date, result, source, minT, maxT, rain})
		w.Flush()
		f.Close()
	}
	WriteHistoryFiles()
}

// WriteHistoryFiles flushes the latest forecast records and search queries to structured files on disk.
// This synchronizes filesystem artifacts after database write or retention cleanup operations.
func WriteHistoryFiles() {
	if DB == nil {
		return
	}
	// Export recent forecast history snapshots into structured JSON format.
	rows, err := DB.Query(`SELECT id,user_id,location_name,date,source,from_cache,result,ts
		FROM forecast_history ORDER BY id DESC LIMIT 500`)
	if err == nil {
		type histRow struct {
			ID        int64  `json:"id"`
			UserID    *int64 `json:"user_id"`
			Location  string `json:"location"`
			Date      string `json:"date"`
			Source    string `json:"source"`
			FromCache bool   `json:"from_cache"`
			Result    string `json:"result"`
			TS        string `json:"timestamp"`
		}
		out := []histRow{}
		for rows.Next() {
			var r histRow
			var uid *int64
			var fromCache int
			var ts string
			if err := rows.Scan(&r.ID, &uid, &r.Location, &r.Date, &r.Source, &fromCache, &r.Result, &ts); err == nil {
				r.UserID = uid
				r.FromCache = fromCache == 1
				r.TS = ts
				out = append(out, r)
			}
		}
		rows.Close()
		if b, err := json.MarshalIndent(out, "", "  "); err == nil {
			_ = os.WriteFile(filepath.Join(historyDir(), "forecast_history.json"), b, 0o644)
		}
	}
	// Rebuild query search history CSV files to reflect current database entries.
	if qrows, err := DB.Query(`SELECT id,user_id,query_text,result_count,ts FROM query_history ORDER BY id DESC LIMIT 1000`); err == nil {
		if f, err := os.Create(filepath.Join(historyDir(), "query_history.csv")); err == nil {
			w := csv.NewWriter(f)
			_ = w.Write([]string{"id", "user_id", "query", "results", "timestamp"})
			for qrows.Next() {
				var id, count int
				var uid *int64
				var text, ts string
				if err := qrows.Scan(&id, &uid, &text, &count, &ts); err == nil {
					uidStr := ""
					if uid != nil {
						uidStr = strconv.FormatInt(*uid, 10)
					}
					_ = w.Write([]string{strconv.Itoa(id), uidStr, text, strconv.Itoa(count), ts})
				}
			}
			w.Flush()
			f.Close()
		}
		qrows.Close()
	}
}
