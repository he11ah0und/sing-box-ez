package pages

import (
	"fmt"
	"strings"

	"gio.tools/icons"
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/remote"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// RemotePage is a minimal control surface for a remote sing-box-ez daemon.
type RemotePage struct {
	th         *material.Theme
	cfg        *config.AppConfig
	client     *remote.Client
	status     remote.CoreStatusRes
	logLines   []string
	connected  bool
	address    string
	passphrase string

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

// SetClient connects the page to a remote client.
func (p *RemotePage) SetClient(client *remote.Client) {
	p.client = client
	p.connected = client != nil
	if client != nil {
		client.SetOnLog(func(line string) {
			p.logLines = append(p.logLines, line)
			if len(p.logLines) > 500 {
				p.logLines = p.logLines[len(p.logLines)-500:]
			}
		})
		_ = p.refreshStatus()
	}
}

// Layout renders the remote control UI.
func (p *RemotePage) Layout(gtx layout.Context) layout.Dimensions {
	p.handleInteractions(gtx)

	return layout.UniformInset(16).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Body1(p.th, "Remote daemon control").Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: 16}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if p.connected {
					return p.layoutConnected(gtx)
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

func (p *RemotePage) layoutConnected(gtx layout.Context) layout.Dimensions {
	status := "not running"
	if p.status.Running {
		status = fmt.Sprintf("running (PID %d)", p.status.PID)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(material.Body1(p.th, "Status: "+status).Layout),
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
			// Render last log lines.
			var lines string
			for _, l := range p.logLines {
				lines += l + "\n"
			}
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
		transport, err := remote.ParseAddress(p.address)
		if err == nil {
			client, err := remote.Dial(transport)
			if err == nil {
				p.SetClient(client)
				if p.cfg != nil {
					if strings.HasPrefix(p.address, "tcp://") {
						_ = p.cfg.MustGet("remote", "last_tcp_address").Update(p.address)
					}
					_ = p.cfg.MustGet("remote", "last_passphrase").Update(p.passphrase)
					_ = p.cfg.Save()
				}
			}
		}
	}
	if p.disconnectBtn.Clicked(gtx) && p.client != nil {
		_ = p.client.Close()
		p.client = nil
		p.connected = false
	}
	if p.startBtn.Clicked(gtx) && p.client != nil {
		_ = p.client.CoreStart()
		_ = p.refreshStatus()
	}
	if p.stopBtn.Clicked(gtx) && p.client != nil {
		_ = p.client.CoreStop()
		_ = p.refreshStatus()
	}
	if p.restartBtn.Clicked(gtx) && p.client != nil {
		_ = p.client.CoreRestart()
		_ = p.refreshStatus()
	}
}

func (p *RemotePage) refreshStatus() error {
	if p.client == nil {
		return nil
	}
	status, err := p.client.CoreStatus()
	if err != nil {
		return err
	}
	p.status = status
	return nil
}
