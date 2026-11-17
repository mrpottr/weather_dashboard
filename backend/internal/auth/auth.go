package auth

import (
	"errors"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

/*
Package auth manages cryptographic token issuance and brute-force mitigation.
Isolating identity concerns here keeps HTTP transport layers decoupled from credential verification rules.
*/

var jwtSecret = []byte("atmos-dev-secret") // Fallback secret prevents startup panics in local environments lacking configured environment variables.

// SetSecret registers the runtime key so token verification matches deployment credentials.
func SetSecret(secret []byte) {
	jwtSecret = secret
}

// GenerateToken encapsulates identity claims within signed tokens to enable stateless authentication across backend services.
func GenerateToken(u *User) (string, error) {
	claims := jwt.MapClaims{
		"sub":   u.ID,
		"role":  u.Role,
		"email": u.Email,
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(jwtSecret)
}

// ParseToken validates cryptographic integrity and extracts authorization claims to protect restricted endpoints.
func ParseToken(raw string) (int64, string, error) {
	tok, err := jwt.Parse(raw, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSecret, nil
	})
	if err != nil || !tok.Valid {
		return 0, "", errors.New("invalid or expired token")
	}
	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return 0, "", errors.New("invalid claims")
	}
	sub, ok := claims["sub"].(float64)
	if !ok {
		return 0, "", errors.New("missing subject")
	}
	role, _ := claims["role"].(string)
	return int64(sub), role, nil
}

type rateBucket struct {
	count   int
	resetAt time.Time
}

var (
	rateMu      sync.Mutex
	rateBuckets = map[string]*rateBucket{}
)

// AllowLogin applies localized IP throttling to protect authentication endpoints against credential stuffing and brute-force attacks.
func AllowLogin(ip string) bool {
	rateMu.Lock()
	defer rateMu.Unlock()
	now := time.Now()
	b, ok := rateBuckets[ip]
	if !ok || now.After(b.resetAt) {
		rateBuckets[ip] = &rateBucket{count: 1, resetAt: now.Add(time.Minute)}
		return true
	}
	if b.count >= 10 {
		return false
	}
	b.count++
	return true
}
