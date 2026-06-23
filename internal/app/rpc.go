package app

import (
	"context"
	"time"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/framework/logger"
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

// EditConfigReq is the request for config/edit.
type EditConfigReq struct {
	OldName string          `msgpack:"old_name"`
	Rec     ConfigRecordMsg `msgpack:"rec"`
}

// UpdateAllRes is the response for config/update_all.
type UpdateAllRes struct {
	Updated int `msgpack:"updated"`
	Total   int `msgpack:"total"`
}

// PrivilegeTabStateMsg is a msgpack-friendly representation of core.PrivilegeTabState.
type PrivilegeTabStateMsg struct {
	Mode                string `msgpack:"mode"`
	IsAdmin             bool   `msgpack:"is_admin"`
	HasSetcap           bool   `msgpack:"has_setcap"`
	RunAsAdmin          bool   `msgpack:"run_as_admin"`
	AdminStatusText     string `msgpack:"admin_status_text"`
	AdminStatusColor    string `msgpack:"admin_status_color"`
	AdminLabel          string `msgpack:"admin_label"`
	PrivilegeText       string `msgpack:"privilege_text"`
	PrivilegeColor      string `msgpack:"privilege_color"`
	ShowRestartAdminBtn bool   `msgpack:"show_restart_admin_btn"`
	ShowSetcapBtn       bool   `msgpack:"show_setcap_btn"`
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
	registry.Register("core", "prepare_config", func(ctx context.Context, _ rpc.Empty) (ConfigRecordMsg, error) {
		rec, err := a.Controller.PrepareConfig()
		if err != nil {
			return ConfigRecordMsg{}, err
		}
		return configRecordToMsg(*rec), nil
	})
	registry.Register("core", "download", func(ctx context.Context, _ rpc.Empty) (CoreDownloadRes, error) {
		path, err := a.Controller.DownloadCore(nil)
		return CoreDownloadRes{Path: path}, err
	})
	registry.Register("core", "get_installed_version", func(ctx context.Context, _ rpc.Empty) (string, error) {
		return a.Controller.GetInstalledCoreVersion()
	})
	registry.Register("core", "get_latest_version", func(ctx context.Context, _ rpc.Empty) (string, error) {
		return a.Controller.GetLatestCoreVersion()
	})
	registry.Register("core", "get_privilege_tab_state", func(ctx context.Context, _ rpc.Empty) (PrivilegeTabStateMsg, error) {
		return privilegeStateToMsg(a.Controller.GetPrivilegeTabState()), nil
	})
	registry.Register("core", "restart_as_admin", func(ctx context.Context, _ rpc.Empty) (rpc.Empty, error) {
		return rpc.Empty{}, a.Controller.RestartAsAdmin()
	})
	registry.Register("core", "set_run_as_admin", func(ctx context.Context, req rpc.BoolValue) (rpc.Empty, error) {
		return rpc.Empty{}, a.Controller.SetRunAsAdmin(req.Value)
	})
	registry.Register("core", "set_auto_restart", func(ctx context.Context, req rpc.BoolValue) (rpc.Empty, error) {
		return rpc.Empty{}, a.Controller.SetAutoRestart(req.Value)
	})
	registry.Register("core", "apply_setcap", func(ctx context.Context, _ rpc.Empty) (rpc.Empty, error) {
		return rpc.Empty{}, a.Controller.ApplySetcap()
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
	registry.Register("config", "has_cached", func(ctx context.Context, req rpc.StringValue) (rpc.BoolValue, error) {
		return rpc.BoolValue{Value: a.Controller.HasCachedConfig(req.Value)}, nil
	})
	registry.Register("config", "delete", func(ctx context.Context, req rpc.StringValue) (rpc.Empty, error) {
		return rpc.Empty{}, a.Controller.DeleteConfig(req.Value)
	})
	registry.Register("config", "add", func(ctx context.Context, req ConfigRecordMsg) (rpc.Empty, error) {
		return rpc.Empty{}, a.Controller.AddConfig(msgToConfigRecord(req))
	})
	registry.Register("config", "edit", func(ctx context.Context, req EditConfigReq) (rpc.Empty, error) {
		return rpc.Empty{}, a.Controller.EditConfig(req.OldName, msgToConfigRecord(req.Rec))
	})
	registry.Register("config", "update_all", func(ctx context.Context, _ rpc.Empty) (UpdateAllRes, error) {
		updated, total, err := a.Controller.UpdateAllConfigs(nil)
		return UpdateAllRes{Updated: updated, Total: total}, err
	})

	registry.Register("log", "core_lines", func(ctx context.Context, _ rpc.Empty) ([]string, error) {
		return a.Controller.GetCoreLogLines(), nil
	})
	registry.Register("log", "core_clean_lines", func(ctx context.Context, _ rpc.Empty) ([]string, error) {
		return a.Controller.GetCoreLogCleanLines(), nil
	})
	registry.Register("log", "app_lines", func(ctx context.Context, _ rpc.Empty) ([]string, error) {
		return a.Controller.GetLogLines(), nil
	})
	registry.Register("log", "app_lines_at_least", func(ctx context.Context, req rpc.IntValue) ([]string, error) {
		return a.Controller.GetLogLinesAtLeast(logger.LogLevel(req.Value)), nil
	})
	registry.Register("log", "clear_logs", func(ctx context.Context, _ rpc.Empty) (rpc.Empty, error) {
		a.Controller.ClearLogs()
		return rpc.Empty{}, nil
	})
	registry.Register("log", "clear_core_logs", func(ctx context.Context, _ rpc.Empty) (rpc.Empty, error) {
		a.Controller.ClearCoreLogs()
		return rpc.Empty{}, nil
	})

	registry.Register("app", "open_data_dir", func(ctx context.Context, _ rpc.Empty) (rpc.Empty, error) {
		return rpc.Empty{}, a.Controller.OpenDataDir()
	})
	registry.Register("app", "set_log_limit", func(ctx context.Context, req rpc.IntValue) (rpc.Empty, error) {
		a.Controller.SetLogLimit(req.Value)
		return rpc.Empty{}, nil
	})
	registry.Register("app", "set_default_interval", func(ctx context.Context, req rpc.IntValue) (rpc.Empty, error) {
		a.Controller.SetDefaultInterval(req.Value)
		return rpc.Empty{}, nil
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

func msgToConfigRecord(msg ConfigRecordMsg) config.ConfigRecord {
	var last config.Timestamp
	if msg.LastUpdateUnix != 0 {
		last.Time = time.Unix(msg.LastUpdateUnix, 0)
	}
	return config.ConfigRecord{
		Name:                msg.Name,
		URL:                 msg.URL,
		UpdateIntervalHours: msg.UpdateIntervalHours,
		LastUpdate:          last,
		Parent:              msg.Parent,
		AutoUpdate:          msg.AutoUpdate,
	}
}

func privilegeStateToMsg(state core.PrivilegeTabState) PrivilegeTabStateMsg {
	return PrivilegeTabStateMsg{
		Mode:                state.Mode,
		IsAdmin:             state.IsAdmin,
		HasSetcap:           state.HasSetcap,
		RunAsAdmin:          state.RunAsAdmin,
		AdminStatusText:     state.AdminStatusText,
		AdminStatusColor:    state.AdminStatusColor,
		AdminLabel:          state.AdminLabel,
		PrivilegeText:       state.PrivilegeText,
		PrivilegeColor:      state.PrivilegeColor,
		ShowRestartAdminBtn: state.ShowRestartAdminBtn,
		ShowSetcapBtn:       state.ShowSetcapBtn,
	}
}
