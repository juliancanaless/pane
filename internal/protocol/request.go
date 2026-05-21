package protocol

type RequestType string

const (
	RequestDaemonHealth     RequestType = "DaemonHealth"
	RequestDaemonStop       RequestType = "DaemonStop"
	RequestSessionInit      RequestType = "SessionInit"
	RequestSessionHeartbeat RequestType = "SessionHeartbeat"
	RequestSessionClose     RequestType = "SessionClose"
	RequestSessionStatus    RequestType = "SessionStatus"
	RequestSessionIntent    RequestType = "SessionIntent"
	RequestSessionContinue  RequestType = "SessionContinue"
	RequestSessionHistory   RequestType = "SessionHistory"
	RequestGitPreflight     RequestType = "GitPreflight"
	RequestGitRecord        RequestType = "GitRecord"
	RequestGetBoard         RequestType = "GetBoard"
	RequestGetSummary       RequestType = "GetSummary"
	RequestMessageSend      RequestType = "MessageSend"
	RequestMessageList      RequestType = "MessageList"
	RequestMessageReply     RequestType = "MessageReply"
	RequestStateSet         RequestType = "StateSet"
	RequestStateGet         RequestType = "StateGet"
	RequestStateList        RequestType = "StateList"
	RequestStateDelete      RequestType = "StateDelete"
)

type Request struct {
	Type      RequestType    `json:"type"`
	SessionID string         `json:"session_id,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
}
