package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"weather-forecast-database"
	"weather-forecast-system/internal/auth"
	"weather-forecast-system/internal/supports"
	"weather-forecast-system/internal/weather"
)

/*
Package handlers provides administrative management across security policies, account lifecycles, and system telemetry.
Centralizing administrative privileges here protects privileged mutations and enforces operational safety invariants.
*/

// GetThresholdsHandler exposes active hazard limits and retention policies to the administrator dashboard.
func GetThresholdsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, database.GetThresholds())
}

// PutThresholdsHandler updates safety bounds and triggers data pruning to enforce database retention policies.
func PutThresholdsHandler(c *gin.Context) {
	u, ok := requireAuthCtx(c)
	if !ok {
		return
	}
	var th Thresholds
	if err := c.ShouldBindJSON(&th); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid threshold payload.", Code: "validation"})
		return
	}
	if th.HighTempC < -10 || th.HighTempC > 55 || th.LowTempC < -20 || th.LowTempC > 40 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Temperature thresholds are outside the plausible range.", Code: "validation"})
		return
	}
	if th.RainPct < 0 || th.RainPct > 100 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Rain threshold must be between 0 and 100.", Code: "validation"})
		return
	}
	switch th.WindLevel {
	case "strong", "gale", "storm":
	default:
		th.WindLevel = "strong"
	}
	if th.UVIndex <= 0 || th.UVIndex > 16 {
		th.UVIndex = 8
	}
	if th.RetentionDays < 7 || th.RetentionDays > 3650 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Retention must be between 7 and 3650 days.", Code: "validation"})
		return
	}
	_, err := database.DB.Exec(`UPDATE thresholds SET high_temp_c=?,low_temp_c=?,rain_pct=?,wind_level=?,uv_index=?,
		push_enabled=?,retention_days=?,keep_cache=?,retain_history=? WHERE id = 1`,
		th.HighTempC, th.LowTempC, th.RainPct, th.WindLevel, th.UVIndex,
		database.BoolToInt(th.PushEnabled), th.RetentionDays, database.BoolToInt(th.KeepCache), database.BoolToInt(th.RetainHistory))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Could not save thresholds.", Code: "server_error"})
		return
	}
	database.LogSystemForUser(u, "admin.thresholds", fmt.Sprintf("Thresholds updated (high %.0f°C, low %.0f°C, rain %d%%, wind %s, UV %.0f, retention %dd)",
		th.HighTempC, th.LowTempC, th.RainPct, th.WindLevel, th.UVIndex, th.RetentionDays))
	if th.RetainHistory {
		database.CleanupRetention()
	}
	c.JSON(http.StatusOK, database.GetThresholds())
}

// ListUsersHandler returns user records matching search criteria to enable administrative account auditing.
func ListUsersHandler(c *gin.Context) {
	q := "%" + strings.ToLower(strings.TrimSpace(c.Query("q"))) + "%"
	role := c.Query("role")     // "", standard, admin
	status := c.Query("status") // "", active, suspended, deactivated
	query := `SELECT id,first_name,last_name,email,role,status,default_location,created_at,COALESCE(last_active_at,'')
		FROM users WHERE (LOWER(first_name||' '||last_name) LIKE ? OR LOWER(email) LIKE ?)`
	args := []interface{}{q, q}
	if role != "" {
		query += ` AND role = ?`
		args = append(args, role)
	}
	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY id`
	rows, err := database.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Could not load users.", Code: "server_error"})
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int64
		var first, lastName, email, r, st, defLoc, created, lastActive string
		if err := rows.Scan(&id, &first, &lastName, &email, &r, &st, &defLoc, &created, &lastActive); err != nil {
			continue
		}
		lastSeen := "never"
		if lastActive != "" {
			lastSeen = supports.HumanSince(database.ParseTime(lastActive))
		}
		out = append(out, gin.H{
			"id": id, "first_name": first, "last_name": lastName, "email": email, "role": r, "status": st,
			"location": defLoc, "created_at": created, "last_active": lastSeen,
		})
	}
	c.JSON(http.StatusOK, gin.H{"users": out})
}

// UpdateUserStatusHandler mutates account status while strictly prohibiting self-modification to prevent admin lockout.
func UpdateUserStatusHandler(c *gin.Context) {
	admin, ok := requireAuthCtx(c)
	if !ok {
		return
	}
	id, err := parseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid user id.", Code: "validation"})
		return
	}
	var body struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "A status of active, suspended or deactivated is required.", Code: "validation"})
		return
	}
	switch body.Status {
	case StatusActive, StatusSuspend, StatusDeact:
	default:
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Unknown status.", Code: "validation"})
		return
	}
	if id == admin.ID {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "You cannot suspend, deactivate or change your own account status.", Code: "self_action_blocked"})
		return
	}
	target, err := auth.FindUserByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "User not found.", Code: "not_found"})
		return
	}
	if _, err := database.DB.Exec(`UPDATE users SET status = ? WHERE id = ?`, body.Status, id); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Could not update the user status.", Code: "server_error"})
		return
	}
	action := "admin.user." + body.Status
	database.LogSystemForUser(admin, action, fmt.Sprintf("%s (%s) -> %s", target.Email, target.Role, body.Status))
	database.LogSystem("system", RoleAdmin, action, fmt.Sprintf("Status of %s changed to %s by %s", target.Email, body.Status, admin.Email))
	target.Status = body.Status
	c.JSON(http.StatusOK, gin.H{"user": target})
}

// CreateAnnouncementHandler publishes a system-wide notice to broadcast maintenance windows or severe weather warnings.
func CreateAnnouncementHandler(c *gin.Context) {
	admin, ok := requireAuthCtx(c)
	if !ok {
		return
	}
	var body struct {
		Title     string     `json:"title" binding:"required"`
		Message   string     `json:"message" binding:"required"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "A title and a message are required.", Code: "validation"})
		return
	}
	var exp interface{}
	if body.ExpiresAt != nil {
		exp = body.ExpiresAt.Format(time.RFC3339)
	}
	res, err := database.DB.Exec(`INSERT INTO announcements(title,message,author_id,author,expires_at,active) VALUES(?,?,?,?,?,1)`,
		strings.TrimSpace(body.Title), strings.TrimSpace(body.Message), admin.ID, admin.FirstName+" "+admin.LastName, exp)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Could not publish the announcement.", Code: "server_error"})
		return
	}
	id, _ := res.LastInsertId()
	database.LogSystemForUser(admin, "admin.broadcast", "Announcement published: "+body.Title)
	c.JSON(http.StatusCreated, gin.H{"id": id, "title": body.Title, "message": body.Message, "active": true})
}

// ListAnnouncementsHandler loads complete announcement records including expired notices for administrative oversight.
func ListAnnouncementsHandler(c *gin.Context) {
	rows, err := database.DB.Query(`SELECT id,title,message,author,created_at,COALESCE(expires_at,''),active FROM announcements ORDER BY id DESC LIMIT 100`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Could not load announcements.", Code: "server_error"})
		return
	}
	defer rows.Close()
	out := []Announcement{}
	for rows.Next() {
		var a Announcement
		var created, expires string
		var active int
		if err := rows.Scan(&a.ID, &a.Title, &a.Message, &a.Author, &created, &expires, &active); err != nil {
			continue
		}
		a.CreatedAt = database.ParseTime(created)
		a.Active = active == 1
		if expires != "" {
			t := database.ParseTime(expires)
			a.ExpiresAt = &t
			if t.Before(time.Now()) {
				a.State = "expired"
			}
		} else if a.Active {
			a.State = "active"
		} else {
			a.State = "inactive"
		}
		out = append(out, a)
	}
	c.JSON(http.StatusOK, gin.H{"announcements": out})
}

// UpdateAnnouncementHandler modifies publication status or deletes notices to manage public dashboard banners.
func UpdateAnnouncementHandler(c *gin.Context) {
	admin, ok := requireAuthCtx(c)
	if !ok {
		return
	}
	id, err := parseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid announcement id.", Code: "validation"})
		return
	}
	var body struct {
		Active *bool `json:"active"`
		Delete bool  `json:"delete"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid payload.", Code: "validation"})
		return
	}
	if body.Delete {
		_, _ = database.DB.Exec(`DELETE FROM announcements WHERE id = ?`, id)
		database.LogSystemForUser(admin, "admin.announcement.delete", "Announcement #"+strconv.Itoa(int(id))+" removed")
		c.JSON(http.StatusOK, gin.H{"deleted": id})
		return
	}
	if body.Active != nil {
		_, _ = database.DB.Exec(`UPDATE announcements SET active = ? WHERE id = ?`, database.BoolToInt(*body.Active), id)
		database.LogSystemForUser(admin, "admin.announcement.state", fmt.Sprintf("Announcement #%d active=%v", id, *body.Active))
	}
	c.JSON(http.StatusOK, gin.H{"updated": id})
}

// PublicAnnouncementsHandler retrieves active announcements while filtering out notices previously dismissed by the caller.
func PublicAnnouncementsHandler(c *gin.Context) {
	u, ok := requireAuthCtx(c)
	if !ok {
		return
	}
	rows, err := database.DB.Query(`SELECT a.id,a.title,a.message,a.author,a.created_at,COALESCE(a.expires_at,''),a.active,
		CASE WHEN d.user_id IS NULL THEN 0 ELSE 1 END AS dismissed
		FROM announcements a
		LEFT JOIN announcement_dismissals d ON d.announcement_id = a.id AND d.user_id = ?
		WHERE a.active = 1
		ORDER BY a.id DESC LIMIT 50`, u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Could not load notices.", Code: "server_error"})
		return
	}
	defer rows.Close()
	now := time.Now()
	out := []Announcement{}
	for rows.Next() {
		var a Announcement
		var created, expires string
		var active, dismissed int
		if err := rows.Scan(&a.ID, &a.Title, &a.Message, &a.Author, &created, &expires, &active, &dismissed); err != nil {
			continue
		}
		a.CreatedAt = database.ParseTime(created)
		a.Active = active == 1
		a.Dismissed = dismissed == 1
		a.State = "active"
		if expires != "" {
			t := database.ParseTime(expires)
			a.ExpiresAt = &t
			if t.Before(now) {
				a.State = "expired"
			}
		}
		if a.State == "expired" || a.Dismissed {
			continue
		}
		out = append(out, a)
	}
	c.JSON(http.StatusOK, gin.H{"announcements": out})
}

// DismissAnnouncementHandler records user acknowledgment to hide a notice across future sessions.
func DismissAnnouncementHandler(c *gin.Context) {
	u, ok := requireAuthCtx(c)
	if !ok {
		return
	}
	id, err := parseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid announcement id.", Code: "validation"})
		return
	}
	_, err = database.DB.Exec(`INSERT OR IGNORE INTO announcement_dismissals(announcement_id,user_id) VALUES(?,?)`, id, u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Could not dismiss the notice.", Code: "server_error"})
		return
	}
	database.LogSystemForUser(u, "announcement.dismiss", "Announcement #"+strconv.Itoa(int(id))+" dismissed")
	c.JSON(http.StatusOK, gin.H{"dismissed": id})
}

// ManualRefreshHandler executes concurrent and sequential upstream fetches to benchmark goroutine performance improvements.
func ManualRefreshHandler(c *gin.Context) {
	admin, ok := requireAuthCtx(c)
	if !ok {
		return
	}
	locs := database.MajorCities()
	// Cache bypass forces a fresh network roundtrip so concurrent and sequential latency measurements reflect real network conditions.
	concResults, concDur, _ := weather.FetchManyConcurrently(c.Request.Context(), locs, true)
	concOK := 0
	perLoc := []gin.H{}
	for _, r := range concResults {
		entry := gin.H{"name": r.Loc.Name, "latency_ms": r.LatencyMS, "ok": r.Err == nil}
		if r.Forecast != nil {
			entry["from_cache"] = r.Forecast.FromCache
			if r.Forecast.Current != nil {
				entry["temp_c"] = r.Forecast.Current.TempC
				entry["condition"] = r.Forecast.Current.Condition
			}
		} else if r.Err != nil {
			entry["error"] = r.Err.Error()
		}
		if r.Err == nil {
			concOK++
		}
		perLoc = append(perLoc, entry)
	}
	// Sequential execution provides the comparative baseline needed to calculate speedup multiples.
	seqStart := time.Now()
	seqOK := 0
	for _, loc := range locs {
		if _, err := weather.FetchForecast(c.Request.Context(), loc, true); err == nil {
			seqOK++
		}
	}
	seqDur := time.Since(seqStart)
	weather.Metrics.SetConcurrency(concDur.Milliseconds(), seqDur.Milliseconds(), len(locs))
	speedup := 0.0
	if concDur.Milliseconds() > 0 {
		speedup = weather.Round1(float64(seqDur.Milliseconds()) / float64(concDur.Milliseconds()))
	}
	database.LogSystemForUser(admin, "admin.refresh", fmt.Sprintf("Manual BOM refresh: %d/%d concurrent in %s, %d/%d sequential in %s",
		concOK, len(locs), concDur.Round(time.Millisecond), seqOK, len(locs), seqDur.Round(time.Millisecond)))
	c.JSON(http.StatusOK, gin.H{
		"concurrent_ms":  concDur.Milliseconds(),
		"sequential_ms":  seqDur.Milliseconds(),
		"speedup_factor": speedup,
		"locations":      perLoc,
		"concurrent_ok":  concOK,
		"sequential_ok":  seqOK,
		"checked_at":     time.Now(),
	})
}

// SourceCheckHandler probes primary and secondary endpoints to diagnose upstream feed availability and fallback statuses.
func SourceCheckHandler(c *gin.Context) {
	start := time.Now()
	q := weather.BOMQuery(database.Melbourne)
	check := SourceCheck{
		Endpoint:  weather.BOMEndpoint,
		CheckedAt: time.Now(),
	}
	primary, pErr := weather.GetOpenMeteo(c.Request.Context(), weather.BOMEndpoint, q)
	switch {
	case pErr != nil:
		check.PrimaryNote = "unreachable: " + pErr.Error()
	case !weather.OMHasData(primary):
		check.PrimaryNote = "responded but returned no usable values"
	default:
		check.PrimaryOk = true
		check.PrimaryNote = "live data available"
	}
	fallback, fErr := weather.GetOpenMeteo(c.Request.Context(), weather.NodeEndpoint, q)
	switch {
	case fErr != nil:
		check.FallbackNote = "unreachable: " + fErr.Error()
	case !weather.OMHasData(fallback):
		check.FallbackNote = "responded but returned no usable values"
	default:
		check.FallbackOk = true
		check.FallbackNote = "live data available"
	}
	check.LatencyMS = time.Since(start).Milliseconds()
	check.Healthy = check.PrimaryOk || check.FallbackOk

	// Probing the active Melbourne forecast verifies the exact payload structure delivered to end users.
	fc, err := weather.FetchForecast(c.Request.Context(), database.Melbourne, true)
	if err != nil {
		check.Message = "BOM endpoint: " + check.PrimaryNote + "; fallback: " + check.FallbackNote
		database.LogSystemForUser(currentUser(c), "admin.source.check", "FAILED: "+check.Message)
		c.JSON(http.StatusOK, check)
		return
	}
	check.Serving = fc.Source
	switch {
	case fc.SourceDegraded:
		check.SourceNote = "serving the fallback Open-Meteo forecast; the dedicated BOM ACCESS-G feed is not populated at the moment"
	case fc.FromCache:
		check.SourceNote = "serving cached data (upstream unavailable)"
	default:
		check.SourceNote = "serving live BOM ACCESS-G data"
	}
	check.Message = fmt.Sprintf("BOM feed: %s; fallback: %s. Serving %s in %d ms (%d forecast days, current %.0f°C in %s).",
		check.PrimaryNote, check.FallbackNote, check.Serving, check.LatencyMS, len(fc.Daily), fc.Current.TempC, fc.Location.Name)
	database.LogSystemForUser(currentUser(c), "admin.source.check", check.Message)
	c.JSON(http.StatusOK, check)
}

// LivePreviewHandler renders raw forecast payloads alongside operational telemetry to assist administrators in troubleshooting.
func LivePreviewHandler(c *gin.Context) {
	if _, ok := requireAuthCtx(c); !ok {
		return
	}
	fc, ok2 := forecastFromQuery(c, false)
	if !ok2 {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"banner":       "Admin live preview — data exactly as standard users receive it",
		"forecast":     fc,
		"metrics":      adminMetricsPayload(),
		"previewed_at": time.Now(),
	})
}

// LogsHandler queries the security audit log chronologically to support incident reviews and administrative tracking.
func LogsHandler(c *gin.Context) {
	if _, ok := requireAuthCtx(c); !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "60"))
	if limit <= 0 || limit > 500 {
		limit = 60
	}
	q := "%" + strings.ToLower(strings.TrimSpace(c.Query("q"))) + "%"
	rows, err := database.DB.Query(`SELECT id,ts,actor,role,action,detail FROM system_logs
		WHERE (LOWER(action) LIKE ? OR LOWER(detail) LIKE ? OR LOWER(actor) LIKE ?)
		ORDER BY id DESC LIMIT ?`, q, q, q, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Could not load logs.", Code: "server_error"})
		return
	}
	defer rows.Close()
	out := []SystemLog{}
	for rows.Next() {
		var l SystemLog
		var ts string
		if err := rows.Scan(&l.ID, &ts, &l.Actor, &l.Role, &l.Action, &l.Detail); err == nil {
			l.TS = database.ParseTime(ts)
			out = append(out, l)
		}
	}
	c.JSON(http.StatusOK, gin.H{"logs": out})
}

// LoginAsHandler grants an ephemeral standard-user session to administrators to test user permissions and reproduce reported issues.
func LoginAsHandler(c *gin.Context) {
	admin, ok := requireAuthCtx(c)
	if !ok {
		return
	}
	var body struct {
		UserID int64 `json:"user_id"`
	}
	_ = c.ShouldBindJSON(&body)
	target, err := auth.FindUserByID(body.UserID)
	if err != nil || target.Role != RoleStandard || target.Status != StatusActive {
		// Impersonating the first active account provides a zero-setup fallback when no explicit user target is supplied.
		rows, qErr := database.DB.Query(`SELECT id FROM users WHERE role='standard' AND status='active' ORDER BY id LIMIT 1`)
		if qErr == nil {
			if rows.Next() {
				var id int64
				_ = rows.Scan(&id)
				target, err = auth.FindUserByID(id)
			}
			rows.Close()
		}
	}
	if err != nil || target == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "No active standard user is available to log in as.", Code: "not_found"})
		return
	}
	token, err := auth.GenerateToken(target)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Could not start the impersonated session.", Code: "server_error"})
		return
	}
	database.LogSystemForUser(admin, "admin.login_as", "Administrator opened a standard-user session as "+target.Email)
	c.JSON(http.StatusOK, AuthResponse{Token: token, User: auth.LoadUserWithPrefs(target)})
}

// adminMetricsPayload aggregates active user counts, storage volumes, and concurrency multipliers for status widgets.
func adminMetricsPayload() AdminMetrics {
	m := AdminMetrics{}
	_ = database.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&m.TotalUsers)
	_ = database.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE last_active_at > ?`,
		time.Now().Add(-15*time.Minute).Format(time.RFC3339)).Scan(&m.ActiveUsers)
	_ = database.DB.QueryRow(`SELECT COUNT(*) FROM weather_cache`).Scan(&m.CacheEntries)
	_ = database.DB.QueryRow(`SELECT COUNT(*) FROM forecast_history`).Scan(&m.HistoryRows)
	_ = database.DB.QueryRow(`SELECT COUNT(*) FROM weather_alerts WHERE dismissed = 0`).Scan(&m.AlertsActive)
	avg, success, cache, _ := weather.Metrics.Snapshot()
	m.AvgLatencyMS = int64(avg)
	m.APISuccessPct = weather.Round1(success)
	m.CacheCoveragePct = weather.Round1(cache)
	conc, seq, locs := weather.Metrics.Concurrency()
	m.ConcurrentMS, m.SequentialMS, m.LocationsFetched = conc, seq, locs
	if conc > 0 {
		m.SpeedupFactor = weather.Round1(float64(seq) / float64(conc))
	}
	return m
}

// MetricsHandler serves consolidated operational metrics to drive real-time telemetry gauges in the admin console.
func MetricsHandler(c *gin.Context) {
	if _, ok := requireAuthCtx(c); !ok {
		return
	}
	c.JSON(http.StatusOK, adminMetricsPayload())
}
