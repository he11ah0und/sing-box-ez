package core

import (
	"strings"
	"sync"
	"time"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/framework/logger"
)

// CoreLogProcessor reads stdout/stderr lines from a running core process,
// optionally mirrors them into the application log, and handles fatal-error
// auto-restart logic.
type CoreLogProcessor struct {
	cfg      *config.AppConfig
	manager  *Manager
	writer   *CoreLogWriter
	terminal *logger.LogTerminal
	started  bool
	startMu  sync.Mutex
	ticker   *time.Ticker
	stopCh   chan struct{}
	wg       sync.WaitGroup

	lastAutoRestart time.Time
	autoRestartMu   sync.Mutex

	// OnAutoRestart is an optional callback invoked when a fatal core error triggers auto-restart.
	OnAutoRestart func()
}

// NewCoreLogProcessor creates a processor tied to the given manager, writer and logger root.
func NewCoreLogProcessor(cfg *config.AppConfig, manager *Manager, writer *CoreLogWriter, root *logger.LogTerminal) *CoreLogProcessor {
	return &CoreLogProcessor{
		cfg:      cfg,
		manager:  manager,
		writer:   writer,
		terminal: root.Allocate("core"),
		stopCh:   make(chan struct{}),
	}
}

// Start begins reading core process logs in a background goroutine.
func (p *CoreLogProcessor) Start() {
	p.startMu.Lock()
	defer p.startMu.Unlock()
	if p.started {
		return
	}
	p.started = true
	p.stopCh = make(chan struct{})
	p.ticker = time.NewTicker(100 * time.Millisecond)
	p.wg.Add(1)
	go p.readLoop()
}

// Stop signals the reader to finish, drains pending log lines and closes the underlying writer.
func (p *CoreLogProcessor) Stop() {
	p.startMu.Lock()
	started := p.started
	p.started = false
	p.startMu.Unlock()

	if !started {
		return
	}
	close(p.stopCh)
	p.wg.Wait()
	if p.ticker != nil {
		p.ticker.Stop()
	}
	if p.writer != nil {
		p.writer.Close()
	}
}

func (p *CoreLogProcessor) readLoop() {
	defer p.wg.Done()

	batch := make([]string, 0, 128)
	defer p.ticker.Stop()

	for {
		select {
		case line, ok := <-p.writer.Chan():
			if !ok {
				if len(batch) > 0 {
					p.processCoreLogs(batch)
				}
				return
			}
			batch = append(batch, line)

		case <-p.ticker.C:
			if len(batch) > 0 {
				p.processCoreLogs(batch)
				batch = batch[:0]
			}

		case <-p.stopCh:
			if len(batch) > 0 {
				p.processCoreLogs(batch)
				batch = batch[:0]
			}
			// Drain any lines already buffered in the writer channel without blocking.
			for {
				select {
				case line, ok := <-p.writer.Chan():
					if !ok {
						return
					}
					batch = append(batch, line)
					if len(batch) >= cap(batch) {
						p.processCoreLogs(batch)
						batch = batch[:0]
					}
				default:
					if len(batch) > 0 {
						p.processCoreLogs(batch)
					}
					return
				}
			}
		}
	}
}

func (p *CoreLogProcessor) processCoreLogs(lines []string) {
	watch := p.cfg.MustGet("core", "watch_logs").Bool()
	restart := p.cfg.MustGet("core", "auto_restart").Bool()
	if !watch && !restart {
		return
	}

	for _, msg := range lines {
		msg = strings.TrimSpace(msg)
		if msg == "" {
			continue
		}
		if restart && isCoreFatalError(msg) {
			p.autoRestartMu.Lock()
			if time.Since(p.lastAutoRestart) > 30*time.Second {
				p.lastAutoRestart = time.Now()
				p.autoRestartMu.Unlock()
				p.terminal.Infof("Detected core fatal error, auto-restarting...")
				go func() {
					if err := p.manager.Restart(); err != nil {
						p.terminal.Errorf("Auto-restart failed: %v", err)
					}
				}()
				if p.OnAutoRestart != nil {
					p.OnAutoRestart()
				}
			} else {
				p.autoRestartMu.Unlock()
			}
		}
		if watch {
			p.terminal.Infof("%s", msg)
		}
	}
}

// isCoreFatalError detects fatal error patterns in core output.
func isCoreFatalError(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "fatal[") ||
		strings.Contains(lower, "panic:")
}
