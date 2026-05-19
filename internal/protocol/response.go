package protocol

type Response struct {
	OK       bool           `json:"ok"`
	Warnings []string       `json:"warnings,omitempty"`
	Block    bool           `json:"block"`
	Payload  map[string]any `json:"payload,omitempty"`
	Error    string         `json:"error,omitempty"`
}

func Success(payload map[string]any, warnings ...string) Response {
	return Response{OK: true, Warnings: warnings, Payload: payload}
}

func Failure(err string) Response {
	return Response{OK: false, Error: err}
}
