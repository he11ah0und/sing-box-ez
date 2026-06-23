// Package net provides a scoped HTTP client with logging and progress reporting
// for framework services.
package net

import (
	"context"
	"io"
	"net/http"
	"os"
	"time"

	"sing-box-ez/internal/framework/fs"
	"sing-box-ez/internal/framework/logger"
	"sing-box-ez/internal/framework/progress"
)

const defaultTimeout = 30 * time.Second

// Client is a scoped HTTP client that logs under its own terminal and reports
// progress for upload/download operations.
type Client struct {
	HTTP     *http.Client
	Log      *logger.LogTerminal
	Progress *progress.Config
}

// NewClient creates a Client with a logger allocated under parent.
func NewClient(parent *logger.LogTerminal) *Client {
	return &Client{
		HTTP:     &http.Client{Timeout: defaultTimeout},
		Log:      parent.Allocate("net"),
		Progress: nil,
	}
}

// NewClientWithProgress creates a Client with the given progress configuration.
func NewClientWithProgress(parent *logger.LogTerminal, cfg *progress.Config) *Client {
	c := NewClient(parent)
	c.Progress = cfg
	return c
}

// Do executes the provided request and returns the response. It validates the
// status code and logs errors.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	c.Log.Debugf("%s %s", req.Method, req.URL.String())
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, c.Log.Errorf("%s %s failed: %v", req.Method, req.URL.String(), err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
		_ = resp.Body.Close()
		return nil, c.Log.Errorf("%s %s returned %s: %s", req.Method, req.URL.String(), resp.Status, string(body))
	}
	return resp, nil
}

// GetReader performs a GET request and returns the response body together with
// the content length. The caller is responsible for closing the body.
func (c *Client) GetReader(ctx context.Context, url string) (io.ReadCloser, int64, error) {
	c.Log.Debugf("GET %s", url)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, c.Log.Errorf("create request for %s: %v", url, err)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, c.Log.Errorf("GET %s failed: %v", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
		_ = resp.Body.Close()
		return nil, 0, c.Log.Errorf("GET %s returned %s: %s", url, resp.Status, string(body))
	}
	return resp.Body, resp.ContentLength, nil
}

// GetBytes downloads url and returns the response body as bytes.
func (c *Client) GetBytes(ctx context.Context, url string) ([]byte, error) {
	r, total, err := c.GetReader(ctx, url)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	reporter := progress.NewReporter(c.Progress)
	pr := &progressReader{Reader: r, reporter: reporter, op: "download", label: url, total: total}

	data, err := io.ReadAll(pr)
	if err != nil {
		return nil, c.Log.Errorf("download %s failed: %v", url, err)
	}
	reporter.Finish("download", url, int64(len(data)))
	return data, nil
}

// Download streams url into w and reports progress.
func (c *Client) Download(ctx context.Context, url string, w io.Writer) error {
	r, total, err := c.GetReader(ctx, url)
	if err != nil {
		return err
	}
	defer r.Close()

	reporter := progress.NewReporter(c.Progress)
	pw := &progressWriter{Writer: w, reporter: reporter, op: "download", label: url, total: total}

	if _, err := io.Copy(pw, r); err != nil {
		return c.Log.Errorf("download %s failed: %v", url, err)
	}
	reporter.Finish("download", url, pw.current)
	return nil
}

// DownloadToFile downloads url and writes it to path using the given file system.
func (c *Client) DownloadToFile(ctx context.Context, fsys fs.FS, url, path string) error {
	c.Log.Infof("downloading %s → %s", url, path)
	dst := fsys.Root().File(path)
	f, err := dst.OpenFile(os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640)
	if err != nil {
		return c.Log.Errorf("create %q: %v", path, err)
	}

	if err := c.Download(ctx, url, f); err != nil {
		_ = f.Close()
		_ = dst.Remove()
		return err
	}
	if err := f.Close(); err != nil {
		_ = dst.Remove()
		return c.Log.Errorf("close %q: %v", path, err)
	}
	c.Log.Infof("downloaded %s", path)
	return nil
}

// Post performs a POST request with an optional body and headers, reporting
// upload progress when contentLength is known.
func (c *Client) Post(ctx context.Context, url string, body io.Reader, contentLength int64, headers http.Header) (*http.Response, error) {
	c.Log.Debugf("POST %s", url)
	return c.doWithBody(ctx, http.MethodPost, url, body, contentLength, headers)
}

// Put performs a PUT request with an optional body and headers, reporting
// upload progress when contentLength is known.
func (c *Client) Put(ctx context.Context, url string, body io.Reader, contentLength int64, headers http.Header) (*http.Response, error) {
	c.Log.Debugf("PUT %s", url)
	return c.doWithBody(ctx, http.MethodPut, url, body, contentLength, headers)
}

func (c *Client) doWithBody(ctx context.Context, method, url string, body io.Reader, contentLength int64, headers http.Header) (*http.Response, error) {
	reporter := progress.NewReporter(c.Progress)
	var r io.Reader
	if body != nil {
		r = &progressReader{Reader: body, reporter: reporter, op: "upload", label: url, total: contentLength}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, r)
	if err != nil {
		return nil, c.Log.Errorf("create %s request for %s: %v", method, url, err)
	}
	if contentLength >= 0 {
		req.ContentLength = contentLength
	}
	for k, v := range headers {
		req.Header[k] = v
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, c.Log.Errorf("%s %s failed: %v", method, url, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
		_ = resp.Body.Close()
		return nil, c.Log.Errorf("%s %s returned %s: %s", method, url, resp.Status, string(body))
	}
	if contentLength > 0 {
		reporter.Finish("upload", url, contentLength)
	}
	return resp, nil
}

// progressWriter wraps an io.Writer and reports progress.
type progressWriter struct {
	Writer   io.Writer
	reporter *progress.Reporter
	op       string
	label    string
	total    int64
	current  int64
}

func (w *progressWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	w.current += int64(n)
	w.reporter.Report(progress.State{Op: w.op, Label: w.label, Current: w.current, Total: w.total})
	return n, err
}

// progressReader wraps an io.Reader and reports progress.
type progressReader struct {
	Reader   io.Reader
	reporter *progress.Reporter
	op       string
	label    string
	total    int64
	current  int64
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.current += int64(n)
	r.reporter.Report(progress.State{Op: r.op, Label: r.label, Current: r.current, Total: r.total})
	return n, err
}
