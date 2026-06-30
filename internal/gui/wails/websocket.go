//go:build !nogui

package wails

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// wsServer exposes the Wails binding layer over a WebSocket so the frontend
// can be developed and tested in a normal browser without changing the
// generated binding API.
type wsServer struct {
	bindings *application.Bindings
	upgrader websocket.Upgrader
	mu       sync.RWMutex
	clients  map[*websocket.Conn]*sync.Mutex
	addr     string
}

// wsMessage is the envelope used on the WebSocket wire.
type wsMessage struct {
	Type       string          `json:"type"`
	ID         string          `json:"id,omitempty"`
	Object     int             `json:"object,omitempty"`
	Method     int             `json:"method,omitempty"`
	WindowName string          `json:"windowName,omitempty"`
	Args       json.RawMessage `json:"args,omitempty"`
	Name       string          `json:"name,omitempty"`
	Data       any             `json:"data,omitempty"`
	Result     any             `json:"result,omitempty"`
	Error      string          `json:"error,omitempty"`
}

// newWSServer creates a WebSocket IPC server backed by the given Wails bindings.
func newWSServer(bindings *application.Bindings) *wsServer {
	return &wsServer{
		bindings: bindings,
		clients:  make(map[*websocket.Conn]*sync.Mutex),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// start runs the WebSocket server in the background. It uses SINGBOX_EZ_WS_PORT
// or defaults to 34115. Set SINGBOX_EZ_WS_PORT=0 to disable. A nil error only
// means the listener was started.
func (s *wsServer) start() error {
	port := os.Getenv("SINGBOX_EZ_WS_PORT")
	if port == "" {
		port = "34115"
	}
	if port == "0" || port == "-1" {
		return nil
	}

	listener, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		return fmt.Errorf("listen websocket ipc: %w", err)
	}
	s.addr = listener.Addr().String()

	mux := http.NewServeMux()
	mux.HandleFunc("/wails/ws", s.handleConn)

	go func() {
		srv := &http.Server{
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		}
		if err := srv.Serve(listener); err != nil {
			log.Printf("websocket ipc server exited: %v", err)
		}
	}()

	log.Printf("WebSocket IPC server listening on ws://%s/wails/ws", s.addr)
	return nil
}

// handleConn upgrades an HTTP request to WebSocket and serves it.
func (s *wsServer) handleConn(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	s.addClient(conn)
	defer s.removeClient(conn)

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var msg wsMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			s.reply(conn, msg.ID, nil, fmt.Sprintf("invalid message: %v", err))
			continue
		}

		switch msg.Type {
		case "call":
			go s.handleCall(conn, msg)
		default:
			s.reply(conn, msg.ID, nil, "unknown message type")
		}
	}
}

// handleCall executes a Wails binding call and sends the result back.
func (s *wsServer) handleCall(conn *websocket.Conn, msg wsMessage) {
	const (
		callObjectID      = 0
		callBindingMethod = 0
	)

	if msg.Object != callObjectID || msg.Method != callBindingMethod {
		s.reply(conn, msg.ID, nil, "only binding calls are supported over websocket")
		return
	}

	var options application.CallOptions
	if err := json.Unmarshal(msg.Args, &options); err != nil {
		s.reply(conn, msg.ID, nil, fmt.Sprintf("parse call options: %v", err))
		return
	}

	var method *application.BoundMethod
	if options.MethodName != "" {
		method = s.bindings.Get(&options)
	} else {
		method = s.bindings.GetByID(options.MethodID)
	}
	if method == nil {
		ref := options.MethodName
		if ref == "" {
			ref = fmt.Sprintf("id:%d", options.MethodID)
		}
		s.reply(conn, msg.ID, nil, fmt.Sprintf("unknown bound method %s", ref))
		return
	}

	result, err := method.Call(context.Background(), options.Args)
	if err != nil {
		s.reply(conn, msg.ID, nil, err.Error())
		return
	}
	s.reply(conn, msg.ID, result, "")
}

// reply sends a call response to a single client.
func (s *wsServer) reply(conn *websocket.Conn, id string, result any, errStr string) {
	msg := wsMessage{Type: "call", ID: id, Result: result}
	if errStr != "" {
		msg.Error = errStr
	}
	_ = s.write(conn, msg)
}

// broadcast sends an event to all connected WebSocket clients.
func (s *wsServer) broadcast(name string, data any) {
	s.mu.RLock()
	clients := make([]*websocket.Conn, 0, len(s.clients))
	for c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.RUnlock()

	msg := wsMessage{Type: "event", Name: name, Data: data}
	for _, c := range clients {
		if err := s.write(c, msg); err != nil {
			s.removeClient(c)
			_ = c.Close()
		}
	}
}

// write marshals and sends a message with a short deadline.
func (s *wsServer) write(conn *websocket.Conn, msg wsMessage) error {
	s.mu.RLock()
	mu := s.clients[conn]
	s.mu.RUnlock()
	if mu == nil {
		return fmt.Errorf("client disconnected")
	}
	mu.Lock()
	defer mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetWriteDeadline(time.Time{})
	return conn.WriteMessage(websocket.TextMessage, data)
}

func (s *wsServer) addClient(conn *websocket.Conn) {
	s.mu.Lock()
	s.clients[conn] = &sync.Mutex{}
	s.mu.Unlock()
}

func (s *wsServer) removeClient(conn *websocket.Conn) {
	s.mu.Lock()
	delete(s.clients, conn)
	s.mu.Unlock()
}
