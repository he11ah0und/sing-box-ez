package rpc

// BoolValue is a request/response wrapper for a boolean value.
type BoolValue struct {
	Value bool `msgpack:"value"`
}

// StringValue is a request/response wrapper for a string value.
type StringValue struct {
	Value string `msgpack:"value"`
}

// IntValue is a request/response wrapper for an integer value.
type IntValue struct {
	Value int `msgpack:"value"`
}
