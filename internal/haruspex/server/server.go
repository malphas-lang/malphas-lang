package server

import (
	"bufio"
	"io"
	"os"
	"sync"
)

// Server represents the Haruspex LSP server.
type Server struct {
	reader *bufio.Reader
	writer io.Writer
	mu     sync.Mutex
}

// NewServer creates a new LSP server.
func NewServer() *Server {
	return &Server{
		reader: bufio.NewReader(os.Stdin),
		writer: os.Stdout,
	}
}

// Serve starts the LSP server.
func (s *Server) Serve() error {
	for {
		msg, err := ReadMessage(s.reader)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		s.HandleMessage(msg)
	}
}

func (s *Server) write(msg *RPCMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	WriteMessage(s.writer, msg)
}
