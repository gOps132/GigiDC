package agent

import (
	"context"
	"testing"
	"time"
)

func TestMemoryFollowUpStoreExpiresSnapshots(t *testing.T) {
	now := time.Date(2026, 6, 18, 1, 0, 0, 0, time.UTC)
	store := NewMemoryFollowUpStore()
	store.ttl = time.Minute
	store.clock = func() time.Time { return now }
	if err := store.Save(context.Background(), agentTestRequest(), RunSnapshot{Intent: "summary"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	now = now.Add(2 * time.Minute)
	_, ok, err := store.Load(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if ok {
		t.Fatalf("Load returned expired snapshot")
	}
}

func TestMemoryFollowUpStoreEvictsOldest(t *testing.T) {
	store := NewMemoryFollowUpStore()
	store.maxEntries = 1
	first := agentTestRequest()
	first.ActorUserID = "first"
	second := agentTestRequest()
	second.ActorUserID = "second"
	if err := store.Save(context.Background(), first, RunSnapshot{Intent: "first"}); err != nil {
		t.Fatalf("Save first returned error: %v", err)
	}
	if err := store.Save(context.Background(), second, RunSnapshot{Intent: "second"}); err != nil {
		t.Fatalf("Save second returned error: %v", err)
	}
	if _, ok, _ := store.Load(context.Background(), first); ok {
		t.Fatalf("oldest snapshot still present")
	}
	if snapshot, ok, _ := store.Load(context.Background(), second); !ok || snapshot.Intent != "second" {
		t.Fatalf("snapshot=%+v ok=%v, want newest", snapshot, ok)
	}
}
