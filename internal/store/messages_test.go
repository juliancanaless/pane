package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/juliancanalez/pane/internal/messages"
	"github.com/juliancanalez/pane/internal/session"
)

func TestMessageStoreInboxLifecycle(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "pane.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	sessions := NewSessionStore(db)
	for _, value := range []session.Session{
		{ID: "session-a", PaneID: "pane-a", WorkspaceRoot: "/workspace", StartedAt: 1, LastSeenAt: 1, Status: session.StatusActive},
		{ID: "session-b", PaneID: "pane-b", WorkspaceRoot: "/workspace", StartedAt: 1, LastSeenAt: 1, Status: session.StatusActive},
	} {
		if err := sessions.Save(ctx, value); err != nil {
			t.Fatalf("Save session returned error: %v", err)
		}
	}

	store := NewMessageStore(db)
	message := messages.Message{ID: "msg-1", ThreadID: "msg-1", FromSession: "session-a", ToSession: "session-b", Body: "Are you done?", State: messages.StateQueued, CreatedAt: 10}
	if err := store.Save(ctx, message); err != nil {
		t.Fatalf("Save message returned error: %v", err)
	}

	inbox, err := store.ListQueuedForSession(ctx, "session-b")
	if err != nil {
		t.Fatalf("ListQueuedForSession returned error: %v", err)
	}
	if len(inbox) != 1 || inbox[0].ID != "msg-1" {
		t.Fatalf("unexpected inbox: %#v", inbox)
	}

	if err := store.MarkDelivered(ctx, []string{"msg-1"}, 20); err != nil {
		t.Fatalf("MarkDelivered returned error: %v", err)
	}
	inbox, err = store.ListQueuedForSession(ctx, "session-b")
	if err != nil {
		t.Fatalf("ListQueuedForSession returned error: %v", err)
	}
	if len(inbox) != 0 {
		t.Fatalf("expected empty inbox after delivery: %#v", inbox)
	}

	stored, err := store.FindByID(ctx, "msg-1")
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if stored.State != messages.StateDelivered || stored.DeliveredAt == nil || *stored.DeliveredAt != 20 {
		t.Fatalf("unexpected stored message: %#v", stored)
	}
}
