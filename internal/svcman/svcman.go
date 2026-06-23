// Package svcman abstracts service installation and lifecycle management across
// different init systems and operating systems.
package svcman

// Status represents the runtime state of a service.
type Status int

const (
	StatusUnknown Status = iota
	StatusStopped
	StatusRunning
)

func (s Status) String() string {
	switch s {
	case StatusStopped:
		return "stopped"
	case StatusRunning:
		return "running"
	default:
		return "unknown"
	}
}

// InstallOptions describes the service to be installed.
type InstallOptions struct {
	DisplayName string
	Description string
	ExecPath    string
	Args        []string
	User        string // empty means root / LocalSystem
	WorkDir     string
}

// Manager abstracts interaction with a service manager (systemd, Windows
// services, OpenRC, etc.).
type Manager interface {
	// Name returns the human-readable name of the backend.
	Name() string

	// Available reports whether this backend can be used on the current system.
	Available() bool

	// IsInstalled reports whether the service is already registered.
	IsInstalled() bool

	// Install registers the service with the system.
	Install(opts InstallOptions) error

	// Remove unregisters the service.
	Remove() error

	// Start starts the service.
	Start() error

	// Stop stops the service.
	Stop() error

	// Restart restarts the service.
	Restart() error

	// Status returns the current runtime status.
	Status() (Status, error)
}
