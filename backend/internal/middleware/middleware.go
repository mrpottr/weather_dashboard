package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"weather-forecast-database"

	"weather-forecast-system/internal/auth"
)

/*
Package middleware verifies bearer tokens and enforces role-based access controls across protected API endpoints.
Validating active account states on each request blocks revoked, suspended, or deactivated credentials immediately.
*/

type ctxKey string

const currentUserKey ctxKey = "current_user"

type (
	User          = database.User
	ErrorResponse = database.ErrorResponse
)

// AuthRequired validates bearer tokens and hydrates the request context with active user credentials.
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{Error: "Sign in required.", Code: "unauthorized"})
			return
		}
		uid, _, err := auth.ParseToken(strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{Error: "Session expired. Please sign in again.", Code: "token_invalid"})
			return
		}
		u, err := auth.FindUserByID(uid)
		if err != nil || u.Status != database.StatusActive {
			msg := "This account is not active."
			if err == nil && u.Status == database.StatusSuspend {
				msg = "This account has been suspended. Contact your administrator."
			} else if err == nil && u.Status == database.StatusDeact {
				msg = "This account has been deactivated. Contact your administrator."
			}
			c.AbortWithStatusJSON(http.StatusForbidden, ErrorResponse{Error: msg, Code: "account_inactive"})
			return
		}
		c.Set(string(currentUserKey), u)
		auth.TouchUser(u.ID)
		c.Next()
	}
}

// AdminRequired restricts route execution exclusively to authenticated users possessing administrator privileges.
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil || u.Role != database.RoleAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, ErrorResponse{
				Error: "Access denied: this function is reserved for administrator accounts.",
				Code:  "access_denied",
			})
			return
		}
		c.Next()
	}
}

// CurrentUser retrieves the authenticated user model attached to the gin context during upstream token inspection.
func CurrentUser(c *gin.Context) *User {
	if v, ok := c.Get(string(currentUserKey)); ok {
		if u, ok2 := v.(*User); ok2 {
			return u
		}
	}
	return nil
}
