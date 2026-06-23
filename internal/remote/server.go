// Package remote provides the RPC daemon that exposes the core.Controller over IPC.
package remote

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/ipc"
)

// Server exposes a core.Controller over the remote RPC protocol.
type Server struct {
	transport ipc.Transport
	ctrl      *core.Controller
	listener  net.Listener
	mu        sync.Mutex
	clients   map[net.Conn]*clientConn
	logSub    *core.LogSubscription
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

type clientConn struct {
	conn   net.Conn
	server *Server
}

// NewServer creates a remote server for the given controller and transport.
func NewServer(transport ipc.Transport, ctrl *core.Controller) *Server {
	s := &Server{
		transport: transport,
		ctrl:      ctrl,
		clients:   make(map[net.Conn]*clientConn),
		stopCh:    make(chan struct{}),
	}
	return s
}

// Run starts listening and serving connections. Blocks until Close is called.
func (s *Server) Run(ctx context.Context) error {
	ln, err := s.transport.Listen()
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.transport.Addr(), err)
	}
	s.listener = ln

	// Subscribe to core log lines so they can be broadcast to clients.
	s.logSub = s.ctrl.LogProcessor().AddSubscriber(func(line string) {
		s.broadcastLog(line)
	})

	s.wg.Add(1)
	go s.acceptLoop(ln)

	<-ctx.Done()
	return s.Close()
}

// Close shuts down the server and disconnects all clients.
func (s *Server) Close() error {
	close(s.stopCh)
	if s.listener != nil {
		_ = s.listener.Close()
	}
	if s.logSub != nil {
		s.ctrl.LogProcessor().RemoveSubscriber(s.logSub)
	}

	s.mu.Lock()
	for _, cc := range s.clients {
		_ = cc.conn.Close()
	}
	s.clients = make(map[net.Conn]*clientConn)
	s.mu.Unlock()

	s.wg.Wait()
	return nil
}

func (s *Server) acceptLoop(ln net.Listener) {
	defer s.wg.Done()
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
				// Log and continue after a short delay to avoid busy-loop on transient errors.
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}
		s.wg.Add(1)
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer s.wg.Done()

	cc := &clientConn{conn: conn, server: s}
	s.mu.Lock()
	s.clients[conn] = cc
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.clients, conn)
		s.mu.Unlock()
		_ = conn.Close()
	}()

	for {
		msg, err := ReadMessage(conn)
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

		s.wg.Add(1)
		go func(m Message) {
			defer s.wg.Done()
			s.dispatch(cc, m)
		}(msg)
	}
}

func (s *Server) dispatch(cc *clientConn, msg Message) {
	var respPayload []byte
	var flags uint8 = FlagResponse

	handle := func() (any, error) {
		switch msg.Header.Method {
		case MethodPing:
			return Empty{}, nil
		case MethodCoreStart:
			if _, err := s.ctrl.PrepareConfig(); err != nil {
				return nil, err
			}
			return Empty{}, s.ctrl.Start()
		case MethodCoreStop:
			return Empty{}, s.ctrl.Stop()
		case MethodCoreRestart:
			return Empty{}, s.ctrl.Restart()
		case MethodCoreStatus:
			return CoreStatusRes{Running: s.ctrl.IsRunning(), PID: s.ctrl.GetPID()}, nil
		case MethodSetWatchLogs:
			var req BoolValue
			if err := UnmarshalPayload(msg.Payload, &req); err != nil {
				return nil, err
			}
			s.ctrl.Config().MustGet("core", "watch_logs").Update(req.Value)
			_ = s.ctrl.Config().Save()
			return Empty{}, nil
		case MethodConfigGetActive:
			rec := s.ctrl.Config().GetActiveConfig()
			if rec == nil {
				return ConfigRecordMsg{}, fmt.Errorf("no active config")
			}
			return configRecordToMsg(*rec), nil
		case MethodConfigSetActive:
			var req ConfigSetActiveReq
			if err := UnmarshalPayload(msg.Payload, &req); err != nil {
				return nil, err
			}
			return Empty{}, s.ctrl.ActivateConfig(req.Name)
		case MethodConfigUpdate:
			var req ConfigUpdateReq
			if err := UnmarshalPayload(msg.Payload, &req); err != nil {
				return nil, err
			}
			return Empty{}, s.ctrl.UpdateConfigNow(req.Name, req.URL)
		case MethodConfigList:
			return buildConfigList(s.ctrl.Config()), nil
		case MethodCoreDownloadCore:
			_, err := s.ctrl.DownloadCore(nil)
			return Empty{}, err
		case MethodAppShutdown:
			go func() {
				time.Sleep(100 * time.Millisecond)
				s.ctrl.Close()
			}()
			return Empty{}, nil
		default:
			return nil, fmt.Errorf("unknown method %d", msg.Header.Method)
		}
	}

	res, err := handle()
	if err != nil {
		flags = FlagError
		respPayload, _ = MarshalPayload(NewError(err))
	} else {
		respPayload, _ = MarshalPayload(res)
	}

	respHeader := Header{
		RequestID: msg.Header.RequestID,
		Method:    msg.Header.Method,
		Flags:     flags,
	}
	_ = WriteMessage(cc.conn, respHeader, respPayload)
}

func (s *Server) broadcastLog(line string) {
	payload, _ := MarshalPayload(LogEvent{Line: line})
	header := Header{Method: MethodLogEvent, Flags: FlagEvent}

	s.mu.Lock()
	clients := make([]*clientConn, 0, len(s.clients))
	for _, cc := range s.clients {
		clients = append(clients, cc)
	}
	s.mu.Unlock()

	for _, cc := range clients {
		_ = WriteMessage(cc.conn, header, payload)
	}
}

func configRecordToMsg(rec config.ConfigRecord) ConfigRecordMsg {
	var last int64
	if !rec.LastUpdate.IsZero() {
		last = rec.LastUpdate.Unix()
	}
	return ConfigRecordMsg{
		Name:                rec.Name,
		URL:                 rec.URL,
		UpdateIntervalHours: rec.UpdateIntervalHours,
		LastUpdateUnix:      last,
		Parent:              rec.Parent,
		AutoUpdate:          rec.AutoUpdate,
	}
}

func buildConfigList(cfg *config.AppConfig) ConfigListRes {
	recs := cfg.GetConfigs()
	msgs := make([]ConfigRecordMsg, len(recs))
	for i, rec := range recs {
		msgs[i] = configRecordToMsg(rec)
	}
	return ConfigListRes{ActiveName: cfg.GetActiveName(), Configs: msgs}
}
