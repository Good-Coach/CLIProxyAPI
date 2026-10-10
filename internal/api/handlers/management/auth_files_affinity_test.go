package management

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/auth"
)

func TestDeleteAuthFileSessionAffinity(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	manager := coreauth.NewManager(&memoryAuthStore{}, nil, nil)
	record := &coreauth.Auth{ID: "team.json", FileName: "team.json", Provider: "claude", Status: coreauth.StatusActive}
	if _, errRegister := manager.Register(context.Background(), record); errRegister != nil {
		t.Fatalf("register: %v", errRegister)
	}
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: t.TempDir()}, manager)

	for _, tc := range []struct {
		query string
		want  int
	}{
		{"?name=team.json", http.StatusOK},
		{"?name=missing.json", http.StatusNotFound},
		{"", http.StatusBadRequest},
	} {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodDelete, "/v0/management/auth-files/session-affinity"+tc.query, nil)
		h.DeleteAuthFileSessionAffinity(ctx)
		if rec.Code != tc.want {
			t.Fatalf("%q: status %d, want %d (%s)", tc.query, rec.Code, tc.want, rec.Body.String())
		}
	}
	if auth, ok := manager.GetByID("team.json"); !ok || auth.Disabled {
		t.Fatalf("auth must stay registered and enabled: %+v", auth)
	}
}
