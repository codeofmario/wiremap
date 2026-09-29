package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSameOriginBlocksCrossSiteWrites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SameOrigin(false))
	r.Any("/api/x", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	cases := []struct {
		method, origin string
		want           int
	}{
		{http.MethodPost, "https://evil.example", http.StatusForbidden},
		{http.MethodDelete, "http://localhost:5173", http.StatusForbidden},
		{http.MethodPost, "http://localhost:7070", http.StatusNoContent},
		{http.MethodPost, "", http.StatusNoContent},
		{http.MethodGet, "https://evil.example", http.StatusNoContent},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, "http://localhost:7070/api/x", nil)
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Errorf("%s from %q: got %d, want %d", tc.method, tc.origin, w.Code, tc.want)
		}
	}
}
