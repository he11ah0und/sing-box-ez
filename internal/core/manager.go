package core

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/he11ah0und/logger"
	"sing-box-ez/internal/framework/fs"
	"sing-box-ez/internal/framework/net"
	"sing-box-ez/internal/framework/rpc"
	"sing-box-ez/internal/framework/updater"
)

// Manager manages the sing-box core process and its artifacts.
type Manager struct {
	mu              sync.Mutex
	cmd             *exec.Cmd
	cancel          context.CancelFunc
	running         bool
	waitDone        chan struct{}
	configURL       string
	configName      string
	tempConfigPath  string
	elevated        bool
	logOutput       io.Writer
	configTransform func([]byte) ([]byte, error)

	baseDir string
	fsys    fs.FS
	net     *net.Client
	updater *updater.Manager
	log     *logger.Logger

	// binaryPath, when set, resolves the core binary path on every call so a
	// core-source mode switch (official/system/custom) takes effect without
	// rebuilding the manager.
	binaryPath func() string
}

// ProgressFunc is called during downloads: downloaded, total.
type ProgressFunc = rpc.ProgressFunc

// NewManager creates a new core manager for the given base directory and framework services.
func NewManager(baseDir string, fsys fs.FS, updater *updater.Manager, log *logger.Logger) *Manager {
	return &Manager{
		baseDir: baseDir,
		fsys:    fsys,
		net:     net.NewClient(log.Root),
		updater: updater,
		log:     log,
	}
}

// SetBinaryResolver installs a resolver for the core binary path (core
// source mode support); coreBinary falls back to the managed path when unset.
func (m *Manager) SetBinaryResolver(fn func() string) {
	m.binaryPath = fn
}

func (m *Manager) coreBinary() string {
	if m.binaryPath != nil {
		return m.binaryPath()
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(m.baseDir, "sing-box.exe")
	}
	return filepath.Join(m.baseDir, "sing-box")
}

func (m *Manager) cachedConfig(name string) string {
	return filepath.Join(m.baseDir, "configs", name+".json")
}

func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	pid := 0
	if m.cmd != nil && m.cmd.Process != nil {
		pid = m.cmd.Process.Pid
	}
	m.mu.Unlock()
	if pid <= 0 {
		return false
	}
	return ProcessExists(pid)
}

func (m *Manager) GetPID() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd != nil && m.cmd.Process != nil {
		return m.cmd.Process.Pid
	}
	return 0
}

func (m *Manager) SetLogOutput(w io.Writer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logOutput = w
}

// SetConfigTransform sets a function that transforms config bytes before they
// are passed to the core process.
func (m *Manager) SetConfigTransform(fn func([]byte) ([]byte, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configTransform = fn
}

func (m *Manager) absPath(p string) (string, error) {
	if filepath.IsAbs(p) {
		return p, nil
	}
	return filepath.Abs(p)
}

func (m *Manager) buildCommand(ctx context.Context, corePath, configArg string) (*exec.Cmd, error) {
	if !m.elevated {
		// #nosec G204 — corePath and configArg are internal managed values.
		return exec.CommandContext(ctx, corePath, "run", "-c", configArg), nil
	}

	switch runtime.GOOS {
	case "linux":
		if HasNetAdminCapability(corePath) {
			// #nosec G204 — corePath and configArg are internal managed values.
			return exec.CommandContext(ctx, corePath, "run", "-c", configArg), nil
		}
		// No pkexec fallback: a pkexec-launched core is an independent root
		// process that survives app exit (orphan).
		return nil, fmt.Errorf("administrator privileges not available: apply setcap to the core binary")
	case "darwin":
		absCore, err := m.absPath(corePath)
		if err != nil {
			return nil, fmt.Errorf("resolve core path: %w", err)
		}
		script := fmt.Sprintf(`do shell script %s with administrator privileges`, strconv.Quote(absCore+" run -c "+configArg))
		// #nosec G204 — osascript is a system binary; script is built from resolved internal paths.
		return exec.CommandContext(ctx, "osascript", "-e", script), nil
	case "windows":
		if m.elevated && !IsAdmin() {
			return nil, fmt.Errorf("administrator privileges required: please run sing-box-ez as administrator")
		}
		// #nosec G204 — corePath and configArg are internal managed values.
		return exec.CommandContext(ctx, corePath, "run", "-c", configArg), nil
	default:
		// #nosec G204 — corePath and configArg are internal managed values.
		return exec.CommandContext(ctx, corePath, "run", "-c", configArg), nil
	}
}

// ReadConfig reads the cached config bytes for the currently selected config name.
func (m *Manager) ReadConfig() ([]byte, error) {
	if m.configName == "" {
		return nil, fmt.Errorf("no config name set")
	}
	return m.ReadConfigByName(m.configName)
}

// ReadConfigByName reads the cached config bytes for the given config name.
func (m *Manager) ReadConfigByName(name string) ([]byte, error) {
	configPath := m.cachedConfig(name)
	data, err := m.fsys.Root().File(configPath).Read()
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", configPath, err)
	}
	return data, nil
}

func (m *Manager) Start() error {
	data, err := m.ReadConfig()
	if err != nil {
		return err
	}
	m.mu.Lock()
	transform := m.configTransform
	m.mu.Unlock()
	if transform != nil {
		data, err = transform(data)
		if err != nil {
			return err
		}
	}
	return m.StartWithConfig(data)
}

// StartWithConfig starts the core process with the provided config bytes.
// The config is passed through stdin (sing-box run -c stdin) except on
// elevated macOS, where a temporary file is used because osascript cannot
// forward stdin to the privileged child process.
func (m *Manager) StartWithConfig(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return fmt.Errorf("already running")
	}
	if m.cmd != nil && m.cmd.Process != nil && ProcessExists(m.cmd.Process.Pid) {
		return fmt.Errorf("core process already running (PID %d)", m.cmd.Process.Pid)
	}

	corePath := m.coreBinary()
	if _, err := m.fsys.Root().File(corePath).Stat(); err != nil {
		return fmt.Errorf("sing-box core not found at %s", corePath)
	}

	configArg := "stdin"
	var stdin io.Reader = bytes.NewReader(data)
	if runtime.GOOS == "darwin" && m.elevated {
		tmpPath, err := m.writeTempConfig(data)
		if err != nil {
			return fmt.Errorf("write temp config: %w", err)
		}
		m.tempConfigPath = tmpPath
		configArg = tmpPath
		stdin = nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd, err := m.buildCommand(ctx, corePath, configArg)
	if err != nil {
		cancel()
		return err
	}
	m.cmd = cmd
	setProcessGroup(m.cmd)
	if stdin != nil {
		m.cmd.Stdin = stdin
	}
	if m.logOutput != nil {
		m.cmd.Stdout = m.logOutput
		m.cmd.Stderr = m.logOutput
	} else {
		m.cmd.Stdout = os.Stdout
		m.cmd.Stderr = os.Stderr
	}

	if err := m.cmd.Start(); err != nil {
		cancel()
		return err
	}

	m.cancel = cancel
	m.running = true
	m.waitDone = make(chan struct{})

	startedCmd := m.cmd
	go func() {
		_ = startedCmd.Wait()
		m.mu.Lock()
		if m.cmd == startedCmd {
			m.running = false
		}
		m.mu.Unlock()
		close(m.waitDone)
	}()

	return nil
}

func (m *Manager) writeTempConfig(data []byte) (string, error) {
	tmpDir := filepath.Join(m.baseDir, "configs", ".tmp")
	if err := m.fsys.Root().Subdir(tmpDir).MkdirAll(0750); err != nil {
		return "", err
	}
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("stdin-%d.json", os.Getpid()))
	if err := m.fsys.Root().File(tmpFile).AtomicWrite(data, 0640); err != nil {
		return "", err
	}
	return tmpFile, nil
}

func (m *Manager) Stop() error {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return nil
	}
	m.running = false

	var killErr error
	var done chan struct{}
	if m.cmd != nil && m.cmd.Process != nil {
		elevated := m.elevated
		if runtime.GOOS == "linux" && HasNetAdminCapability(m.coreBinary()) {
			elevated = false
		}
		done = m.waitDone
		if ProcessExists(m.cmd.Process.Pid) {
			if err := KillProcess(m.cmd.Process.Pid, elevated); err != nil {
				killErr = fmt.Errorf("kill process: %w", err)
			}
		}
	}
	if m.cancel != nil {
		m.cancel()
	}
	tmpPath := m.tempConfigPath
	m.tempConfigPath = ""
	m.mu.Unlock()

	if done != nil {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
		}
	}
	if tmpPath != "" {
		_ = m.fsys.Root().File(tmpPath).Remove()
	}
	return killErr
}

func (m *Manager) Restart() error {
	if err := m.Stop(); err != nil {
		return err
	}
	for range 100 {
		if !m.IsRunning() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	return m.Start()
}

func (m *Manager) UpdateConfig(ctx context.Context) ([]byte, error) {
	if m.configURL == "" {
		return nil, fmt.Errorf("no URL configured")
	}
	if m.configName == "" {
		return nil, fmt.Errorf("no config name set")
	}

	data, err := m.DownloadConfigFor(ctx, m.configName, m.configURL)
	if err != nil {
		if m.hasCachedConfig(m.configName) {
			return nil, fmt.Errorf("download failed, using cached config: %w", err)
		}
		return nil, err
	}
	return data, nil
}

func (m *Manager) hasCachedConfig(name string) bool {
	return m.fsys.Root().File(m.cachedConfig(name)).Exists()
}

func (m *Manager) DownloadConfigFor(ctx context.Context, name, url string) ([]byte, error) {
	return m.downloadConfigFor(ctx, name, url, m.net)
}

// DownloadConfigForWith is DownloadConfigFor with an explicit HTTP client —
// used to route the download through the running core's local proxy.
func (m *Manager) DownloadConfigForWith(ctx context.Context, name, url string, client *net.Client) ([]byte, error) {
	return m.downloadConfigFor(ctx, name, url, client)
}

func (m *Manager) downloadConfigFor(ctx context.Context, name, url string, client *net.Client) ([]byte, error) {
	path := m.cachedConfig(name)
	if err := m.fsys.Root().Subdir("configs").MkdirAll(0750); err != nil {
		return nil, err
	}
	data, err := client.GetBytes(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := m.fsys.Root().File(path).AtomicWrite(data, 0640); err != nil {
		return nil, err
	}
	return data, nil
}

// CreateLocalConfig creates an empty local config file for the given name.
func (m *Manager) CreateLocalConfig(name string) error {
	path := m.cachedConfig(name)
	if err := m.fsys.Root().Subdir("configs").MkdirAll(0750); err != nil {
		return err
	}
	return m.fsys.Root().File(path).AtomicWrite([]byte("{}"), 0640)
}

// RenameConfigFile renames a cached config file when a config is renamed.
func (m *Manager) RenameConfigFile(oldName, newName string) error {
	oldPath := m.cachedConfig(oldName)
	newPath := m.cachedConfig(newName)
	if !m.fsys.Root().File(oldPath).Exists() {
		return nil
	}
	if m.fsys.Root().File(newPath).Exists() {
		return nil
	}
	return m.fsys.Root().File(oldPath).Rename(filepath.Base(newPath))
}

// DeleteConfigFile removes the cached config file for the given name. A
// missing file is not an error.
func (m *Manager) DeleteConfigFile(name string) error {
	f := m.fsys.Root().File(m.cachedConfig(name))
	if !f.Exists() {
		return nil
	}
	return f.Remove()
}

func (m *Manager) CheckCoreUpdate(ctx context.Context) (*updater.UpdateInfo, error) {
	return m.CheckCoreUpdateChannel(ctx, CoreChannelStable)
}

// CheckCoreUpdateChannel checks for a core update on the given release
// channel: stable tracks the latest stable release, beta also considers
// pre-releases.
func (m *Manager) CheckCoreUpdateChannel(ctx context.Context, channel string) (*updater.UpdateInfo, error) {
	if m.updater == nil {
		return nil, fmt.Errorf("core updater not configured")
	}
	current, _ := GetCoreVersion(m.coreBinary())
	if current != "" && !strings.HasPrefix(current, "v") {
		current = "v" + current
	}
	if channel == CoreChannelBeta {
		gh, ok := m.updater.Source.(*updater.GitHubBackend)
		if !ok {
			return nil, fmt.Errorf("core updater source does not support channels")
		}
		releases, err := gh.ListReleases(ctx)
		if err != nil {
			return nil, err
		}
		if len(releases) == 0 {
			return &updater.UpdateInfo{Current: current, Latest: current, ReleaseCount: 0}, nil
		}
		return m.updater.CheckVersion(ctx, releases[0].Version, current)
	}
	return m.updater.CheckWithCurrent(ctx, "", current)
}

func (m *Manager) DownloadCore(onProgress ProgressFunc) (string, error) {
	if err := m.UpdateCore(onProgress); err != nil {
		return "", err
	}
	return m.coreBinary(), nil
}

func (m *Manager) UpdateCore(onProgress ProgressFunc) error {
	info, err := m.CheckCoreUpdate(context.Background())
	if err != nil {
		return err
	}
	if info.ReleaseCount == 0 {
		return nil
	}
	return m.installCoreUpdate(info, onProgress)
}

// ListCoreReleases returns the available core releases, newest first
// (prereleases included) — the official-mode version picker.
func (m *Manager) ListCoreReleases(ctx context.Context) ([]updater.Release, error) {
	if m.updater == nil {
		return nil, fmt.Errorf("core updater not configured")
	}
	gh, ok := m.updater.Source.(*updater.GitHubBackend)
	if !ok {
		return nil, fmt.Errorf("core updater source does not support listing releases")
	}
	return gh.ListReleases(ctx)
}

func (m *Manager) installCoreUpdate(info *updater.UpdateInfo, onProgress ProgressFunc) error {
	info.Files = []updater.UpdateFile{{
		Asset:    info.Asset,
		DestPath: ".",
	}}

	wasRunning := m.IsRunning()

	stopCore := func() error {
		if !m.IsRunning() {
			return nil
		}
		if err := m.Stop(); err != nil {
			return fmt.Errorf("stop core for update: %w", err)
		}
		for range 100 {
			if !m.IsRunning() {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if m.IsRunning() {
			return fmt.Errorf("core process did not stop in time for update")
		}
		return nil
	}

	if fa, ok := m.updater.Apply.(*updater.FilesUpdateApply); ok {
		fa.BeforeInstall = stopCore
		defer func() { fa.BeforeInstall = nil }()
	} else {
		if err := stopCore(); err != nil {
			return err
		}
	}

	if err := m.updater.Install(context.Background(), info, onProgress); err != nil {
		return err
	}

	if wasRunning {
		m.log.Root.TInfof("core.manager.restarting_after_update")
		if err := m.Start(); err != nil {
			m.log.Root.TInfof("core.manager.restart_after_update_failed", err)
		}
	}

	return nil
}

func (m *Manager) SetConfigURL(url string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configURL = url
}

func (m *Manager) SetConfigName(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configName = name
}

func (m *Manager) SetElevated(v bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.elevated = v
}

// CoreLogWriter redirects sing-box stdout/stderr into a channel for the GUI.
type CoreLogWriter struct {
	Ch     chan string
	mu     sync.Mutex
	buf    []byte
	closed bool
}

func NewCoreLogWriter() *CoreLogWriter {
	return &CoreLogWriter{
		Ch: make(chan string, 1024),
	}
}

func (w *CoreLogWriter) Chan() <-chan string {
	return w.Ch
}

func (w *CoreLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, fmt.Errorf("log writer is closed")
	}
	w.buf = append(w.buf, p...)
	for {
		idx := bytes.IndexByte(w.buf, '\n')
		if idx < 0 {
			break
		}
		line := string(w.buf[:idx])
		w.buf = w.buf[idx+1:]
		select {
		case w.Ch <- line:
		default:
		}
	}
	return len(p), nil
}

func (w *CoreLogWriter) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	w.closed = true
	close(w.Ch)
}
