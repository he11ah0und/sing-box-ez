package updater

import "context"

// Apply abstracts how a downloaded update is installed.
type Apply interface {
	// Name returns the apply strategy identifier.
	Name() string

	// Apply installs the update described by info using source for asset download.
	Apply(ctx context.Context, source Source, info UpdateInfo, progress func(downloaded, total int64)) error
}
