package tcp

import (
	"io"
	"testing"
)

func TestTransportRoundTrip(t *testing.T) {
	tr := New("127.0.0.1:0")
	ln, err := tr.Listen()
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		buf := make([]byte, 4)
		n, err := c.Read(buf)
		if err != nil || n != 4 {
			return
		}
		_, _ = c.Write(buf[:n])
	}()

	clientTr := New(ln.Addr().String())
	client, err := clientTr.Dial()
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	if _, err := client.Write([]byte("ping")); err != nil {
		t.Fatalf("write: %v", err)
	}

	buf := make([]byte, 4)
	if _, err := io.ReadFull(client, buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(buf) != "ping" {
		t.Fatalf("expected ping, got %q", string(buf))
	}

	<-done
}

func TestTransportAddr(t *testing.T) {
	tr := New("127.0.0.1:8080")
	if got := tr.Addr(); got != "tcp://127.0.0.1:8080" {
		t.Fatalf("unexpected addr: %s", got)
	}
}
