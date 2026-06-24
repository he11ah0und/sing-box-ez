package rpc

import (
	"github.com/vmihailenco/msgpack/v5"
)

// Codec serializes RPC payloads.
type Codec interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
}

// MsgpackCodec uses msgpack for payload encoding.
type MsgpackCodec struct{}

// Marshal implements Codec.
func (MsgpackCodec) Marshal(v any) ([]byte, error) {
	return msgpack.Marshal(v)
}

// Unmarshal implements Codec.
func (MsgpackCodec) Unmarshal(data []byte, v any) error {
	return msgpack.Unmarshal(data, v)
}
