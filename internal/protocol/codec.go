package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

const MaxMessageBytes = 1 << 20

func WriteMessage(w io.Writer, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(payload) > MaxMessageBytes {
		return fmt.Errorf("message too large: %d bytes", len(payload))
	}

	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(payload)))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	_, err = w.Write(payload)
	return err
}

func ReadMessage[T any](r io.Reader) (T, error) {
	var zero T
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return zero, err
	}

	length := binary.BigEndian.Uint32(header[:])
	if length > MaxMessageBytes {
		return zero, fmt.Errorf("message too large: %d bytes", length)
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return zero, err
	}

	var value T
	if err := json.Unmarshal(payload, &value); err != nil {
		return zero, err
	}
	return value, nil
}
