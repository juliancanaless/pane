package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestAgentStateStoreSetGetListDelete(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "pane.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	store := NewAgentStateStore(db)
	ctx := context.Background()
	item := AgentState{WorkspaceRoot: "/workspace", Key: "agent.memory", ValueJSON: `{"status":"ok"}`, UpdatedAt: 10, SessionID: "session-1"}
	if err := store.Set(ctx, item); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	got, err := store.Get(ctx, "/workspace", "agent.memory")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.ValueJSON != item.ValueJSON || got.SessionID != item.SessionID {
		t.Fatalf("got %#v, want %#v", got, item)
	}

	items, err := store.List(ctx, "/workspace", "agent.")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 1 || items[0].Key != "agent.memory" {
		t.Fatalf("unexpected list: %#v", items)
	}

	if err := store.Delete(ctx, "/workspace", "agent.memory"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := store.Get(ctx, "/workspace", "agent.memory"); err != ErrNotFound {
		t.Fatalf("Get after delete error = %v", err)
	}
}
