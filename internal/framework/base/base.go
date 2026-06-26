// Package base provides a common base type for framework components that need
// hierarchical logging and consistent warnings for missing lookup keys.
package base

import "sing-box-ez/internal/framework/logger"

// Base holds a logger terminal and provides helpers for components such as
// config sheets and localization engines.
type Base struct {
	*logger.LogTerminal
}

// Init allocates a child logger terminal from parent with the given id.
func (b *Base) Init(parent *logger.LogTerminal, id string) {
	if parent != nil {
		b.LogTerminal = parent.Allocate(id)
	}
}

// WarnMissing logs a warning that a value for the given path was requested but
// does not exist.
func (b *Base) WarnMissing(path []string) {
	if b.LogTerminal != nil {
		b.Warnf("missing value for path %v", path)
	}
}
