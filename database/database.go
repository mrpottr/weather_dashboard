// Package database encapsulates SQLite persistence, connection lifecycle, and schema migration logic.
// It applies parameterized execution and WAL-mode concurrency controls across all dashboard entities.
package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

// DB holds the shared SQLite connection pool configured for thread-safe operations.
var DB *sql.DB

// Open initializes the SQLite storage engine with WAL journaling and executes schema migrations.
// It limits open connections to serialize write operations and prevent lock contention.
func Open(path string) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0o755) // Ensure parent directories exist before creating database files.
	}
	handle, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	handle.SetMaxOpenConns(1) // Single writer model prevents concurrent lock failures in SQLite.
	if err := handle.Ping(); err != nil {
		return fmt.Errorf("ping sqlite: %w", err)
	}
	DB = handle
	if err := migrate(); err != nil {
		return err
	}
	return seed()
}

// migrate creates required application tables idempotently during startup.
func migrate() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			first_name TEXT NOT NULL,
			last_name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'standard',
			status TEXT NOT NULL DEFAULT 'active',
			default_location TEXT NOT NULL DEFAULT 'Melbourne, VIC',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_active_at TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS user_preferences (
			user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
			theme TEXT NOT NULL DEFAULT 'dark',
			clock_format TEXT NOT NULL DEFAULT '24h'
		)`,
		`CREATE TABLE IF NOT EXISTS locations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			latitude REAL NOT NULL,
			longitude REAL NOT NULL,
			timezone TEXT NOT NULL DEFAULT 'Australia/Sydney',
			admin_area TEXT NOT NULL DEFAULT '',
			saved_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS weather_cache (
			cache_key TEXT PRIMARY KEY,
			location_name TEXT NOT NULL,
			payload TEXT NOT NULL,
			fetched_at TIMESTAMP NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			hits INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS thresholds (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			high_temp_c REAL NOT NULL DEFAULT 38,
			low_temp_c REAL NOT NULL DEFAULT 4,
			rain_pct INTEGER NOT NULL DEFAULT 80,
			wind_level TEXT NOT NULL DEFAULT 'strong',
			uv_index REAL NOT NULL DEFAULT 8,
			push_enabled INTEGER NOT NULL DEFAULT 1,
			retention_days INTEGER NOT NULL DEFAULT 365,
			keep_cache INTEGER NOT NULL DEFAULT 1,
			retain_history INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE TABLE IF NOT EXISTS announcements (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			message TEXT NOT NULL,
			author_id INTEGER,
			author TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP,
			active INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE TABLE IF NOT EXISTS announcement_dismissals (
			announcement_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			PRIMARY KEY (announcement_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS system_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ts TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			actor TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL,
			detail TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS forecast_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			location_name TEXT NOT NULL,
			location_key TEXT NOT NULL DEFAULT '',
			date TEXT NOT NULL,
			source TEXT NOT NULL,
			from_cache INTEGER NOT NULL DEFAULT 0,
			result TEXT NOT NULL DEFAULT 'retrieved',
			payload TEXT NOT NULL DEFAULT '{}',
			ts TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS query_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			query_text TEXT NOT NULL,
			result_count INTEGER NOT NULL DEFAULT 0,
			ts TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS accuracy_samples (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			location_key TEXT NOT NULL,
			location_name TEXT NOT NULL,
			date TEXT NOT NULL,
			metric TEXT NOT NULL,
			forecast REAL NOT NULL,
			observed REAL NOT NULL,
			percent REAL NOT NULL,
			ts TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS weather_alerts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			location_name TEXT NOT NULL,
			alert_type TEXT NOT NULL,
			date TEXT NOT NULL,
			dismissed INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, location_name, alert_type, date)
		)`,
		`CREATE TABLE IF NOT EXISTS images_cache (
			query TEXT PRIMARY KEY,
			provider TEXT NOT NULL,
			urls TEXT NOT NULL,
			credit TEXT NOT NULL DEFAULT '',
			fetched_at TIMESTAMP NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_hist_user ON forecast_history(user_id, ts)`,
		`CREATE INDEX IF NOT EXISTS idx_hist_loc ON forecast_history(location_key, ts)`,
		`CREATE INDEX IF NOT EXISTS idx_query_user ON query_history(user_id, ts)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_ts ON system_logs(ts)`,
		`CREATE INDEX IF NOT EXISTS idx_accuracy_loc ON accuracy_samples(location_key, ts)`,
		`CREATE INDEX IF NOT EXISTS idx_alerts_user ON weather_alerts(user_id, dismissed)`,
	}
	for _, stmt := range statements {
		if _, err := DB.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

// seed provisions default system accounts and threshold limits if the database is unpopulated.
// This ensures that fresh deployments provide out-of-the-box administrator access and operational baselines.
func seed() error {
	var n int
	if err := DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		adminHash, err := HashPassword("admin123")
		if err != nil {
			return err
		}
		userHash, err := HashPassword("user123")
		if err != nil {
			return err
		}
		seedUsers := []struct {
			first, last, email, hash, role, defLoc string
		}{
			{"System", "Administrator", "admin@weather.local", adminHash, RoleAdmin, "All Australia"},
			{"Minindu", "Demo", "user@weather.local", userHash, RoleStandard, "Melbourne, VIC"},
		}
		for _, u := range seedUsers {
			res, err := DB.Exec(`INSERT INTO users(first_name,last_name,email,password_hash,role,status,default_location)
				VALUES(?,?,?,?,?, 'active', ?)`, u.first, u.last, u.email, u.hash, u.role, u.defLoc)
			if err != nil {
				return fmt.Errorf("seed user: %w", err)
			}
			id, _ := res.LastInsertId()
			if _, err := DB.Exec(`INSERT INTO user_preferences(user_id,theme,clock_format) VALUES(?,?,?)`, id, "dark", "24h"); err != nil {
				return err
			}
			if u.role == RoleStandard {
				// Seed starter locations to populate the dashboard UI on initial demo login.
				for _, loc := range []LocationRef{Melbourne, Sydney, Hobart} {
					_, _ = DB.Exec(`INSERT OR IGNORE INTO locations(user_id,name,latitude,longitude,timezone,admin_area)
						VALUES(?,?,?,?,?,?)`, id, loc.Name, loc.Latitude, loc.Longitude, loc.Timezone, loc.AdminArea)
				}
			}
		}
		log.Printf("seeded default accounts: admin@weather.local / admin123, user@weather.local / user123")
	}
	var tn int
	if err := DB.QueryRow(`SELECT COUNT(*) FROM thresholds`).Scan(&tn); err != nil {
		return err
	}
	if tn == 0 {
		if _, err := DB.Exec(`INSERT INTO thresholds(id) VALUES(1)`); err != nil {
			return err
		}
	}
	return nil
}

// HashPassword generates bcrypt hashes to safeguard stored user credentials.
func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

// CheckPassword verifies plaintext inputs against salted bcrypt hashes in constant time.
func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// GetThresholds retrieves alert boundaries and retention rules from the singleton configuration row.
func GetThresholds() Thresholds {
	th := Thresholds{HighTempC: 38, LowTempC: 4, RainPct: 80, WindLevel: "strong", UVIndex: 8,
		PushEnabled: true, RetentionDays: 365, KeepCache: true, RetainHistory: true}
	if DB == nil {
		return th
	}
	var keep, retain, push int
	err := DB.QueryRow(`SELECT high_temp_c,low_temp_c,rain_pct,wind_level,uv_index,push_enabled,retention_days,keep_cache,retain_history
		FROM thresholds WHERE id = 1`).
		Scan(&th.HighTempC, &th.LowTempC, &th.RainPct, &th.WindLevel, &th.UVIndex, &push, &th.RetentionDays, &keep, &retain)
	if err != nil {
		return th
	}
	th.PushEnabled = push == 1
	th.KeepCache = keep == 1
	th.RetainHistory = retain == 1
	return th
}

// LogSystem records audit events in the system log table for compliance and administrative tracking.
func LogSystem(actor, role, action, detail string) {
	if DB == nil {
		return
	}
	_, err := DB.Exec(`INSERT INTO system_logs(actor,role,action,detail) VALUES(?,?,?,?)`,
		actor, role, action, detail)
	if err != nil {
		log.Printf("system log write failed: %v", err)
	}
}

// LogSystemForUser captures security events attributing authenticated user contexts.
func LogSystemForUser(u *User, action, detail string) {
	if u == nil {
		LogSystem("anonymous", "", action, detail)
		return
	}
	LogSystem(u.Email, u.Role, action, detail)
}

// CleanupRetention purges historical forecasts exceeding configured age limits and mirrors changes to disk.
// This prevents unbounded storage growth while maintaining compliance with retention policies.
func CleanupRetention() {
	th := GetThresholds()
	if !th.RetainHistory || th.RetentionDays <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -th.RetentionDays).Format(time.RFC3339)
	if _, err := DB.Exec(`DELETE FROM forecast_history WHERE ts < ?`, cutoff); err != nil {
		log.Printf("retention cleanup: %v", err)
		return
	}
	if _, err := DB.Exec(`DELETE FROM query_history WHERE ts < ?`, cutoff); err != nil {
		log.Printf("retention cleanup: %v", err)
		return
	}
	WriteHistoryFiles()
}
