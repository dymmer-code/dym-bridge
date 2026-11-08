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
The right pane is explicitly labeled demonstration data. Double-click a folder
or press Enter to open it; the parent-folder arrow navigates up. The window header provides a
light/dark theme switch and a synthetic 10,000-file workload. Lists are virtualized
with fixed row heights.

No credentials are loaded, no SSH connection is made, and no file is changed.
Transfers, login, tray integration, native file drops, modifier-key multiselection and real
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

## File interaction (#425)

Folders open with a double left click on the same row (within 500 ms and 4 px),
or Enter after selecting them. A single click selects; files are not executed.
Scrolling or changing the filter resets the pending click pair. Navigation runs
only after the old list finishes dispatching its input event.

The input adapter preserves rich pointer/gesture events and fixes GoGPU UI
v0.1.55's scroll unit loss: pixel deltas remain pixels, line deltas move one 28px
row, and page deltas use a nominal 560px page. It compensates for ScrollView's
40x multiplier before the bridge discards DeltaMode. No system mouse setting is
changed. Re-evaluate the adapter when upgrading the toolkit.

Rows now use 16px vector folder/document icons, independent of font glyphs.

## Per-pane toolbars (#426)

Navigation (parent and refresh) sits beside the path. A separate compact action
bar above each file table groups selection, editing and transfer with vector
icons and separators: Todo, Nueva carpeta, Eliminar, and Subir/Descargar.
Native GoGPU toolbar widgets use a painter matching the current Fluent palette.

Todo toggles selection of all visible results; a single row click returns to
single selection. Filtering clears selection and the footer reports its count.
New-folder, delete and transfer actions remain visibly disabled because this
preview has no write/transfer integration. No extra dependencies or CGo added.

Verified with selection model regression tests, make preview-test, and
make preview-bundle. Inspected the macOS window and selected all four demo rows.

## Two-row pane headers (#427)

Supersedes the toolbar layout in #426: each pane now has one identity/path row
and one 32px icon toolbar. Removed the search field and separate shortcut row.
Commands stay together in the familiar order: parent, refresh, select all, new
folder, delete, upload/download. Read-only/demo status sits at the trailing edge.
Write actions remain disabled. The file table gains the space previously used
by the extra rows. No new dependencies or behavioral changes to navigation.
Verified with make preview-test, preview-bundle (CGo disabled), and inspection
of the rebuilt macOS window.

## Toolbar help (#429)

Every pane toolbar button shows its label in a floating tooltip on hover.
Disabled actions add "No disponible en esta vista previa". The overlay uses
Fluent colors and window-space positioning; it disappears on leave, press,
keyboard input or unmount. It does not enable disabled commands or change layout.
The wrapper records actual painted item bounds rather than duplicating layout.
Tests cover disabled help, stable hover, leaving the button and unmount cleanup.

## Checkbox selection (#430)

Each row now starts with a Fluent checkbox. Clicking its checkbox toggles only
that item; clicking the name moves focus without replacing checked items.
Space toggles the focused row, Enter/double-click on the name opens folders.
Checkbox clicks reset double-click tracking and never navigate. Select-all and
footer counts use the same checked-path set. Navigation clears that directory's
selection; rebuilding the same view preserves checked items and their count.
Regression tests exercise multiple checks, unchecking, independent focus,
Space, double-click navigation, and select-all/clear. macOS inspection confirmed
visible checkboxes and a checked row with the corresponding footer count.
