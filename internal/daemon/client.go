package daemon

import (
	"net"
	"time"

	"github.com/juliancanalez/pane/internal/protocol"
)

type Client struct {
	SocketPath string
	Timeout    time.Duration
}

func (c Client) Send(request protocol.Request) (protocol.Response, error) {
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 2 * time.Second
	}

	conn, err := net.DialTimeout("unix", c.SocketPath, timeout)
	if err != nil {
		return protocol.Response{}, err
	}
	defer conn.Close()

	deadline := time.Now().Add(timeout)
	_ = conn.SetDeadline(deadline)
	if err := protocol.WriteMessage(conn, request); err != nil {
		return protocol.Response{}, err
	}
	return protocol.ReadMessage[protocol.Response](conn)
}
