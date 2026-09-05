package updater

// ChannelInfo describes the update channel this build of the application
// belongs to. The channel is selected at compile time via build tags:
// a default build self-updates from release sources declared in the project
// spec, while builds made for an external package manager (e.g. tag "aur"
// for Arch Linux AUR packages) carry no self-update capability at all.
type ChannelInfo struct {
	// ID is the machine-readable channel id ("selfupdate", "aur", ...).
	ID string `json:"id"`
	// Name is the human-facing channel label ("self-update", "AUR", ...).
	Name string `json:"name"`
	// External reports whether updates are managed by an external package
	// manager. External builds expose no self-update check or install.
	External bool `json:"external"`
}
