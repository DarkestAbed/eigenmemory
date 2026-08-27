package mcp

import "encoding/json"

// Request is a JSON-RPC request object.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC response object.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *ErrorObject    `json:"error,omitempty"`
}

// Notification is a JSON-RPC notification object (no id).
type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// ErrorObject is a JSON-RPC error.
type ErrorObject struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Error codes from MCP spec.
const (
	ErrParseError           = -32700
	ErrInvalidRequest       = -32600
	ErrMethodNotFound       = -32601
	ErrInvalidParams        = -32602
	ErrInternalError        = -32603
	ErrServerNotInitialized = -32002
)

func newError(code int, message string, data any) *ErrorObject {
	return &ErrorObject{Code: code, Message: message, Data: data}
}

// marshalResult wraps a result value in a Response with the given id.
func marshalResult(id json.RawMessage, result any) ([]byte, error) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	return json.Marshal(resp)
}

// marshalError wraps an error in a Response with the given id.
func marshalError(id json.RawMessage, err *ErrorObject) ([]byte, error) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   err,
	}
	return json.Marshal(resp)
}

// isNotification reports whether a request is a notification. Per JSON-RPC
// 2.0, a notification is distinguished by the absence of the "id" member
// entirely — an explicit `"id": null` is a normal request that must still
// receive a response.
func isNotification(id json.RawMessage) bool {
	return len(id) == 0
}

// rawJSON returns a json.RawMessage for an arbitrary value.
func rawJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

