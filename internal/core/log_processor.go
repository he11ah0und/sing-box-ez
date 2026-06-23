package core

import (
	"strings"
	"sync"
	"time"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/framework/logger"
)

// CoreLogProcessor reads stdout/stderr lines from a running core process,
// stores them in a dedicated CoreLogBuffer, and handles fatal-error auto-restart
// logic.
type CoreLogProcessor struct {
	cfg       *config.AppConfig
	manager   *Manager
	writer    *CoreLogWriter
	terminal  *logger.LogTerminal
	logBuffer *CoreLogBuffer
	started   bool
	startMu   sync.Mutex
	ticker    *time.Ticker
	stopCh    chan struct{}
	wg        sync.WaitGroup

	lastAutoRestart time.Time
	autoRestartMu   sync.Mutex

	// OnAutoRestart is an optional callback invoked when a fatal core error triggers auto-restart.
	OnAutoRestart func()

	subsMu      sync.RWMutex
	subscribers []*LogSubscription
}

// LogSubscription is a handle returned by AddSubscriber and used to unsubscribe.
type LogSubscription struct {
	fn func(string)
}

// NewCoreLogProcessor creates a processor tied to the given manager, writer and logger root.
func NewCoreLogProcessor(cfg *config.AppConfig, manager *Manager, writer *CoreLogWriter, root *logger.LogTerminal) *CoreLogProcessor {
	return &CoreLogProcessor{
		cfg:       cfg,
		manager:   manager,
		writer:    writer,
		terminal:  root.Allocate("core"),
		logBuffer: NewCoreLogBuffer(cfg.Int("log", "limit")),
		stopCh:    make(chan struct{}),
	}
}

// LogBuffer returns the dedicated core log buffer.
func (p *CoreLogProcessor) LogBuffer() *CoreLogBuffer {
	return p.logBuffer
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

func (p *CoreLogProcessor) AddSubscriber(fn func(string)) *LogSubscription {
	p.subsMu.Lock()
	defer p.subsMu.Unlock()
	s := &LogSubscription{fn: fn}
	p.subscribers = append(p.subscribers, s)
	return s
}

func (p *CoreLogProcessor) RemoveSubscriber(s *LogSubscription) {
	p.subsMu.Lock()
	defer p.subsMu.Unlock()
	for i, sub := range p.subscribers {
		if sub == s {
			p.subscribers = append(p.subscribers[:i], p.subscribers[i+1:]...)
			return
		}
	}
}

func (p *CoreLogProcessor) notifySubscribers(line string) {
	p.subsMu.RLock()
	subs := make([]*LogSubscription, len(p.subscribers))
	copy(subs, p.subscribers)
	p.subsMu.RUnlock()
	for _, sub := range subs {
		sub.fn(line)
	}
}

func (p *CoreLogProcessor) processCoreLogs(lines []string) {
	restart := p.cfg.MustGet("core", "auto_restart").Bool()

	for _, msg := range lines {
		msg = strings.TrimSpace(msg)
		if msg == "" {
			continue
		}
		p.logBuffer.Add(msg)
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
		p.notifySubscribers(msg)
	}
}

// isCoreFatalError detects fatal error patterns in core output.
func isCoreFatalError(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "fatal[") ||
		strings.Contains(lower, "panic:")
}
