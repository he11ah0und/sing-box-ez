package core

import (
	"fmt"

	"sing-box-ez/internal/singboxconfig"
)

// LogOverride holds the user-defined log level that replaces the active
// sing-box config's top-level "log" object before starting or validating core.
type LogOverride struct {
	Level string `msgpack:"level"`
}

// SetCoreLogOverride persists the core log level in the application config.
func (c *Controller) SetCoreLogOverride(o LogOverride) error {
	_ = c.cfg.MustGet("core", "log", "level").Update(o.Level)
	if err := c.cfg.Save(); err != nil {
		return c.terminal.TErrorf("core.log_override.save_failed", err)
	}
	c.terminal.TInfof("core.log_override.updated")
	return nil
}

func defaultCoreLogLevel(level string) string {
	if level == "" {
		return "error"
	}
	return level
}

func (c *Controller) currentLogOverride() LogOverride {
	return LogOverride{
		Level: defaultCoreLogLevel(c.cfg.MustGet("core", "log", "level").String()),
	}
}

// applyLogOverride replaces the top-level "log" section of the sing-box config
// so that logging is always enabled, timestamps are always included, and the
// output file is always removed. Only the log level is configurable.
func (c *Controller) applyLogOverride(data []byte) ([]byte, error) {
	o := c.currentLogOverride()

	version, _ := c.GetInstalledCoreVersion()
	parser, err := singboxconfig.NewConfigParserForVersion(version)
	if err != nil {
		c.terminal.TWarnf("core.invalid_core_version", version, err)
		parser = singboxconfig.NewConfigParser()
	}

	output, ok, err := parser.Override(data, func(tree map[string]any) bool {
		tree["log"] = map[string]any{
			"timestamp": true,
			"level":     o.Level,
		}
		return true
	})
	if err != nil {
		return nil, fmt.Errorf("apply log override: %w", err)
	}
	if !ok {
		c.terminal.TWarnf("core.log_override.unknown_fields")
	}
	return output, nil
}
