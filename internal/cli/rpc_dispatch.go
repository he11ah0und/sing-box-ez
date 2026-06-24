package cli

import (
	"context"
	"fmt"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/framework"
	fwcli "sing-box-ez/internal/framework/cli"
	"sing-box-ez/internal/framework/rpc"
)

// remoteRequest/response types mirror the server-side types in internal/app/rpc.go.
type (
	coreStatusRes struct {
		Running bool `msgpack:"running"`
		PID     int  `msgpack:"pid"`
	}

	coreDownloadRes struct {
		Path string `msgpack:"path"`
	}

	configUpdateReq struct {
		Name string `msgpack:"name"`
		URL  string `msgpack:"url"`
	}
)

// remoteAddressFromContext returns the --remote global flag value or an empty string.
func remoteAddressFromContext(ctx *fwcli.Context) string {
	return fwcli.AsString(ctx.Global("remote"))
}

// dispatchRemoteCommand runs a supported CLI command against a remote daemon
// using the framework RPC backend.
func dispatchRemoteCommand(app *framework.App, ctx *fwcli.Context) error {
	backend := app.Backend
	if backend == nil {
		return fmt.Errorf("remote backend not configured")
	}

	cfg := app.Config.(*config.AppConfig)

	dispatchers := map[string]func(rpc.Backend, *config.AppConfig) error{
		"start":    remoteStart,
		"stop":     remoteStop,
		"restart":  remoteRestart,
		"status":   remoteStatus,
		"update":   remoteUpdate,
		"download": remoteDownload,
	}

	fn, ok := dispatchers[ctx.Command()]
	if !ok {
		return fmt.Errorf("command %q is not supported with --remote", ctx.Command())
	}
	return fn(backend, cfg)
}

func remoteStart(backend rpc.Backend, cfg *config.AppConfig) error {
	if err := backend.Call(context.Background(), "core", "start", rpc.Empty{}, nil); err != nil {
		return err
	}
	var status coreStatusRes
	if err := backend.Call(context.Background(), "core", "status", rpc.Empty{}, &status); err != nil {
		return err
	}
	fmt.Printf("sing-box started on remote (PID %d)\n", status.PID)
	return nil
}

func remoteStop(backend rpc.Backend, cfg *config.AppConfig) error {
	if err := backend.Call(context.Background(), "core", "stop", rpc.Empty{}, nil); err != nil {
		return err
	}
	fmt.Println("sing-box stopped on remote")
	return nil
}

func remoteRestart(backend rpc.Backend, cfg *config.AppConfig) error {
	if err := backend.Call(context.Background(), "core", "restart", rpc.Empty{}, nil); err != nil {
		return err
	}
	fmt.Println("sing-box restarted on remote")
	return nil
}

func remoteStatus(backend rpc.Backend, cfg *config.AppConfig) error {
	var status coreStatusRes
	if err := backend.Call(context.Background(), "core", "status", rpc.Empty{}, &status); err != nil {
		return err
	}
	if status.Running {
		fmt.Printf("Status: running (PID %d)\n", status.PID)
	} else {
		fmt.Println("Status: not running")
	}
	return nil
}

func remoteUpdate(backend rpc.Backend, cfg *config.AppConfig) error {
	active := cfg.GetActiveConfig()
	if active == nil || active.URL == "" {
		return fmt.Errorf("no active config URL set")
	}
	if err := backend.Call(context.Background(), "config", "update", configUpdateReq{Name: active.Name, URL: active.URL}, nil); err != nil {
		return err
	}
	fmt.Println("Config updated on remote")
	return nil
}

func remoteDownload(backend rpc.Backend, cfg *config.AppConfig) error {
	var res coreDownloadRes
	if err := backend.Call(context.Background(), "core", "download", rpc.Empty{}, &res); err != nil {
		return err
	}
	fmt.Printf("Core downloaded on remote: %s\n", res.Path)
	return nil
}
