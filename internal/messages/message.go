package messages

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

type State string

const (
	StateQueued    State = "queued"
	StateDelivered State = "delivered"
	StateExpired   State = "expired"
)

type Message struct {
	ID          string
	ThreadID    string
	FromSession string
	ToSession   string
	Body        string
	State       State
	CreatedAt   int64
	DeliveredAt *int64
}

func NewID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "msg-" + hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return "msg-" + hex.EncodeToString(bytes[:])
}
