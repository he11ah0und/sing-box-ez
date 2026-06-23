package rpc

import (
	"encoding/binary"
	"fmt"
	"io"
)

// requestEnvelope is sent from client to server.
type requestEnvelope struct {
	Namespace string `msgpack:"namespace"`
	Method    string `msgpack:"method"`
	Payload   []byte `msgpack:"payload"`
}

// responseEnvelope is sent from server to client.
type responseEnvelope struct {
	Payload []byte `msgpack:"payload"`
	Error   string `msgpack:"error"`
}

// readFrame reads a length-prefixed msgpack frame from r.
func readFrame(r io.Reader) ([]byte, error) {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, err
	}
	if length == 0 {
		return nil, nil
	}
	if length > 64<<20 { // 64 MiB sanity limit
		return nil, fmt.Errorf("frame too large: %d", length)
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// writeFrame writes a length-prefixed msgpack frame to w.
func writeFrame(w io.Writer, data []byte) error {
	if err := binary.Write(w, binary.BigEndian, uint32(len(data))); err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	_, err := w.Write(data)
	return err
}
