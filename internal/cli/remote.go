package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/framework"
	fwcli "sing-box-ez/internal/framework/cli"
	"sing-box-ez/internal/remote"
)

// remoteAddressFromContext returns the --remote global flag value or an empty string.
func remoteAddressFromContext(ctx *fwcli.Context) string {
	return fwcli.AsString(ctx.Global("remote"))
}

// connectRemote creates a remote client from the --remote global flag.
func connectRemote(ctx *fwcli.Context) (*remote.Client, error) {
	addr := remoteAddressFromContext(ctx)
	if addr == "" {
		return nil, fmt.Errorf("--remote is required")
	}
	transport, err := remote.ParseAddress(addr)
	if err != nil {
		return nil, err
	}
	return remote.Dial(transport)
}

// cmdRemote runs the sing-box-ez remote daemon.
func cmdRemote(app *framework.App, ctx *fwcli.Context) error {
	addr := fwcli.AsString(ctx.Arg("address"))
	if addr == "" {
		addr = "auto"
	}
	transport, err := remote.ParseAddress(addr)
	if err != nil {
		return err
	}

	cfg := app.Config.(*config.AppConfig)
	ctrl := core.NewController(cfg, app, app.Logger.Root)
	defer ctrl.Close()

	server := remote.NewServer(transport, ctrl)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	ctxRun, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-sigCh
		cancel()
	}()

	return server.Run(ctxRun)
}

// remoteStart sends core.start to the remote daemon.
func remoteStart(ctx *fwcli.Context) error {
	client, err := connectRemote(ctx)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.CoreStart(); err != nil {
		return err
	}
	status, err := client.CoreStatus()
	if err != nil {
		return err
	}
	fmt.Printf("sing-box started on remote (PID %d)\n", status.PID)
	return nil
}

// remoteStop sends core.stop to the remote daemon.
func remoteStop(ctx *fwcli.Context) error {
	client, err := connectRemote(ctx)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.CoreStop(); err != nil {
		return err
	}
	fmt.Println("sing-box stopped on remote")
	return nil
}

// remoteRestart sends core.restart to the remote daemon.
func remoteRestart(ctx *fwcli.Context) error {
	client, err := connectRemote(ctx)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.CoreRestart(); err != nil {
		return err
	}
	fmt.Println("sing-box restarted on remote")
	return nil
}

// remoteStatus prints the remote core status.
func remoteStatus(ctx *fwcli.Context) error {
	client, err := connectRemote(ctx)
	if err != nil {
		return err
	}
	defer client.Close()
	status, err := client.CoreStatus()
	if err != nil {
		return err
	}
	if status.Running {
		fmt.Printf("Status: running (PID %d)\n", status.PID)
	} else {
		fmt.Println("Status: not running")
	}
	return nil
}

// remoteUpdate downloads the active config on the remote daemon.
func remoteUpdate(cfg *config.AppConfig, ctx *fwcli.Context) error {
	active := cfg.GetActiveConfig()
	if active == nil || active.URL == "" {
		return fmt.Errorf("no active config URL set")
	}
	client, err := connectRemote(ctx)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.ConfigUpdate(active.Name, active.URL); err != nil {
		return err
	}
	fmt.Println("Config updated on remote")
	return nil
}

// remoteDownload downloads the latest core on the remote daemon.
func remoteDownload(ctx *fwcli.Context) error {
	client, err := connectRemote(ctx)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.CoreDownloadCore(); err != nil {
		return err
	}
	fmt.Println("Core downloaded on remote")
	return nil
}

// remoteShutdown asks the remote daemon to shut down.
func remoteShutdown(ctx *fwcli.Context) error {
	client, err := connectRemote(ctx)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.AppShutdown(); err != nil {
		return err
	}
	fmt.Println("Remote daemon shut down")
	return nil
}
