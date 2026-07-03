package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ans-b/server/internal/auth"

	"github.com/gin-gonic/gin"
)

func TestRegisterRoutesAddsHealthEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	RegisterRoutes(engine)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected /healthz to return 200, got %d", recorder.Code)
	}
}

func TestRegisterRoutesHandlesCORSPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	RegisterRoutes(engine)

	for _, origin := range []string{
		"http://127.0.0.1:23457",
		"http://100.115.97.57:23457",
	} {
		t.Run(origin, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodOptions, "/api/v1/qa/ask", nil)
			request.Header.Set("Origin", origin)
			request.Header.Set("Access-Control-Request-Method", "POST")
			request.Header.Set("Access-Control-Request-Headers", "content-type,authorization")
			engine.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusNoContent {
				t.Fatalf("expected CORS preflight to return 204, got %d", recorder.Code)
			}
			if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != origin {
				t.Fatalf("expected CORS origin header %q, got %q", origin, got)
			}
			if got := recorder.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type, Authorization" {
				t.Fatalf("expected CORS headers to allow authorization, got %q", got)
			}
		})
	}
}

func TestRegisterRoutesAddsExpectedEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("JWT_SECRET", "test-secret")
	engine := gin.New()

	RegisterRoutes(engine)

	tests := []struct {
		method string
		path   string
		want   int
	}{
		{http.MethodPost, "/api/v1/auth/student/login", http.StatusBadRequest},
		{http.MethodPost, "/api/v1/auth/admin/login", http.StatusBadRequest},
		{http.MethodPost, "/api/v1/auth/logout", http.StatusUnauthorized},
		{http.MethodPost, "/api/v1/users/register", http.StatusBadRequest},
		{http.MethodGet, "/api/v1/users/me", http.StatusUnauthorized},
		{http.MethodGet, "/api/v1/knowledge", http.StatusUnauthorized},
		{http.MethodPost, "/api/v1/knowledge", http.StatusUnauthorized},
		{http.MethodPost, "/api/v1/qa/ask", http.StatusUnauthorized},
		{http.MethodGet, "/api/v1/search/candidates", http.StatusNotImplemented},
		{http.MethodPost, "/api/v1/submissions", http.StatusUnauthorized},
		{http.MethodGet, "/api/v1/submissions", http.StatusUnauthorized},
		{http.MethodGet, "/api/v1/analytics/hot-questions", http.StatusServiceUnavailable},
		{http.MethodPost, "/api/v1/model/embeddings", http.StatusNotImplemented},
		{http.MethodPost, "/api/v1/storage/imports", http.StatusNotImplemented},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			engine.ServeHTTP(recorder, request)

			if recorder.Code != tt.want {
				t.Fatalf("expected %s %s to return %d, got %d", tt.method, tt.path, tt.want, recorder.Code)
			}
		})
	}
}

func TestKnowledgeRoutesRequireAdminRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("JWT_SECRET", "test-secret")

	store := auth.NewMemorySessionStore()
	manager := auth.NewTokenManager("test-secret", time.Hour)

	engine := gin.New()
	RegisterRoutesWithDBEmbedderAndSessionStore(engine, nil, nil, store)

	tests := []struct {
		name  string
		token string
		want  int
	}{
		{
			name:  "student token is forbidden",
			token: createRoleToken(t, manager, store, "student-session", auth.RoleStudent),
			want:  http.StatusForbidden,
		},
		{
			name:  "admin token reaches handler",
			token: createRoleToken(t, manager, store, "admin-session", auth.RoleAdmin),
			want:  http.StatusNotImplemented,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge", nil)
			request.Header.Set("Authorization", "Bearer "+tt.token)

			engine.ServeHTTP(recorder, request)

			if recorder.Code != tt.want {
				t.Fatalf("expected GET /api/v1/knowledge to return %d, got %d", tt.want, recorder.Code)
			}
		})
	}
}

func createRoleToken(t *testing.T, manager *auth.TokenManager, store auth.SessionStore, sessionID string, role string) string {
	t.Helper()

	token, _, err := manager.Sign(12, role+"-user", role, sessionID)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	if err := store.Create(context.Background(), auth.Session{
		ID:        sessionID,
		UserID:    12,
		Username:  role + "-user",
		Role:      role,
		ExpiresAt: time.Now().Add(time.Hour),
	}, time.Hour); err != nil {
		t.Fatalf("create session: %v", err)
	}

	return token
}
