# Agent Notes

## Logging policy

The project uses `internal/framework/logger.LogTerminal` for all logging.
To avoid duplicate messages in the log buffer, follow this rule:

- **Log an error/warning/info exactly once on the path from origin to handler.**
- If a child function already logged a message (e.g. `return t.Log.Errorf(...)`),
  the caller must **not** log the same thing again. It may wrap and propagate
  the error, or convert it into a user-facing action, but should not repeat the
  log line.
- Prefer logging close to the source of the event. If the child is generic and
  the caller owns the context, the caller may log instead; in that case the
  child should return a plain `fmt.Errorf` without logging.
- When returning a sentinel error that the caller is expected to handle silently
  (e.g. `ErrNoRelease`), the child may log an info message. The caller must
  recognize the sentinel and skip its own logging.

Example of a duplicate to avoid:

```go
// child
func (b *Backend) Do() error {
    return b.Log.Errorf("thing failed: %v", err) // logs here
}

// parent
func (m *Manager) Run() error {
    if err := b.Do(); err != nil {
        return m.Log.Errorf("run failed: %v", err) // duplicate log
    }
}
```

Fix by returning the error without logging in the parent:

```go
func (m *Manager) Run() error {
    if err := b.Do(); err != nil {
        return err
    }
    return nil
}
```
