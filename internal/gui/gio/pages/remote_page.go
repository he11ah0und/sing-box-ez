package pages

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"gio.tools/icons"
	apppkg "sing-box-ez/internal/app"
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/framework/rpc"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// RemotePage is a minimal control surface for a remote sing-box-ez daemon.
type RemotePage struct {
	th      *material.Theme
	cfg     *config.AppConfig
	backend rpc.Backend

	mu         sync.Mutex
	status     apppkg.CoreStatusRes
	logLines   []string
	connected  bool
	address    string
	passphrase string
	stopPoll   chan struct{}

	connectBtn    widget.Clickable
	disconnectBtn widget.Clickable
	startBtn      widget.Clickable
	stopBtn       widget.Clickable
	restartBtn    widget.Clickable
	addrEd        widget.Editor
	passEd        widget.Editor
}

// NewRemotePage creates a remote control page.
func NewRemotePage(th *material.Theme, cfg *config.AppConfig) *RemotePage {
	p := &RemotePage{th: th, cfg: cfg}
	p.addrEd.SingleLine = true
	p.passEd.SingleLine = true
	p.passEd.Mask = '*'
	return p
}

// Tag returns the page identifier.
func (p *RemotePage) Tag() string { return "remote" }

// Name returns the localized page name.
func (p *RemotePage) Name() string { return "Remote" }

// Icon returns a settings-like icon for navigation.
func (p *RemotePage) Icon() *widget.Icon { return icons.ActionSettings }

// SetAddress sets the address editor text.
func (p *RemotePage) SetAddress(addr string) {
	p.addrEd.SetText(addr)
}

// SetBackend connects the page to a remote RPC backend.
func (p *RemotePage) SetBackend(backend rpc.Backend) {
	p.mu.Lock()
	p.backend = backend
	p.connected = backend != nil
	if p.stopPoll != nil {
		close(p.stopPoll)
	}
	p.stopPoll = nil
	if backend != nil {
		p.stopPoll = make(chan struct{})
		go p.pollLogs()
		_ = p.refreshStatusLocked()
	}
	p.mu.Unlock()
}

// Layout renders the remote control UI.
func (p *RemotePage) Layout(gtx layout.Context) layout.Dimensions {
	p.handleInteractions(gtx)

	p.mu.Lock()
	connected := p.connected
	status := p.status
	logLines := make([]string, len(p.logLines))
	copy(logLines, p.logLines)
	p.mu.Unlock()

	return layout.UniformInset(16).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Body1(p.th, "Remote daemon control").Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: 16}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if connected {
					return p.layoutConnected(gtx, status, logLines)
				}
				return p.layoutConnect(gtx)
			}),
		)
	})
}

func (p *RemotePage) layoutConnect(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(material.Editor(p.th, &p.addrEd, "tcp://127.0.0.1:8080 or unix:///path").Layout),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Rigid(material.Editor(p.th, &p.passEd, "passphrase (optional)").Layout),
		layout.Rigid(layout.Spacer{Height: 16}.Layout),
		layout.Rigid(material.Button(p.th, &p.connectBtn, "Connect").Layout),
	)
}

func (p *RemotePage) layoutConnected(gtx layout.Context, status apppkg.CoreStatusRes, logLines []string) layout.Dimensions {
	statusText := "not running"
	if status.Running {
		statusText = fmt.Sprintf("running (PID %d)", status.PID)
	}

	var lines string
	for _, l := range logLines {
		lines += l + "\n"
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(material.Body1(p.th, "Status: "+statusText).Layout),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, material.Button(p.th, &p.startBtn, "Start").Layout),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Flexed(1, material.Button(p.th, &p.stopBtn, "Stop").Layout),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Flexed(1, material.Button(p.th, &p.restartBtn, "Restart").Layout),
			)
		}),
		layout.Rigid(layout.Spacer{Height: 16}.Layout),
		layout.Rigid(material.Button(p.th, &p.disconnectBtn, "Disconnect").Layout),
		layout.Rigid(layout.Spacer{Height: 16}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.Body1(p.th, lines).Layout(gtx)
		}),
	)
}

func (p *RemotePage) handleInteractions(gtx layout.Context) {
	if p.connectBtn.Clicked(gtx) {
		p.address = strings.TrimSpace(p.addrEd.Text())
		p.passphrase = p.passEd.Text()
		// TODO: passphrase encryption is not yet implemented.
		_ = p.passphrase
		transport, err := rpc.ParseAddress(p.address)
		if err == nil {
			p.SetBackend(rpc.NewRemoteBackend(transport))
			if p.cfg != nil {
				if strings.HasPrefix(p.address, "tcp://") {
					_ = p.cfg.MustGet("remote", "last_tcp_address").Update(p.address)
				}
				_ = p.cfg.MustGet("remote", "last_passphrase").Update(p.passphrase)
				_ = p.cfg.Save()
			}
		}
	}
	if p.disconnectBtn.Clicked(gtx) {
		p.SetBackend(nil)
	}
	if p.startBtn.Clicked(gtx) {
		p.call("core", "start", rpc.Empty{})
		_ = p.refreshStatus()
	}
	if p.stopBtn.Clicked(gtx) {
		p.call("core", "stop", rpc.Empty{})
		_ = p.refreshStatus()
	}
	if p.restartBtn.Clicked(gtx) {
		p.call("core", "restart", rpc.Empty{})
		_ = p.refreshStatus()
	}
}

func (p *RemotePage) call(namespace, method string, args any) {
	p.mu.Lock()
	backend := p.backend
	p.mu.Unlock()
	if backend == nil {
		return
	}
	_ = backend.Call(context.Background(), namespace, method, args, nil)
}

func (p *RemotePage) refreshStatus() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.refreshStatusLocked()
}

func (p *RemotePage) refreshStatusLocked() error {
	if p.backend == nil {
		return nil
	}
	var status apppkg.CoreStatusRes
	if err := p.backend.Call(context.Background(), "core", "status", rpc.Empty{}, &status); err != nil {
		return err
	}
	p.status = status
	return nil
}

func (p *RemotePage) pollLogs() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-p.stopPoll:
			return
		case <-ticker.C:
			p.mu.Lock()
			backend := p.backend
			p.mu.Unlock()
			if backend == nil {
				continue
			}
			var lines []string
			if err := backend.Call(context.Background(), "log", "core_lines", rpc.Empty{}, &lines); err != nil {
				continue
			}
			p.mu.Lock()
			p.logLines = lines
			p.mu.Unlock()
		}
	}
}
