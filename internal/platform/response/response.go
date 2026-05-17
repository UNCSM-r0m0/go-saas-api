package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ApiResponse is the standard response wrapper expected by the r3-chat frontend.
type ApiResponse struct {
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
	Success bool        `json:"success"`
}

// OK sends a successful response with data.
func OK(c *gin.Context, data interface{}, message string) {
	c.JSON(http.StatusOK, ApiResponse{
		Data:    data,
		Message: message,
		Success: true,
	})
}

// Created sends a successful created response with data.
func Created(c *gin.Context, data interface{}, message string) {
	c.JSON(http.StatusCreated, ApiResponse{
		Data:    data,
		Message: message,
		Success: true,
	})
}

// Error sends an error response.
func Error(c *gin.Context, status int, message string) {
	c.JSON(status, ApiResponse{
		Data:    nil,
		Message: message,
		Success: false,
	})
}

// Raw sends a raw response without the ApiResponse wrapper.
// Use sparingly for endpoints that must return plain data (e.g., file downloads).
func Raw(c *gin.Context, status int, data interface{}) {
	c.JSON(status, data)
}
