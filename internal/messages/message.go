package messages

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
