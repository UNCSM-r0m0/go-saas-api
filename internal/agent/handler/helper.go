package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/platform/response"
)

func getUserID(c *gin.Context) (uuid.UUID, bool) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		response.Error(c, http.StatusUnauthorized, "missing X-User-ID")
		return uuid.Nil, false
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid X-User-ID")
		return uuid.Nil, false
	}
	return userID, true
}

func getUserIDSSE(c *gin.Context) (uuid.UUID, bool) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		writeSSEError(c, http.StatusUnauthorized, "AUTH_ERROR", "missing auth")
		return uuid.Nil, false
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeSSEError(c, http.StatusBadRequest, "STREAM_ERROR", "invalid user")
		return uuid.Nil, false
	}
	return userID, true
}

type UserPreferences struct {
	DisplayName string   `json:"display_name"`
	Profession  string   `json:"profession"`
	Traits      []string `json:"traits"`
	AboutMe     string   `json:"about_me"`
}

func getUserPreferences(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID) (*UserPreferences, error) {
	if pool == nil {
		return nil, nil
	}

	query := `
		SELECT display_name, profession, traits, about_me
		FROM user_preferences
		WHERE user_id = $1
	`
	row := pool.QueryRow(ctx, query, userID)

	var prefs UserPreferences
	var traits []string
	err := row.Scan(&prefs.DisplayName, &prefs.Profession, &traits, &prefs.AboutMe)
	if err != nil {
		return nil, err
	}
	prefs.Traits = traits

	return &prefs, nil
}

func buildUserContext(prefs *UserPreferences, contextItems []model.UserContextItem) string {
	var parts []string

	if prefs != nil {
		if prefs.DisplayName != "" {
			parts = append(parts, fmt.Sprintf("El usuario se llama %s", prefs.DisplayName))
		}
		if prefs.Profession != "" {
			parts = append(parts, fmt.Sprintf("trabaja como %s", prefs.Profession))
		}
		if len(prefs.Traits) > 0 {
			parts = append(parts, fmt.Sprintf("deberías ser %s", strings.Join(prefs.Traits, ", ")))
		}
		if prefs.AboutMe != "" {
			parts = append(parts, fmt.Sprintf("contexto adicional: %s", prefs.AboutMe))
		}
	}

	for _, item := range contextItems {
		if item.Key == "" || item.Value == nil {
			continue
		}
		valStr := fmt.Sprintf("%v", item.Value)
		if valStr == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s: %s", item.Key, valStr))
	}

	if len(parts) == 0 {
		return ""
	}

	return "\n\n[Preferencias del usuario] " + strings.Join(parts, ". ") + "."
}

func writeSSEError(c *gin.Context, status int, code string, msg string) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Writer.WriteHeader(status)
	c.Writer.Write([]byte(fmt.Sprintf("data: {\"error\":\"%s\",\"message\":\"%s\"}\n\n", code, msg)))
}

func writeSSEHeaders(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Writer.WriteHeader(http.StatusOK)
}
