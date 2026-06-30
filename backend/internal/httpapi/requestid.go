package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"regexp"
)

type requestIDKey struct{}

// validIncomingID accepts 1-128 ASCII letters, digits, '.', '_', '-'.
var validIncomingID = regexp.MustCompile(`^[A-Za-z0-9._\-]{1,128}$`)

// RequestIDFromContext returns the request ID stored in ctx, or "".
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

func generateRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "fallback-id"
	}
	return hex.EncodeToString(b)
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if !validIncomingID.MatchString(id) {
			id = generateRequestID()
		}
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
