package updater

import "errors"

// errNotExternalChannel is returned by ChannelLatestVersion on self-update
// builds, where the latest version comes from the release source instead.
var errNotExternalChannel = errors.New("not an externally managed channel")
