package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// RPCMessage represents a JSON-RPC 2.0 message.
type RPCMessage struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *RPCError        `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC 2.0 error.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// ReadMessage reads a JSON-RPC message from the reader.
func ReadMessage(reader *bufio.Reader) (*RPCMessage, error) {
	// Read headers
	var contentLength int
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break // End of headers
		}

		if strings.HasPrefix(line, "Content-Length: ") {
			lengthStr := strings.TrimPrefix(line, "Content-Length: ")
			contentLength, err = strconv.Atoi(lengthStr)
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %v", err)
			}
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	// Read body
	body := make([]byte, contentLength)
	_, err := io.ReadFull(reader, body)
	if err != nil {
		return nil, err
	}

	var msg RPCMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse JSON-RPC message: %v", err)
	}

	return &msg, nil
}

// WriteMessage writes a JSON-RPC message to the writer.
func WriteMessage(writer io.Writer, msg *RPCMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if _, err := writer.Write([]byte(header)); err != nil {
		return err
	}
	if _, err := writer.Write(body); err != nil {
		return err
	}

	return nil
}
