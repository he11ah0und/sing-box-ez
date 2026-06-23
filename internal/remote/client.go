// Package remote provides the RPC client for connecting to a sing-box-ez daemon.
package remote

import (
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"

	"sing-box-ez/internal/framework/ipc"
)

// Client connects to a remote sing-box-ez daemon and executes RPC calls.
type Client struct {
	conn      net.Conn
	enc       *msgpackEncoder
	dec       *msgpackDecoder
	mu        sync.Mutex
	nextID    uint32
	pending   map[uint32]chan Message
	closed    atomic.Bool
	onLog     func(string)
	stopLogCh chan struct{}
}

// msgpackEncoder wraps msgpack encoding with a mutex to serialize writes.
type msgpackEncoder struct {
	w  io.Writer
	mu sync.Mutex
}

type msgpackDecoder struct {
	r io.Reader
}

// Dial connects to a remote daemon using the given IPC transport.
func Dial(transport ipc.Transport) (*Client, error) {
	conn, err := transport.Dial()
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", transport.Addr(), err)
	}
	return NewClient(conn), nil
}

// NewClient wraps an existing connection.
func NewClient(conn net.Conn) *Client {
	c := &Client{
		conn:      conn,
		enc:       &msgpackEncoder{w: conn},
		dec:       &msgpackDecoder{r: conn},
		pending:   make(map[uint32]chan Message),
		stopLogCh: make(chan struct{}),
	}
	go c.readLoop()
	return c
}

// Close terminates the connection.
func (c *Client) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}
	close(c.stopLogCh)
	return c.conn.Close()
}

// SetOnLog sets a callback for incoming log lines.
func (c *Client) SetOnLog(fn func(string)) {
	c.mu.Lock()
	c.onLog = fn
	c.mu.Unlock()
}

// Call sends a request and waits for a response.
func (c *Client) Call(method uint8, req, resp any) error {
	id := atomic.AddUint32(&c.nextID, 1)
	payload, err := MarshalPayload(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	ch := make(chan Message, 1)
	c.mu.Lock()
	if c.closed.Load() {
		c.mu.Unlock()
		return fmt.Errorf("client closed")
	}
	c.pending[id] = ch
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	if err := c.writeMessage(Header{RequestID: id, Method: method, Flags: FlagRequest}, payload); err != nil {
		return fmt.Errorf("write request: %w", err)
	}

	msg := <-ch
	if msg.Header.Flags == FlagError {
		var errRes ErrorRes
		if err := UnmarshalPayload(msg.Payload, &errRes); err != nil {
			return fmt.Errorf("remote error (decode failed): %w", err)
		}
		return &errRes
	}
	if resp != nil {
		if err := UnmarshalPayload(msg.Payload, resp); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}
	return nil
}

// Ping checks connectivity.
func (c *Client) Ping() error {
	return c.Call(MethodPing, Empty{}, nil)
}

// CoreStart starts the sing-box core.
func (c *Client) CoreStart() error {
	return c.Call(MethodCoreStart, Empty{}, nil)
}

// CoreStop stops the sing-box core.
func (c *Client) CoreStop() error {
	return c.Call(MethodCoreStop, Empty{}, nil)
}

// CoreRestart restarts the sing-box core.
func (c *Client) CoreRestart() error {
	return c.Call(MethodCoreRestart, Empty{}, nil)
}

// CoreStatus returns whether the core is running and its PID.
func (c *Client) CoreStatus() (CoreStatusRes, error) {
	var res CoreStatusRes
	err := c.Call(MethodCoreStatus, Empty{}, &res)
	return res, err
}

// ConfigGetActive returns the active config record.
func (c *Client) ConfigGetActive() (ConfigRecordMsg, error) {
	var res ConfigRecordMsg
	err := c.Call(MethodConfigGetActive, Empty{}, &res)
	return res, err
}

// ConfigSetActive activates a config by name.
func (c *Client) ConfigSetActive(name string) error {
	return c.Call(MethodConfigSetActive, ConfigSetActiveReq{Name: name}, nil)
}

// ConfigUpdate downloads a config by name and URL.
func (c *Client) ConfigUpdate(name, url string) error {
	return c.Call(MethodConfigUpdate, ConfigUpdateReq{Name: name, URL: url}, nil)
}

// ConfigList returns all configs and the active name.
func (c *Client) ConfigList() (ConfigListRes, error) {
	var res ConfigListRes
	err := c.Call(MethodConfigList, Empty{}, &res)
	return res, err
}

// CoreDownloadCore downloads the latest core binary.
func (c *Client) CoreDownloadCore() error {
	return c.Call(MethodCoreDownloadCore, Empty{}, nil)
}

// AppShutdown asks the remote daemon to shut down.
func (c *Client) AppShutdown() error {
	return c.Call(MethodAppShutdown, Empty{}, nil)
}

func (c *Client) writeMessage(h Header, payload []byte) error {
	c.enc.mu.Lock()
	defer c.enc.mu.Unlock()
	return WriteMessage(c.enc.w, h, payload)
}

func (c *Client) readLoop() {
	for {
		msg, err := ReadMessage(c.dec.r)
		if err != nil {
			if c.closed.Load() {
				return
			}
			c.failPending(fmt.Errorf("read message: %w", err))
			_ = c.Close()
			return
		}

		if msg.Header.Flags == FlagEvent {
			c.handleEvent(msg)
			continue
		}

		c.mu.Lock()
		ch, ok := c.pending[msg.Header.RequestID]
		c.mu.Unlock()
		if ok {
			ch <- msg
		}
	}
}

func (c *Client) handleEvent(msg Message) {
	switch msg.Header.Method {
	case MethodLogEvent:
		var ev LogEvent
		if err := UnmarshalPayload(msg.Payload, &ev); err != nil {
			return
		}
		c.mu.Lock()
		onLog := c.onLog
		c.mu.Unlock()
		if onLog != nil {
			onLog(ev.Line)
		}
	}
}

func (c *Client) failPending(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, ch := range c.pending {
		ch <- Message{Header: Header{Flags: FlagError}, Payload: mustMarshal(NewError(err))}
	}
	c.pending = make(map[uint32]chan Message)
}

func mustMarshal(v any) []byte {
	b, _ := MarshalPayload(v)
	return b
}
