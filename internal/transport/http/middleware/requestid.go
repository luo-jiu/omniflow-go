package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

const RequestIDKey = "request_id"

func RequestID() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestID := ctx.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = newRequestID()
		}

		ctx.Set(RequestIDKey, requestID)
		ctx.Writer.Header().Set("X-Request-ID", requestID)
		ctx.Next()
	}
}

func newRequestID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "request-id-unavailable"
	}
	return hex.EncodeToString(buf)
}
