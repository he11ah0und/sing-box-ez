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
	"sync"
	"time"
)

type Manager struct {
	mu         sync.Mutex
	cmd        *exec.Cmd
	cancel     context.CancelFunc
	running    bool
	configURL  string
	configName string
	elevated   bool
	logOutput  io.Writer
}

func NewManager(configURL string) *Manager {
	return &Manager{
		configURL: configURL,
	}
}

func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
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

func absPath(p string) (string, error) {
	if filepath.IsAbs(p) {
		return p, nil
	}
	return filepath.Abs(p)
}

func (m *Manager) buildCommand(ctx context.Context, corePath, configPath string) (*exec.Cmd, error) {
	if !m.elevated {
		return exec.CommandContext(ctx, corePath, "run", "-c", configPath), nil
	}

	switch runtime.GOOS {
	case "linux":
		// если setcap уже применён — запускаем напрямую без pkexec
		if HasNetAdminCapability(corePath) {
			return exec.CommandContext(ctx, corePath, "run", "-c", configPath), nil
		}
		absCore, err := absPath(corePath)
		if err != nil {
			return nil, fmt.Errorf("resolve core path: %w", err)
		}
		absConfig, err := absPath(configPath)
		if err != nil {
			return nil, fmt.Errorf("resolve config path: %w", err)
		}
		return exec.CommandContext(ctx, "pkexec", absCore, "run", "-c", absConfig), nil
	case "darwin":
		absCore, err := absPath(corePath)
		if err != nil {
			return nil, fmt.Errorf("resolve core path: %w", err)
		}
		absConfig, err := absPath(configPath)
		if err != nil {
			return nil, fmt.Errorf("resolve config path: %w", err)
		}
		script := fmt.Sprintf(`do shell script %s with administrator privileges`, strconv.Quote(absCore+" run -c "+absConfig))
		return exec.CommandContext(ctx, "osascript", "-e", script), nil
	case "windows":
		if m.elevated && !IsAdmin() {
			return nil, fmt.Errorf("administrator privileges required: please run sing-box-ez as administrator")
		}
		return exec.CommandContext(ctx, corePath, "run", "-c", configPath), nil
	default:
		return exec.CommandContext(ctx, corePath, "run", "-c", configPath), nil
	}
}

func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return fmt.Errorf("already running")
	}

	corePath := GetCorePath()
	if !CoreExists() {
		return fmt.Errorf("sing-box core not found at %s", corePath)
	}

	if m.configName == "" {
		return fmt.Errorf("no config name set")
	}
	configPath := GetConfigPath(m.configName)
	if _, err := os.Stat(configPath); err != nil {
		return fmt.Errorf("config not found at %s", configPath)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd, err := m.buildCommand(ctx, corePath, configPath)
	if err != nil {
		cancel()
		return err
	}
	m.cmd = cmd
	setProcessGroup(m.cmd)
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

	go func() {
		m.cmd.Wait()
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
	}()

	return nil
}

func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return nil
	}
	if m.cancel != nil {
		m.cancel()
	}
	if m.cmd != nil && m.cmd.Process != nil {
		// If setcap CAP_NET_ADMIN is applied, the process runs without
		// pkexec/sudo, so we don't need elevated kill either.
		elevated := m.elevated
		if runtime.GOOS == "linux" && HasNetAdminCapability(GetCorePath()) {
			elevated = false
		}
		if err := KillProcess(m.cmd.Process.Pid, elevated); err != nil {
			return fmt.Errorf("kill process: %w", err)
		}
	}
	m.running = false
	return nil
}

func (m *Manager) Restart() error {
	if err := m.Stop(); err != nil {
		return err
	}
	// Wait up to 5 seconds for the process to actually stop.
	for range 50 {
		if !m.IsRunning() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	return m.Start()
}

func (m *Manager) UpdateConfig() error {
	if m.configURL == "" {
		return fmt.Errorf("no URL configured")
	}
	if m.configName == "" {
		return fmt.Errorf("no config name set")
	}

	err := DownloadConfigFor(m.configName, m.configURL)
	if err != nil {
		if HasCachedConfig(m.configName) {
			return fmt.Errorf("download failed, using cached config: %w", err)
		}
		return err
	}
	return nil
}

func (m *Manager) UpdateCore(onProgress ProgressFunc) error {
	if m.running {
		if err := m.Stop(); err != nil {
			return err
		}
		time.Sleep(500 * time.Millisecond)
	}

	_, err := DownloadCore("", onProgress)
	return err
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

// CoreLogWriter перенаправляет stdout/stderr sing-box в GUI.
// Использует канал с буфером, чтобы не блокировать pipe процесса.
type CoreLogWriter struct {
	GUI    func(line string)
	Ch     chan string
	mu     sync.Mutex
	buf    []byte
	closed bool
}

func NewCoreLogWriter(gui func(string)) *CoreLogWriter {
	return &CoreLogWriter{
		GUI: gui,
		Ch:  make(chan string, 100),
	}
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
			// канал переполнен — дропаем строку
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
