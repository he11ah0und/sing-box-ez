//go:build !aur

package updater

// SelfUpdateEnabled reports whether the self-update apply backend is part of
// this build. It is a compile-time constant so builds for external package
// managers drop the self-update code entirely (dead-code elimination).
const SelfUpdateEnabled = true

// ActiveChannel returns the update channel selected at compile time.
func ActiveChannel() ChannelInfo {
	return ChannelInfo{ID: "selfupdate", Name: "self-update"}
}

// ChannelCurrentVersion reports the running build's own version as seen by
// the active channel. For self-update builds this is the build metadata
// itself, so it cannot fail.
func ChannelCurrentVersion() (string, error) {
	return currentVersionLabel(""), nil
}
