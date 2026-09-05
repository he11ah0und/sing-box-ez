package core

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"syscall"
)

// Download-failure kinds reported to the UI so it can phrase the dialog and
// decide whether a retry through the running core is likely to help.
const (
	// FailureRefused means the connection was refused or the host was
	// unreachable immediately — a strong hint that direct access is blocked.
	FailureRefused = "refused"
	// FailureTimeout means the request ran into a deadline.
	FailureTimeout = "timeout"
	// FailureDNS means the name could not be resolved.
	FailureDNS = "dns"
	// FailureHTTP means the server answered with a non-2xx status.
	FailureHTTP = "http"
	// FailureOther covers everything else.
	FailureOther = "other"
)

// DownloadErrorKind classifies a download/update error. The net backend keeps
// the cause chain (%w) so errors.Is/As reach the original network error.
func DownloadErrorKind(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
		return FailureTimeout
	}
	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EHOSTUNREACH) || errors.Is(err, syscall.ENETUNREACH) {
		return FailureRefused
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return FailureDNS
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Timeout() {
		return FailureTimeout
	}
	// Status failures are created by the net client as plain text without a
	// sentinel; match the request-status shape loosely via the URL error absence.
	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		return FailureHTTP
	}
	return FailureOther
}
