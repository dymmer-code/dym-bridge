# GoGPU UI preview

Experimental branch: `codex/gogpu-ui-prototype`. Tracking: Dymmer Backlog #423.

This is a **read-only UX prototype**, not a replacement for the production client.
It uses GoGPU UI v0.1.55, Fluent widgets and Metal on macOS. No WebView and no
CGo are used by the preview executable. The existing Fyne executable is unchanged.

## Run

```sh
make preview
# Or choose a local directory:
./bin/dym-bridge-gpu -dir /path/to/folder
# Independent macOS app, with its own application identifier:
make preview-bundle
open 'bin/Dym Bridge Preview.app'
```

The left pane lists real local files using the existing local/SFTP item model.
The right pane is explicitly labeled demonstration data. Select a folder and
press **Abrir carpeta**; **Subir** goes to its parent. Search filters each pane.
The sidebar provides local shortcuts, a light/dark theme switch and a synthetic
10,000-file workload. Lists are virtualized with fixed row heights.

No credentials are loaded, no SSH connection is made, and no file is changed.
Transfers, login, tray integration, native file drops, multiselection and real
remote browsing are not wired up. The transfer area explains that limitation.

## Verification

```sh
make preview-test
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build ./cmd/dym-bridge-gpu
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./cmd/dym-bridge-gpu
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/dym-bridge-gpu
```

On macOS/Apple M1, the actual window was inspected in dark mode. Loading 10,000
synthetic rows and filtering to `asset-09999` produced exactly one visible result.
A point-in-time `ps` sample after those interactions showed 79,424 KiB RSS
(about 78 MiB) and 0.0% CPU for the preview app. This is not a benchmark or total
GPU/process footprint, and is not directly comparable with the production app.

The macOS accessibility tree exposed the window but not its internal controls.
Accessibility is therefore a migration blocker until resolved/verified. Other
acceptance work: responsive resizing, keyboard navigation, long filenames,
large real directories, drag/drop, tray lifetime, and OS-native dialogs.

Directory reads currently run synchronously: slow local/network-mounted folders
could block the prototype. Production integration must move I/O off the UI event
handler and apply results on the UI thread, with stale-request protection.

Do not migrate the SSH/security/provisioning core to justify a UI change. Once
the visual direction is accepted, introduce an application service shared by
both frontends and connect progress, errors and cancellation through it.

## Compact revision (#424)

The dashboard composition was replaced after user review: no sidebar, cards,
large heading or permanent details panel. Both file panes fill the window;
28px alternating rows show name, size and modification time. Per-pane compact
bars contain paths, parent/open/refresh actions, shortcuts and filtering.
Selection details appear in the status bar; transfers expand only on request.
A constrained text renderer clips/ellipsizes long names so columns do not overlap.
The existing read-only/demo scope and integration limitations still apply.
