package core

import (
	"context"
	"errors"
	"time"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/framework/logger"
	"sing-box-ez/internal/framework/rpc"
	"sing-box-ez/internal/singboxconfig"
)

// RemoteController implements Backend by forwarding calls to a remote RPC backend.
type RemoteController struct {
	backend  rpc.Backend
	cfg      *config.AppConfig
	terminal *logger.LogTerminal
}

// NewRemoteController creates a remote backend wrapper.
func NewRemoteController(backend rpc.Backend, cfg *config.AppConfig, terminal *logger.LogTerminal) *RemoteController {
	return &RemoteController{
		backend:  backend,
		cfg:      cfg,
		terminal: terminal,
	}
}

// Compile-time check that RemoteController implements Backend.
var _ Backend = (*RemoteController)(nil)

func (r *RemoteController) call(namespace, method string, args, reply any) error {
	return r.backend.Call(context.Background(), namespace, method, args, reply)
}

func (r *RemoteController) callEmpty(namespace, method string) error {
	return r.call(namespace, method, rpc.Empty{}, nil)
}

// Config returns the local application configuration.
func (r *RemoteController) Config() *config.AppConfig {
	return r.cfg
}

// Terminal returns the local logger terminal.
func (r *RemoteController) Terminal() *logger.LogTerminal {
	return r.terminal
}

// IsRunning reports whether the remote core is running.
func (r *RemoteController) IsRunning() bool {
	var res coreStatusRes
	if err := r.call("core", "status", rpc.Empty{}, &res); err != nil {
		return false
	}
	return res.Running
}

// GetPID returns the remote core process ID.
func (r *RemoteController) GetPID() int {
	var res coreStatusRes
	if err := r.call("core", "status", rpc.Empty{}, &res); err != nil {
		return 0
	}
	return res.PID
}

// Start starts the remote core.
func (r *RemoteController) Start() error {
	return r.callEmpty("core", "start")
}

// Stop stops the remote core.
func (r *RemoteController) Stop() error {
	return r.callEmpty("core", "stop")
}

// Restart restarts the remote core.
func (r *RemoteController) Restart() error {
	return r.callEmpty("core", "restart")
}

// PrepareConfig prepares the active config on the remote side.
func (r *RemoteController) PrepareConfig() (*config.ConfigRecord, error) {
	var res configRecordMsg
	if err := r.call("core", "prepare_config", rpc.Empty{}, &res); err != nil {
		return nil, err
	}
	rec := msgToConfigRecord(res)
	return &rec, nil
}

// GetConfigs returns the remote config list.
func (r *RemoteController) GetConfigs() []config.ConfigRecord {
	var res configListRes
	if err := r.call("config", "list", rpc.Empty{}, &res); err != nil {
		return nil
	}
	return msgSliceToConfigRecords(res.Configs)
}

// GetActiveConfig returns the remote active config.
func (r *RemoteController) GetActiveConfig() *config.ConfigRecord {
	var res configRecordMsg
	if err := r.call("config", "get_active", rpc.Empty{}, &res); err != nil {
		return nil
	}
	if res.Name == "" {
		return nil
	}
	rec := msgToConfigRecord(res)
	return &rec
}

// GetActiveName returns the remote active config name.
func (r *RemoteController) GetActiveName() string {
	var res configListRes
	if err := r.call("config", "list", rpc.Empty{}, &res); err != nil {
		return ""
	}
	return res.ActiveName
}

// ActivateConfig sets the active config on the remote side.
func (r *RemoteController) ActivateConfig(name string) error {
	return r.call("config", "set_active", setActiveReq{Name: name}, nil)
}

// AddConfig adds a config on the remote side.
func (r *RemoteController) AddConfig(rec config.ConfigRecord) error {
	return r.call("config", "add", configRecordToMsg(rec), nil)
}

// EditConfig edits a config on the remote side.
func (r *RemoteController) EditConfig(oldName string, rec config.ConfigRecord) error {
	return r.call("config", "edit", editConfigReq{OldName: oldName, Rec: configRecordToMsg(rec)}, nil)
}

// DeleteConfig deletes a config on the remote side.
func (r *RemoteController) DeleteConfig(name string) error {
	return r.call("config", "delete", rpc.StringValue{Value: name}, nil)
}

// UpdateConfigNow updates a single remote config.
func (r *RemoteController) UpdateConfigNow(name, url string) error {
	return r.call("config", "update", configUpdateReq{Name: name, URL: url}, nil)
}

// UpdateAllConfigs updates all remote configs. The local progress callback is not used.
func (r *RemoteController) UpdateAllConfigs(progress func(done, total int)) (int, int, error) {
	var res updateAllRes
	if err := r.call("config", "update_all", rpc.Empty{}, &res); err != nil {
		return 0, 0, err
	}
	if progress != nil {
		progress(res.Updated, res.Total)
	}
	return res.Updated, res.Total, nil
}

// HasCachedConfig reports whether the remote side has a cached config.
func (r *RemoteController) HasCachedConfig(name string) bool {
	var res rpc.BoolValue
	if err := r.call("config", "has_cached", rpc.StringValue{Value: name}, &res); err != nil {
		return false
	}
	return res.Value
}

// GetInstalledCoreVersion returns the installed core version from the remote side.
func (r *RemoteController) GetInstalledCoreVersion() (string, error) {
	var res string
	if err := r.call("core", "get_installed_version", rpc.Empty{}, &res); err != nil {
		return "", err
	}
	return res, nil
}

// GetLatestCoreVersion returns the latest available core version from the remote side.
func (r *RemoteController) GetLatestCoreVersion() (string, error) {
	var res string
	if err := r.call("core", "get_latest_version", rpc.Empty{}, &res); err != nil {
		return "", err
	}
	return res, nil
}

// DownloadCoreWithProgress downloads the core on the remote side.
// The local progress callback cannot be forwarded over the current RPC protocol.
func (r *RemoteController) DownloadCoreWithProgress(progress func(downloaded, total int64)) (string, error) {
	return r.DownloadCore(progress)
}

// DownloadCore downloads the core on the remote side.
func (r *RemoteController) DownloadCore(progress ProgressFunc) (string, error) {
	var res coreDownloadRes
	if err := r.call("core", "download", rpc.Empty{}, &res); err != nil {
		return "", err
	}
	return res.Path, nil
}

// GetPrivilegeTabState returns the remote privilege tab state.
func (r *RemoteController) GetPrivilegeTabState() PrivilegeTabState {
	var res privilegeTabStateMsg
	if err := r.call("core", "get_privilege_tab_state", rpc.Empty{}, &res); err != nil {
		return PrivilegeTabState{}
	}
	return msgToPrivilegeState(res)
}

// RestartAsAdmin is not available for remote controllers.
func (r *RemoteController) RestartAsAdmin() error {
	return errors.New("restart as admin is not available in remote mode")
}

// SetRunAsAdmin forwards the setting to the remote side.
func (r *RemoteController) SetRunAsAdmin(checked bool) error {
	return r.call("core", "set_run_as_admin", rpc.BoolValue{Value: checked}, nil)
}

// ApplySetcap is not available for remote controllers.
func (r *RemoteController) ApplySetcap() error {
	return errors.New("apply setcap is not available in remote mode")
}

// OpenDataDir is not available for remote controllers.
func (r *RemoteController) OpenDataDir() error {
	return errors.New("open data dir is not available in remote mode")
}

// OpenConfigFile opens the remote config file via the RPC backend.
func (r *RemoteController) OpenConfigFile(name string) error {
	if err := r.call("config", "open_file", rpc.StringValue{Value: name}, nil); err != nil {
		return errors.New("open config file is not available in remote mode")
	}
	return nil
}

// ValidateConfig validates the remote config via the RPC backend.
func (r *RemoteController) ValidateConfig(name string) (singboxconfig.ValidationResult, error) {
	var res singboxconfig.ValidationResult
	if err := r.call("config", "validate", rpc.StringValue{Value: name}, &res); err != nil {
		return singboxconfig.ValidationResult{}, err
	}
	return res, nil
}

// OpenConfigDir opens the remote config directory via the RPC backend.
func (r *RemoteController) OpenConfigDir(name string) error {
	if err := r.call("config", "open_dir", rpc.StringValue{Value: name}, nil); err != nil {
		return errors.New("open config directory is not available in remote mode")
	}
	return nil
}

// RecreateLocalConfig is not available for remote controllers.
func (r *RemoteController) RecreateLocalConfig(name string) error {
	return errors.New("recreate local config is not available in remote mode")
}

// SetLogLimit forwards the log limit to the remote side.
func (r *RemoteController) SetLogLimit(v int) {
	_ = r.call("app", "set_log_limit", rpc.IntValue{Value: v}, nil)
}

// SetDefaultInterval forwards the default interval to the remote side.
func (r *RemoteController) SetDefaultInterval(h int) {
	_ = r.call("app", "set_default_interval", rpc.IntValue{Value: h}, nil)
}

// SetAutoRestart forwards the auto-restart setting to the remote side.
func (r *RemoteController) SetAutoRestart(checked bool) error {
	return r.call("core", "set_auto_restart", rpc.BoolValue{Value: checked}, nil)
}

// SetCoreLogOverride forwards the core log override settings to the remote side.
func (r *RemoteController) SetCoreLogOverride(o LogOverride) error {
	return r.call("core", "set_log_override", o, nil)
}

// GetCoreLogLines returns raw core log lines from the remote side.
func (r *RemoteController) GetCoreLogLines() []string {
	var res []string
	if err := r.call("log", "core_lines", rpc.Empty{}, &res); err != nil {
		return nil
	}
	return res
}

// GetCoreLogCleanLines returns cleaned core log lines from the remote side.
func (r *RemoteController) GetCoreLogCleanLines() []string {
	var res []string
	if err := r.call("log", "core_clean_lines", rpc.Empty{}, &res); err != nil {
		return nil
	}
	return res
}

// GetLogLines returns application log lines from the remote side.
func (r *RemoteController) GetLogLines() []string {
	var res []string
	if err := r.call("log", "app_lines", rpc.Empty{}, &res); err != nil {
		return nil
	}
	return res
}

// GetLogLinesAtLeast returns application log lines at or above the given level.
func (r *RemoteController) GetLogLinesAtLeast(minLevel logger.LogLevel) []string {
	var res []string
	if err := r.call("log", "app_lines_at_least", rpc.IntValue{Value: int(minLevel)}, &res); err != nil {
		return nil
	}
	return res
}

// ClearCoreLogs clears core logs on the remote side.
func (r *RemoteController) ClearCoreLogs() {
	_ = r.callEmpty("log", "clear_core_logs")
}

// ClearLogs clears application logs on the remote side.
func (r *RemoteController) ClearLogs() {
	_ = r.callEmpty("log", "clear_logs")
}

// Mirror RPC message types (must match the server-side types in internal/app/rpc.go).

type coreStatusRes struct {
	Running bool `msgpack:"running"`
	PID     int  `msgpack:"pid"`
}

type coreDownloadRes struct {
	Path string `msgpack:"path"`
}

type configRecordMsg struct {
	Name                string `msgpack:"name"`
	URL                 string `msgpack:"url"`
	Type                string `msgpack:"type"`
	UpdateIntervalHours int    `msgpack:"update_interval_hours"`
	LastUpdateUnix      int64  `msgpack:"last_update_unix"`
	Parent              string `msgpack:"parent"`
	AutoUpdate          *bool  `msgpack:"auto_update,omitempty"`
}

type configListRes struct {
	ActiveName string            `msgpack:"active_name"`
	Configs    []configRecordMsg `msgpack:"configs"`
}

type setActiveReq struct {
	Name string `msgpack:"name"`
}

type configUpdateReq struct {
	Name string `msgpack:"name"`
	URL  string `msgpack:"url"`
}

type editConfigReq struct {
	OldName string          `msgpack:"old_name"`
	Rec     configRecordMsg `msgpack:"rec"`
}

type updateAllRes struct {
	Updated int `msgpack:"updated"`
	Total   int `msgpack:"total"`
}

type privilegeTabStateMsg struct {
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

func configRecordToMsg(rec config.ConfigRecord) configRecordMsg {
	var last int64
	if !rec.LastUpdate.IsZero() {
		last = rec.LastUpdate.Unix()
	}
	return configRecordMsg{
		Name:                rec.Name,
		URL:                 rec.URL,
		Type:                rec.Type,
		UpdateIntervalHours: rec.UpdateIntervalHours,
		LastUpdateUnix:      last,
		Parent:              rec.Parent,
		AutoUpdate:          rec.AutoUpdate,
	}
}

func msgToConfigRecord(msg configRecordMsg) config.ConfigRecord {
	var last config.Timestamp
	if msg.LastUpdateUnix != 0 {
		last.Time = time.Unix(msg.LastUpdateUnix, 0)
	}
	return config.ConfigRecord{
		Name:                msg.Name,
		URL:                 msg.URL,
		Type:                msg.Type,
		UpdateIntervalHours: msg.UpdateIntervalHours,
		LastUpdate:          last,
		Parent:              msg.Parent,
		AutoUpdate:          msg.AutoUpdate,
	}
}

func msgSliceToConfigRecords(msgs []configRecordMsg) []config.ConfigRecord {
	recs := make([]config.ConfigRecord, len(msgs))
	for i, msg := range msgs {
		recs[i] = msgToConfigRecord(msg)
	}
	return recs
}

func msgToPrivilegeState(msg privilegeTabStateMsg) PrivilegeTabState {
	return PrivilegeTabState(msg)
}
