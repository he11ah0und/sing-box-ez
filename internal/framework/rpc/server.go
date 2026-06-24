package rpc

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"

	"sing-box-ez/internal/framework/ipc"
)

// Server exposes a Registry over an IPC transport.
type Server struct {
	transport ipc.Transport
	registry  *Registry
	listener  net.Listener
	clients   sync.WaitGroup
	stopCh    chan struct{}
	codec     Codec
}

// NewServer creates a server for the given registry and transport.
func NewServer(registry *Registry, transport ipc.Transport) *Server {
	return &Server{
		transport: transport,
		registry:  registry,
		stopCh:    make(chan struct{}),
		codec:     MsgpackCodec{},
	}
}

// Addr returns the server's listen address.
func (s *Server) Addr() string {
	if s.listener == nil {
		return s.transport.Addr()
	}
	return s.listener.Addr().String()
}

// Run listens and serves until the context is cancelled or Close is called.
func (s *Server) Run(ctx context.Context) error {
	ln, err := s.transport.Listen()
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.transport.Addr(), err)
	}
	s.listener = ln

	s.clients.Add(1)
	go s.acceptLoop(ln)

	<-ctx.Done()
	return s.Close()
}

// Close shuts down the server.
func (s *Server) Close() error {
	close(s.stopCh)
	if s.listener != nil {
		_ = s.listener.Close()
	}
	s.clients.Wait()
	return nil
}

func (s *Server) acceptLoop(ln net.Listener) {
	defer s.clients.Done()
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
				continue
			}
		}
		s.clients.Add(1)
		go func(c net.Conn) {
			defer s.clients.Done()
			s.handleConn(c)
		}(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	ctx := context.Background()
	for {
		frame, err := readFrame(conn)
		if err != nil {
			if err == io.EOF {
				return
			}
			select {
			case <-s.stopCh:
				return
			default:
				return
			}
		}

		var req requestEnvelope
		if err := s.codec.Unmarshal(frame, &req); err != nil {
			s.writeError(conn, fmt.Errorf("decode request: %w", err))
			continue
		}

		h, ok := s.registry.lookup(req.Namespace, req.Method)
		if !ok {
			s.writeError(conn, fmt.Errorf("unknown method %s/%s", req.Namespace, req.Method))
			continue
		}

		replyBytes, err := h.invoke(ctx, req.Payload)
		if err != nil {
			s.writeError(conn, err)
			continue
		}

		respBytes, err := s.codec.Marshal(responseEnvelope{Payload: replyBytes})
		if err != nil {
			s.writeError(conn, fmt.Errorf("encode response: %w", err))
			continue
		}
		if err := writeFrame(conn, respBytes); err != nil {
			return
		}
	}
}

func (s *Server) writeError(conn net.Conn, err error) {
	respBytes, _ := s.codec.Marshal(responseEnvelope{Error: err.Error()})
	_ = writeFrame(conn, respBytes)
}
