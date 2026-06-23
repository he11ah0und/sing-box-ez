package app

import (
	"context"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/framework/rpc"
)

// CoreStatusRes is the response for core/status.
type CoreStatusRes struct {
	Running bool `msgpack:"running"`
	PID     int  `msgpack:"pid"`
}

// CoreDownloadRes is the response for core/download.
type CoreDownloadRes struct {
	Path string `msgpack:"path"`
}

// ConfigRecordMsg represents a config record in RPC messages.
type ConfigRecordMsg struct {
	Name                string `msgpack:"name"`
	URL                 string `msgpack:"url"`
	UpdateIntervalHours int    `msgpack:"update_interval_hours"`
	LastUpdateUnix      int64  `msgpack:"last_update_unix"`
	Parent              string `msgpack:"parent"`
	AutoUpdate          *bool  `msgpack:"auto_update,omitempty"`
}

// ConfigListRes is the response for config/list.
type ConfigListRes struct {
	ActiveName string            `msgpack:"active_name"`
	Configs    []ConfigRecordMsg `msgpack:"configs"`
}

// SetActiveReq is the request for config/set_active.
type SetActiveReq struct {
	Name string `msgpack:"name"`
}

// ConfigUpdateReq is the request for config/update.
type ConfigUpdateReq struct {
	Name string `msgpack:"name"`
	URL  string `msgpack:"url"`
}

// registerRPC registers application RPC methods on the registry.
func (a *App) registerRPC(registry *rpc.Registry) {
	registry.Register("core", "start", func(ctx context.Context, _ rpc.Empty) (rpc.Empty, error) {
		_, err := a.Controller.PrepareConfig()
		if err != nil {
			return rpc.Empty{}, err
		}
		return rpc.Empty{}, a.Controller.Start()
	})
	registry.Register("core", "stop", func(ctx context.Context, _ rpc.Empty) (rpc.Empty, error) {
		return rpc.Empty{}, a.Controller.Stop()
	})
	registry.Register("core", "restart", func(ctx context.Context, _ rpc.Empty) (rpc.Empty, error) {
		return rpc.Empty{}, a.Controller.Restart()
	})
	registry.Register("core", "status", func(ctx context.Context, _ rpc.Empty) (CoreStatusRes, error) {
		return CoreStatusRes{Running: a.Controller.IsRunning(), PID: a.Controller.GetPID()}, nil
	})
	registry.Register("core", "download", func(ctx context.Context, _ rpc.Empty) (CoreDownloadRes, error) {
		path, err := a.Controller.DownloadCore(nil)
		return CoreDownloadRes{Path: path}, err
	})

	registry.Register("config", "list", func(ctx context.Context, _ rpc.Empty) (ConfigListRes, error) {
		cfg := a.Config.(*config.AppConfig)
		recs := cfg.GetConfigs()
		msgs := make([]ConfigRecordMsg, len(recs))
		for i, rec := range recs {
			msgs[i] = configRecordToMsg(rec)
		}
		return ConfigListRes{ActiveName: cfg.GetActiveName(), Configs: msgs}, nil
	})
	registry.Register("config", "get_active", func(ctx context.Context, _ rpc.Empty) (ConfigRecordMsg, error) {
		rec := a.Config.(*config.AppConfig).GetActiveConfig()
		if rec == nil {
			return ConfigRecordMsg{}, nil
		}
		return configRecordToMsg(*rec), nil
	})
	registry.Register("config", "set_active", func(ctx context.Context, req SetActiveReq) (rpc.Empty, error) {
		return rpc.Empty{}, a.Controller.ActivateConfig(req.Name)
	})
	registry.Register("config", "update", func(ctx context.Context, req ConfigUpdateReq) (rpc.Empty, error) {
		return rpc.Empty{}, a.Controller.UpdateConfigNow(req.Name, req.URL)
	})

	registry.Register("app", "shutdown", func(ctx context.Context, _ rpc.Empty) (rpc.Empty, error) {
		a.Controller.Close()
		return rpc.Empty{}, nil
	})
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
