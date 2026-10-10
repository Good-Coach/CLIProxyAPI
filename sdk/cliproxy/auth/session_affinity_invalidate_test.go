package auth

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/registry"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/executor"
)

func TestManagerInvalidateSessionAffinityRebindsByPriorityAndKeepsAuthEnabled(t *testing.T) {
	ctx := context.Background()
	provider := "affinity-invalidate"
	model := "affinity-invalidate-model"
	highID := provider + "-high"
	lowID := provider + "-low"

	manager := NewManager(nil, nil, nil)
	affinity := NewSessionAffinitySelectorWithConfig(SessionAffinityConfig{
		Fallback: &RoundRobinSelector{},
		TTL:      time.Hour,
	})
	defer affinity.Stop()
	manager.SetSelector(affinity)
	manager.RegisterExecutor(schedulerTestExecutor{provider: provider})

	for _, auth := range []*Auth{
		{ID: highID, Provider: provider, Status: StatusActive, Attributes: map[string]string{"priority": "1"}},
		{ID: lowID, Provider: provider, Status: StatusActive, Attributes: map[string]string{"priority": "0"}},
	} {
		if _, errRegister := manager.Register(WithSkipPersist(ctx), auth); errRegister != nil {
			t.Fatalf("Register(%s): %v", auth.ID, errRegister)
		}
		registry.GetGlobalRegistry().RegisterClient(auth.ID, provider, []*registry.ModelInfo{{ID: model}})
		t.Cleanup(func() { registry.GetGlobalRegistry().UnregisterClient(auth.ID) })
	}

	session := func(id string) cliproxyexecutor.Options {
		return cliproxyexecutor.Options{Metadata: map[string]any{cliproxyexecutor.DerivedSessionIDMetadataKey: id}}
	}
	pick := func(opts cliproxyexecutor.Options) string {
		t.Helper()
		auth, _, errPick := manager.pickNext(ctx, provider, model, opts, nil)
		if errPick != nil || auth == nil {
			t.Fatalf("pick: auth=%v err=%v", auth, errPick)
		}
		return auth.ID
	}

	// Bind the session to the low-priority auth through a failover, then let high recover.
	if got := pick(session("long")); got != highID {
		t.Fatalf("cold binding = %q, want %q", got, highID)
	}
	manager.MarkResult(ctx, Result{AuthID: highID, Provider: provider, Model: model, Success: false,
		Error: &Error{HTTPStatus: http.StatusTooManyRequests, Message: "quota"}})
	if got := pick(session("long")); got != lowID {
		t.Fatalf("failover binding = %q, want %q", got, lowID)
	}
	expireSessionAffinityPriorityModelCooldown(t, manager, highID, model)
	if got := pick(session("long")); got != lowID {
		t.Fatalf("binding before invalidation = %q, want sticky %q", got, lowID)
	}

	manager.InvalidateSessionAffinity(lowID)

	if got := pick(session("long")); got != highID {
		t.Fatalf("binding after invalidation = %q, want highest priority %q", got, highID)
	}
	if got := pick(session("long")); got != highID {
		t.Fatalf("rebinding is not sticky: got %q, want %q", got, highID)
	}
	low, ok := manager.GetByID(lowID)
	if !ok || low.Disabled || low.Status == StatusDisabled {
		t.Fatalf("invalidated auth must stay enabled: %+v", low)
	}
}
