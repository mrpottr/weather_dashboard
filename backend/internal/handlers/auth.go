package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"weather-forecast-database"

	"weather-forecast-system/internal/auth"
	"weather-forecast-system/internal/weather"
)

/*
Package handlers manages identity lifecycles, user sessions, and localized client preferences.
Decoupling credential verification and profile persistence enforces secure state transitions and defense against unauthorized access.
*/

// RegisterHandler provisions new accounts, persists hashed credentials, and automatically registers the default location bookmark.
func RegisterHandler(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Please provide a valid name, email and a password of at least 6 characters.", Code: "validation"})
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if _, _, err := auth.FindUserByEmail(req.Email); err == nil {
		c.JSON(http.StatusConflict, ErrorResponse{Error: "An account with this email already exists.", Code: "email_taken"})
		return
	}
	loc := LocationRef{Name: req.DefaultLocation, Latitude: req.Latitude, Longitude: req.Longitude, Timezone: req.Timezone, AdminArea: req.AdminArea}
	if loc.Latitude == 0 && loc.Longitude == 0 {
		loc = database.MatchDefaultLocation(req.DefaultLocation)
		if loc.Latitude == 0 && loc.Longitude == 0 {
			// Resolve the free-text default location through geocoding.
			if places, err := weather.GeocodeQuery(c.Request.Context(), req.DefaultLocation); err == nil && len(places) > 0 {
				p := places[0]
				loc = LocationRef{Name: p.Name + weather.StateSuffix(p.AdminArea), Latitude: p.Latitude, Longitude: p.Longitude, Timezone: p.Timezone, AdminArea: p.AdminArea}
			}
		}
	}
	if loc.Timezone == "" {
		loc.Timezone = weather.ResolveZone(loc.Latitude, loc.Longitude)
	}
	hash, err := database.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Could not secure the password.", Code: "server_error"})
		return
	}
	res, err := database.DB.Exec(`INSERT INTO users(first_name,last_name,email,password_hash,role,status,default_location)
		VALUES(?,?,?,?,'standard','active',?)`,
		strings.TrimSpace(req.FirstName), strings.TrimSpace(req.LastName), req.Email, hash, loc.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Could not create the account.", Code: "server_error"})
		return
	}
	id, _ := res.LastInsertId()
	auth.EnsurePreferences(id)
	// Automatically seeding the default location into favorites guarantees immediate dashboard usability post-registration.
	_, _ = database.DB.Exec(`INSERT OR IGNORE INTO locations(user_id,name,latitude,longitude,timezone,admin_area)
		VALUES(?,?,?,?,?,?)`, id, loc.Name, loc.Latitude, loc.Longitude, loc.Timezone, loc.AdminArea)

	u, _ := auth.FindUserByID(id)
	token, err := auth.GenerateToken(u)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Could not start the session.", Code: "server_error"})
		return
	}
	database.LogSystemForUser(u, "account.register", "New account created with default location "+loc.Name)
	c.JSON(http.StatusCreated, AuthResponse{Token: token, User: auth.LoadUserWithPrefs(u)})
}

// LoginHandler verifies credentials against password hashes and enforces account suspension locks to safeguard API access.
func LoginHandler(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Enter a valid email and password.", Code: "validation"})
		return
	}
	ip := c.ClientIP()
	if !auth.AllowLogin(ip) {
		c.JSON(http.StatusTooManyRequests, ErrorResponse{Error: "Too many sign-in attempts. Please wait a minute and try again.", Code: "rate_limited"})
		return
	}
	u, hash, err := auth.FindUserByEmail(req.Email)
	if err != nil || !database.CheckPassword(hash, req.Password) {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Email or password is incorrect.", Code: "invalid_credentials"})
		return
	}
	switch u.Status {
	case StatusSuspend:
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "This account has been suspended. Contact your administrator.", Code: "suspended"})
		return
	case StatusDeact:
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "This account has been deactivated. Contact your administrator.", Code: "deactivated"})
		return
	}
	token, err := auth.GenerateToken(u)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Could not start the session.", Code: "server_error"})
		return
	}
	database.LogSystemForUser(u, "auth.login", "User signed in from "+ip)
	c.JSON(http.StatusOK, AuthResponse{Token: token, User: auth.LoadUserWithPrefs(u)})
}

// MeHandler returns the active session profile and preferences to hydrate user state upon page reload.
func MeHandler(c *gin.Context) {
	u := currentUser(c)
	c.JSON(http.StatusOK, auth.LoadUserWithPrefs(u))
}

// UpdateMeHandler applies verified user profile mutations while validating uniqueness and re-authenticating password changes.
func UpdateMeHandler(c *gin.Context) {
	u := currentUser(c)
	var req UpdateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid update payload.", Code: "validation"})
		return
	}
	if req.FirstName != nil && strings.TrimSpace(*req.FirstName) != "" {
		u.FirstName = strings.TrimSpace(*req.FirstName)
	}
	if req.LastName != nil && strings.TrimSpace(*req.LastName) != "" {
		u.LastName = strings.TrimSpace(*req.LastName)
	}
	if req.DefaultLocation != nil && strings.TrimSpace(*req.DefaultLocation) != "" {
		u.DefaultLocation = strings.TrimSpace(*req.DefaultLocation)
	}
	if req.Email != nil {
		newEmail := strings.ToLower(strings.TrimSpace(*req.Email))
		if newEmail != u.Email {
			if _, _, err := auth.FindUserByEmail(newEmail); err == nil {
				c.JSON(http.StatusConflict, ErrorResponse{Error: "That email is already in use.", Code: "email_taken"})
				return
			}
			u.Email = newEmail
		}
	}
	if req.Password != nil && *req.Password != "" {
		if len(*req.Password) < 6 {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "The new password must be at least 6 characters.", Code: "validation"})
			return
		}
		if _, hash, err := auth.FindUserByEmail(u.Email); err != nil || !database.CheckPassword(hash, req.CurrentPassword) {
			c.JSON(http.StatusForbidden, ErrorResponse{Error: "Current password is incorrect.", Code: "password_mismatch"})
			return
		}
		hash, err := database.HashPassword(*req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Could not update the password.", Code: "server_error"})
			return
		}
		if _, err := database.DB.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, hash, u.ID); err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Could not update the password.", Code: "server_error"})
			return
		}
	}
	if _, err := database.DB.Exec(`UPDATE users SET first_name=?, last_name=?, email=?, default_location=? WHERE id=?`,
		u.FirstName, u.LastName, u.Email, u.DefaultLocation, u.ID); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Could not save the account changes.", Code: "server_error"})
		return
	}
	database.LogSystemForUser(u, "account.update", "Profile updated")
	c.JSON(http.StatusOK, auth.LoadUserWithPrefs(u))
}

// DeleteMeHandler irreversibly deletes the authenticated account and cascaded preferences in compliance with data privacy mandates.
func DeleteMeHandler(c *gin.Context) {
	u := currentUser(c)
	if _, err := database.DB.Exec(`DELETE FROM users WHERE id = ?`, u.ID); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Could not delete the account.", Code: "server_error"})
		return
	}
	database.LogSystem("account", u.Role, "account.delete", "Account deleted: "+u.Email)
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// GetPreferencesHandler retrieves active client presentation preferences for the caller.
func GetPreferencesHandler(c *gin.Context) {
	c.JSON(http.StatusOK, auth.LoadPreferences(currentUser(c).ID))
}

// PutPreferencesHandler stores theme and timestamp formatting selections to persist visual styling across devices.
func PutPreferencesHandler(c *gin.Context) {
	u := currentUser(c)
	var req PreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid preference payload.", Code: "validation"})
		return
	}
	auth.EnsurePreferences(u.ID)
	if req.Theme != nil && (*req.Theme == "dark" || *req.Theme == "light") {
		_, _ = database.DB.Exec(`UPDATE user_preferences SET theme = ? WHERE user_id = ?`, *req.Theme, u.ID)
	}
	if req.ClockFormat != nil && (*req.ClockFormat == "12h" || *req.ClockFormat == "24h") {
		_, _ = database.DB.Exec(`UPDATE user_preferences SET clock_format = ? WHERE user_id = ?`, *req.ClockFormat, u.ID)
	}
	c.JSON(http.StatusOK, auth.LoadPreferences(u.ID))
}
