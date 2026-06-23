// Package remote defines the binary RPC protocol used between the sing-box-ez
// remote daemon and its clients (GUI or CLI).
//
// The protocol uses a fixed 12-byte header followed by a msgpack-encoded
// payload. All numeric fields in the header are big-endian.
package remote

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/vmihailenco/msgpack/v5"
)

// Header is the fixed-size message header.
type Header struct {
	RequestID uint32 // correlates responses with requests
	Method    uint8  // Method* constant
	Flags     uint8  // Flag* constant
	Reserved  uint16 // must be zero
	Length    uint32 // payload length in bytes
}

// Binary header size in bytes.
const HeaderSize = 12

// Flags.
const (
	FlagRequest uint8 = iota
	FlagResponse
	FlagError
	FlagEvent
)

// Method identifiers. Events use FlagEvent and do not require a matching request.
const (
	MethodNone uint8 = iota
	MethodPing
	MethodCoreStart
	MethodCoreStop
	MethodCoreRestart
	MethodCoreStatus
	MethodLogEvent     // server -> client push
	MethodSetWatchLogs // client -> server toggle
	MethodConfigGetActive
	MethodConfigSetActive
	MethodConfigUpdate
	MethodConfigList
	MethodCoreDownloadCore
	MethodAppShutdown
)

// Message is a decoded protocol message.
type Message struct {
	Header  Header
	Payload []byte
}

// EncodeHeader writes the header to w.
func (h *Header) Encode(w io.Writer) error {
	buf := make([]byte, HeaderSize)
	binary.BigEndian.PutUint32(buf[0:], h.RequestID)
	buf[4] = h.Method
	buf[5] = h.Flags
	binary.BigEndian.PutUint16(buf[6:], h.Reserved)
	binary.BigEndian.PutUint32(buf[8:], h.Length)
	_, err := w.Write(buf)
	return err
}

// DecodeHeader reads a header from r.
func DecodeHeader(r io.Reader) (Header, error) {
	var h Header
	buf := make([]byte, HeaderSize)
	if _, err := io.ReadFull(r, buf); err != nil {
		return h, err
	}
	h.RequestID = binary.BigEndian.Uint32(buf[0:])
	h.Method = buf[4]
	h.Flags = buf[5]
	h.Reserved = binary.BigEndian.Uint16(buf[6:])
	h.Length = binary.BigEndian.Uint32(buf[8:])
	return h, nil
}

// ReadMessage reads a full message from r.
func ReadMessage(r io.Reader) (Message, error) {
	h, err := DecodeHeader(r)
	if err != nil {
		return Message{}, err
	}
	payload := make([]byte, h.Length)
	if h.Length > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return Message{}, err
		}
	}
	return Message{Header: h, Payload: payload}, nil
}

// WriteMessage writes a full message to w.
func WriteMessage(w io.Writer, h Header, payload []byte) error {
	h.Length = uint32(len(payload))
	if err := h.Encode(w); err != nil {
		return err
	}
	if len(payload) == 0 {
		return nil
	}
	_, err := w.Write(payload)
	return err
}

// MarshalPayload encodes v as msgpack.
func MarshalPayload(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return msgpack.Marshal(v)
}

// UnmarshalPayload decodes msgpack payload into v.
func UnmarshalPayload(data []byte, v any) error {
	if len(data) == 0 {
		return nil
	}
	return msgpack.Unmarshal(data, v)
}

// ---------- Shared payload types ----------

// Empty is an empty request or response payload.
type Empty struct{}

// BoolValue wraps a boolean.
type BoolValue struct {
	Value bool `msgpack:"value"`
}

// StringValue wraps a string.
type StringValue struct {
	Value string `msgpack:"value"`
}

// IntValue wraps an int.
type IntValue struct {
	Value int `msgpack:"value"`
}

// CoreStatusRes is the response for MethodCoreStatus.
type CoreStatusRes struct {
	Running bool `msgpack:"running"`
	PID     int  `msgpack:"pid"`
}

// LogEvent is pushed by the server using MethodLogEvent.
type LogEvent struct {
	Line string `msgpack:"line"`
}

// ConfigRecordMsg represents a config record in RPC messages.
type ConfigRecordMsg struct {
	Name                string `msgpack:"name"`
	URL                 string `msgpack:"url"`
	UpdateIntervalHours int    `msgpack:"update_interval_hours"`
	LastUpdateUnix      int64  `msgpack:"last_update_unix"`
	Parent              string `msgpack:"parent"`
	AutoUpdate          *bool  `msgpack:"auto_update,omitempty"`
}

// ConfigListRes is the response for MethodConfigList.
type ConfigListRes struct {
	ActiveName string            `msgpack:"active_name"`
	Configs    []ConfigRecordMsg `msgpack:"configs"`
}

// ConfigSetActiveReq is the request for MethodConfigSetActive.
type ConfigSetActiveReq struct {
	Name string `msgpack:"name"`
}

// ConfigUpdateReq is the request for MethodConfigUpdate.
type ConfigUpdateReq struct {
	Name string `msgpack:"name"`
	URL  string `msgpack:"url"`
}

// ErrorRes is returned with FlagError.
type ErrorRes struct {
	Message string `msgpack:"message"`
}

func (e *ErrorRes) Error() string {
	return e.Message
}

// NewError returns an error response payload.
func NewError(err error) *ErrorRes {
	if err == nil {
		return &ErrorRes{Message: ""}
	}
	return &ErrorRes{Message: err.Error()}
}

// MethodName returns a human-readable method name for debugging.
func MethodName(m uint8) string {
	switch m {
	case MethodPing:
		return "ping"
	case MethodCoreStart:
		return "core.start"
	case MethodCoreStop:
		return "core.stop"
	case MethodCoreRestart:
		return "core.restart"
	case MethodCoreStatus:
		return "core.status"
	case MethodLogEvent:
		return "log.event"
	case MethodSetWatchLogs:
		return "set.watch_logs"
	case MethodConfigGetActive:
		return "config.get_active"
	case MethodConfigSetActive:
		return "config.set_active"
	case MethodConfigUpdate:
		return "config.update"
	case MethodConfigList:
		return "config.list"
	case MethodCoreDownloadCore:
		return "core.download"
	case MethodAppShutdown:
		return "app.shutdown"
	default:
		return fmt.Sprintf("unknown(%d)", m)
	}
}
