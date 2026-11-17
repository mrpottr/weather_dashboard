package auth

import (
	"strings"
	"time"

	"weather-forecast-database"
)

/*
Package auth persistence helpers consolidate account queries and user lifecycle tracking.
Centralizing these routines ensures consistent parsing, role extraction, and preference defaults across all handlers.
*/

type (
	User          = database.User
	Preferences   = database.Preferences
	UserWithPrefs = database.UserWithPrefs
)

// FindUserByID retrieves a user record by primary key to hydrate authenticated session contexts.
func FindUserByID(id int64) (*User, error) {
	u := &User{}
	var created string
	var lastActive database.SQLNullString
	err := database.DB.QueryRow(`SELECT id,first_name,last_name,email,role,status,default_location,created_at,last_active_at
		FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.FirstName, &u.LastName, &u.Email, &u.Role, &u.Status, &u.DefaultLocation, &created, &lastActive)
	if err != nil {
		return nil, err
	}
	u.CreatedAt = database.ParseTime(created)
	if lastActive.Valid && lastActive.String != "" {
		t := database.ParseTime(lastActive.String)
		u.LastActiveAt = &t
	}
	return u, nil
}

// FindUserByEmail locates accounts by normalized email and fetches password hashes for credential verification.
func FindUserByEmail(email string) (*User, string, error) {
	u := &User{}
	var hash, created string
	var lastActive database.SQLNullString
	err := database.DB.QueryRow(`SELECT id,first_name,last_name,email,role,status,default_location,password_hash,created_at,last_active_at
		FROM users WHERE email = ?`, strings.ToLower(strings.TrimSpace(email))).
		Scan(&u.ID, &u.FirstName, &u.LastName, &u.Email, &u.Role, &u.Status, &u.DefaultLocation, &hash, &created, &lastActive)
	if err != nil {
		return nil, "", err
	}
	u.CreatedAt = database.ParseTime(created)
	if lastActive.Valid && lastActive.String != "" {
		t := database.ParseTime(lastActive.String)
		u.LastActiveAt = &t
	}
	return u, hash, nil
}

// LoadPreferences retrieves display settings with dark mode defaults to ensure predictable UI rendering.
func LoadPreferences(userID int64) Preferences {
	p := Preferences{Theme: "dark", ClockFormat: "24h"}
	var theme, format string
	if err := database.DB.QueryRow(`SELECT theme, clock_format FROM user_preferences WHERE user_id = ?`, userID).Scan(&theme, &format); err == nil {
		p.Theme, p.ClockFormat = theme, format
	}
	return p
}

// EnsurePreferences guarantees preference record existence on demand without requiring explicit registration migration steps.
func EnsurePreferences(userID int64) {
	_, _ = database.DB.Exec(`INSERT OR IGNORE INTO user_preferences(user_id,theme,clock_format) VALUES(?,?,?)`, userID, "dark", "24h")
}

// TouchUser tracks user activity timestamps asynchronously to power admin engagement auditing.
func TouchUser(id int64) {
	_, _ = database.DB.Exec(`UPDATE users SET last_active_at = ? WHERE id = ?`, time.Now().Format(time.RFC3339), id)
}

// LoadUserWithPrefs unites user identity with presentation settings in a single operation to minimize client roundtrips.
func LoadUserWithPrefs(u *User) UserWithPrefs {
	EnsurePreferences(u.ID)
	return UserWithPrefs{User: *u, Preferences: LoadPreferences(u.ID)}
}
