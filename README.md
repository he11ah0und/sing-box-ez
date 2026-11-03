# sing-box-ez

A desktop GUI for the [sing-box](https://github.com/SagerNet/sing-box) proxy core,
built with Wails v3 (Go) and a Svelte frontend.

> **Status:** early development. The **GUI application is the primary deliverable**.
> A CLI-only build exists but is **not** published in any package yet.

## Features

- **Profiles / subscriptions** — remote configs with scheduled updates, local
  configs, per-config update intervals, and validation reports.
- **Client style check** — detects whether a config looks like a client or a
  server config (based on its inbounds) and offers to run it anyway or convert
  the inbounds to a client layout.
- **Core management** — downloads and updates the sing-box core for you, with a
  stable / beta release channel and version picker.
- **Dashboard** — live traffic rates, history graphs, per-connection and
  per-group traffic history.
- **Connections** — searchable/filterable connection list (port, route, network,
  protocol, sniffed protocol), per-target grouping and a per-connection activity
  timeline.
- **Groups & outbounds** — outbound selection per proxy group, with the selected
  outbound persisted per profile.
- **Inbounds override & proxy toggle** — adjust the generated inbounds and switch
  the local HTTP/SOCKS proxy on or off.
- **Privilege handling** — detects configs that need elevated privileges
  (tun / transparent proxy) and guides you through setcap/admin setup.
- **Application self-update** — stable (`main`) and `testing` release channels,
  or fully delegated to your package manager on AUR builds.
- **Internationalization** — English, Russian and Chinese UI, themes, system tray.

The sing-box core itself is downloaded as a separate binary at runtime; this
project is only a wrapper around it.

## Requirements

- **Linux:** `gtk4` and `webkitgtk-6.0` (for example `gtk4` and `webkitgtk-6.0`
  on Arch, or `libgtk-4-dev` and `libwebkitgtk-6.0-dev` on Debian/Ubuntu).
- **Windows:** no extra runtime requirements.
- **sing-box core:** downloaded automatically by the application.

## Installation

### Arch Linux (AUR)

The package is called `sing-box-ez-gui-bin` and installs the prebuilt GUI binary:

```sh
yay -S sing-box-ez-gui-bin
# or, with paru:
paru -S sing-box-ez-gui-bin
```

Or build it directly from the AUR repository:

```sh
git clone https://aur.archlinux.org/sing-box-ez-gui-bin.git
cd sing-box-ez-gui-bin
makepkg -si
```

On AUR builds the application **does not self-update** — updates are delivered by
`pacman` when the AUR package is refreshed.

### Windows (Scoop)

The Scoop manifest lives in this repository under `bucket/`, so the repository
itself works as a Scoop bucket:

```powershell
scoop bucket add sing-box-ez https://github.com/he11ah0und/sing-box-ez
scoop install sing-box-ez-gui
```

This installs the GUI executable and a `sing-box-ez` shortcut.

### Manual download

Prebuilt GUI binaries for Linux and Windows are attached to every
[GitHub release](https://github.com/he11ah0und/sing-box-ez/releases):

- `sing-box-ez-amd64-linux-gcc-gui`
- `sing-box-ez-amd64-windows-gcc-gui.exe`

Download the file for your platform and run it directly.

## Building from source

Requirements: Go, Node.js + npm, and a C toolchain for CGO. The Wails v3 CLI is
installed automatically by the Makefile at the version pinned in `go.mod`.

```sh
git clone https://github.com/he11ah0und/sing-box-ez.git
cd sing-box-ez

# The frontend imports @he11ah0und/localengine-web through a relative file:
# path, so the sibling checkout is required next to this repository.
git clone https://github.com/he11ah0und/localengine.git ../localengine

make setup        # Debian/Ubuntu: gtk4/webkitgtk build dependencies
# make setup-arch # Arch Linux equivalents and the Go analysis tools

make build        # GUI binary -> build/sing-box-ez-amd64-<os>-gcc-gui
```

`make build` runs the Wails binding generator, ESLint and `svelte-check` before
compiling, so all of them must pass. A CLI-only build is available as
`make build-nogui` (no Wails or frontend toolchain involved), and `make dev`
starts Wails in development mode with hot reload.

For local development against the sibling Go modules, the repository uses a
git-ignored `go.work` instead of `replace` directives:

```sh
go work init . ../localengine ../logger ../projectspec
```

Remove `go.work` to build strictly against the published module versions (this
is what CI does).

## Configuration

Application data lives in `~/.sing-box-ez`:

- `config.yaml` — application settings (config schema is declared in
  `internal/app/project.yaml`).
- `profiles.yaml` — your profiles, including `fallback_type` chosen during the
  client style check.
- `configs/` — cached profile payloads.

To undo a previous style-check choice, use **Reset style choice** in the
profile's action menu; it clears `fallback_type` so the check runs again on the
next start.

## Releases and updates

- **`main`** produces stable releases named `vX.Y.Z`; **`testing`** produces
  prereleases tagged `vX.Y.Z-testing.<short-hash>`.
- The updater discovers releases through the GitHub **Atom feed** and downloads
  assets directly from `github.com`, avoiding the REST API rate limit.
- AUR packages are published to `sing-box-ez-gui-bin` and the Scoop manifest is
  refreshed automatically by the publish workflow on each stable release.

## License

[GPLv3](LICENSE).
