package supports

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"weather-forecast-database"
)

/*
Package supports provides file streaming, filename sanitization, and human-friendly time formatting.
Isolating I/O encoding utilities ensures standard browser download behaviors and clean header attachments.
*/

type ErrorResponse = database.ErrorResponse

// WriteCSVResponse streams tabular datasets directly into browser downloads with proper MIME headers to prevent buffering memory bloat.
func WriteCSVResponse(c *gin.Context, filename string, rows [][]string) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w := csv.NewWriter(c.Writer)
	for _, r := range rows {
		if err := w.Write(r); err != nil {
			return
		}
	}
	w.Flush()
}

// WriteJSONResponse serializes arbitrary data models into indented JSON attachments for structured data archiving.
func WriteJSONResponse(c *gin.Context, filename string, payload interface{}) {
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Could not build the export.", Code: "server_error"})
		return
	}
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	_, _ = c.Writer.Write(b)
}

// SafeFilePart sanitizes freeform strings into POSIX-compliant filename slugs to prevent header injection and filesystem errors.
func SafeFilePart(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, s)
	return strings.Trim(s, "-")
}

// HumanSince formats absolute timestamps into relative durations to improve immediate readability in data tables.
func HumanSince(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	}
}
