package errors

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AppError struct {
	StatusCode int    `json:"-"`
	Message    string `json:"message"`
}

func (e *AppError) Error() string {
	return e.Message
}

func NotFound(msg string) *AppError {
	return &AppError{StatusCode: http.StatusNotFound, Message: msg}
}

func BadRequest(msg string) *AppError {
	return &AppError{StatusCode: http.StatusBadRequest, Message: msg}
}

func Internal(msg string) *AppError {
	return &AppError{StatusCode: http.StatusInternalServerError, Message: msg}
}

func HandleError(c *gin.Context, err error) {
	if appErr, ok := err.(*AppError); ok {
		c.JSON(appErr.StatusCode, appErr)
		return
	}
	log.Printf("internal error: %s", err)
	c.JSON(http.StatusInternalServerError, &AppError{Message: "internal server error"})
}
