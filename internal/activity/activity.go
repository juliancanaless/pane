package activity

type EventType string

const (
	EventCreated  EventType = "created"
	EventModified EventType = "modified"
	EventDeleted  EventType = "deleted"
	EventRenamed  EventType = "renamed"
)

type Attribution string

const (
	AttributionHigh   Attribution = "high"
	AttributionMedium Attribution = "medium"
	AttributionLow    Attribution = "low"
)

type FileActivity struct {
	ID          int64
	SessionID   string
	Path        string
	EventType   EventType
	Attribution Attribution
	Timestamp   int64
}
