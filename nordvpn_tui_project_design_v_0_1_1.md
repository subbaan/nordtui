# NordVPN Location TUI Project Design

Version: 0.1.10
Date: 2026-05-11
Status: Active

## 1. Overview

This project is a terminal user interface for connecting to NordVPN locations quickly, without using the NordVPN GUI's limited recent-node list or repeatedly scrolling and searching through a large list.

The core idea is to treat VPN choices as **location targets**, not exact server nodes. In normal use, the user usually wants the fastest available server in a country or city, not a specific server number. The TUI therefore prioritises country and city targets such as:

```text
United Kingdom, fastest
United Kingdom / Manchester
United Kingdom / London
Netherlands, fastest
Germany / Berlin
Switzerland, fastest
```

Exact server IDs can be kept as an advanced fallback for troubleshooting, repeatable tests, or cases where a particular IP behaves better with a site.

## 2. Goals

The application should make NordVPN location switching faster and more pleasant from the terminal.

Primary goals:

- Provide instant fuzzy filtering of VPN locations.
- Prioritise country and city level connection targets.
- Maintain a useful history beyond NordVPN GUI's small recent list.
- Allow favourites to be pinned for fast access.
- Keep exact server selection hidden unless explicitly needed.
- Use keyboard-first navigation.
- Work well inside a normal terminal, especially on Linux.

Secondary goals:

- Show current VPN connection status.
- Allow quick disconnect and reconnect.
- Keep configuration simple and text based.
- Avoid depending on internal or unofficial NordVPN APIs where possible.

## 3. Non-goals

The first version does not need to:

- Replace the entire NordVPN GUI.
- Display a live map.
- Show every individual VPN server by default.
- Manage account subscription settings.
- Implement custom VPN protocols itself.
- Bypass the official `nordvpn` command line tool.

The TUI should act as a friendly wrapper around the existing NordVPN Linux CLI.

## 4. Main Interface Model

The interface is a two-pane layout. The **left pane** (Saved/Recent) is focused at startup since the most common workflow is switching between previously-used nodes. The **right pane** (Locations) contains the searchable location list with the filter input embedded at its top.

```text
┌─ NordVPN TUI  v0.1.8 ─────────────────────────────────────────┐
├──────────────────────────────┬────────────────────────────────┤
│ Saved                        │ Filter:                        │
│                              │                                │
│ ── Favourites                │   Double VPN                   │
│ ★ Netherlands, fastest       │   Onion over VPN               │
│ ★ UK / Manchester            │   P2P                          │
│ ★ Switzerland, fastest       │   Dedicated IP                 │
│                              │ > Quick connect                │
│ ── Recent                    │   Albania, fastest             │
│ > Germany / Berlin           │   Australia, fastest           │
│   Sweden, fastest            │   ...                          │
├──────────────────────────────┴────────────────────────────────┤
│ Status: Connected, United Kingdom / Manchester                │
│ Enter connect  Tab switch  f:★  d:disc  g:groups  q quit      │
└───────────────────────────────────────────────────────────────┘
```

### Left pane: Saved

The left pane contains two sections:

- **Favourites** — user-pinned targets, shown with a star prefix.
- **Recent** — automatically recorded targets in most-recent-first order.

This pane is focused at startup. The cursor lands on the first selectable entry (skipping section headers), so the most common action — reconnect to a recent node — requires only Enter.

### Right pane: Locations

The right pane contains all normal connection targets:

- Quick connect.
- Country fastest targets.
- City fastest targets.

The filter input is embedded at the top of this pane. Typing in it narrows the location list immediately. When the right pane is active (either filter or list focused), the pane border highlights to make the active area clear.

## 5. Why Location Targets Are Better Than Exact Nodes

For normal VPN use, exact server selection is usually unnecessary. The useful question is normally:

```text
Where do I want to appear to be connecting from?
```

not:

```text
Which exact numbered server should I use?
```

Using location targets has several advantages:

- NordVPN can choose a good available server at connection time.
- Saved entries do not become stale when a server is overloaded or unavailable.
- The interface is much smaller and easier to search.
- Country and city names are easier to remember than server IDs.
- Recents become meaningful, because they represent locations rather than temporary node numbers.

Exact server selection should be treated as an advanced feature.

## 6. Advanced Exact Server Mode

Exact server mode may be useful in specific cases:

- A particular site works better with one IP.
- A service blocks one NordVPN server but not another.
- The user is troubleshooting speed or routing problems.
- Repeatable testing requires the same endpoint.
- The user has a dedicated or static IP option.

This mode is not yet implemented. It is planned as a future feature.

## 7. Keyboard Controls

```text
Tab        Cycle focus: Saved → Filter → Locations → Saved
↑/↓  k/j   Move selection in focused pane
Enter      Connect to selected target
/          Jump directly to filter input
f          Toggle favourite for selected target
d          Disconnect VPN
g          Toggle specialty server groups on/off
s          Refresh status
r          Reconnect last connection
Esc        Clear filter or return to Saved pane
Ctrl+R     Force refresh country/city list
Ctrl+D     Delete selected recent item
q  Ctrl+C  Quit
```

## 8. CLI Flags

The binary accepts the following flags and exits immediately without launching the TUI:

```text
-v  --version   Print version number and exit
-h  --help      Print usage, navigation summary, and config paths, then exit
```

Example output of `--help`:

```text
nordtui v0.1.3 — NordVPN TUI location selector

Usage: nordtui [options]

Options:
  -h, --help     Show this help and exit
  -v, --version  Print version and exit

Navigation:
  Tab            Cycle focus: Recent/Saved → Filter → Locations
  ↑/↓  k/j       Move selection
  Enter          Connect to selected target
  /              Jump to filter input
  q  Ctrl+C      Quit

Config: ~/.config/nordtui/config.json
State:  ~/.local/state/nordtui/recent.json
```

## 9. Connection Targets

Internally, each selectable item is represented as a structured target.

Target types:

```text
quick
country
city
server
group
```

Example records:

```json
{"type": "quick",   "label": "Quick connect"}
{"type": "country", "label": "Netherlands, fastest",       "country": "Netherlands"}
{"type": "city",    "label": "United Kingdom / Manchester", "country": "United Kingdom", "city": "Manchester"}
{"type": "server",  "label": "United Kingdom #2041",        "country": "United Kingdom", "server": "uk2041"}
{"type": "group",   "label": "Double VPN",                  "group": "Double_VPN"}
{"type": "group",   "label": "Onion over VPN",              "group": "Onion_Over_VPN"}
{"type": "group",   "label": "P2P",                         "group": "P2P"}
{"type": "group",   "label": "Dedicated IP",                "group": "Dedicated_IP"}
```

## 10. Command Mapping

The application uses the official `nordvpn` CLI as the backend.

```text
Quick connect:         nordvpn connect
Country fastest:       nordvpn connect Netherlands
City fastest:          nordvpn connect "United Kingdom" Manchester
Exact server:          nordvpn connect uk2041
Specialty group:       nordvpn connect Double_VPN
Disconnect:            nordvpn disconnect
Status:                nordvpn status
List countries:        nordvpn countries
List cities:           nordvpn cities "United Kingdom"
List groups:           nordvpn groups
```

## 11. Async Subprocess Execution

The `nordvpn connect` command can block for several seconds. All NordVPN CLI calls run asynchronously so the UI remains fully responsive.

In the Go implementation, Bubble Tea commands (`tea.Cmd`) and goroutines are used for non-blocking subprocess execution. The status line shows "Connecting to X..." immediately when a connection is initiated and is updated with the result when the process exits.

Example flow:

```text
User presses Enter on "Netherlands, fastest"
  → status bar: "Connecting to Netherlands, fastest…"
  → nordvpn connect Netherlands  (runs in background)
  → status bar: "You are connected to Netherlands #123" (on exit)
```

## 12. Data Storage

Configuration and state live in standard XDG locations.

```text
~/.config/nordtui/config.json
~/.local/state/nordtui/recent.json
~/.local/state/nordtui/cache.json
```

### config.json

Stores user preferences and favourites.

```json
{
  "version": "0.1.0",
  "max_recents": 20,
  "favourites": [
    {"type": "country", "label": "Netherlands, fastest",       "country": "Netherlands"},
    {"type": "city",    "label": "United Kingdom / Manchester", "country": "United Kingdom", "city": "Manchester"}
  ]
}
```

### recent.json

Stores recent targets in most-recent-first order.

```json
{
  "version": "0.1.0",
  "items": [
    {
      "type": "city",
      "label": "Germany / Berlin",
      "country": "Germany",
      "city": "Berlin",
      "last_used": "2026-05-11T10:45:00+01:00"
    }
  ]
}
```

### cache.json

Stores the last known country and city list so the application can start quickly. The cache is considered stale if its `updated_at` timestamp is older than 24 hours. On startup, a stale cache triggers a silent background refresh. The user can force an immediate refresh with `Ctrl+r`.

```json
{
  "version": "0.1.0",
  "updated_at": "2026-05-11T10:45:00+01:00",
  "countries": ["United Kingdom", "Netherlands", "Germany"],
  "cities": {
    "United Kingdom": ["London", "Manchester"],
    "Germany": ["Berlin", "Munich"]
  }
}
```

## 13. Startup Behaviour

On startup, the application:

1. Loads configuration and saved favourites.
2. Loads recent targets.
3. Loads cached country and city data.
4. Displays the interface immediately using cached data.
5. Focuses the left (Saved) pane with cursor on the first selectable entry.
6. Refreshes NordVPN status in the background.
7. Triggers a background cache refresh if the cache is stale.

## 14. First-run Behaviour

On first run, the app:

- Checks whether the `nordvpn` command exists.
- Checks whether NordVPN appears to be logged in.
- Fetches the country list.
- Fetches city lists where practical.
- Creates config and state directories.
- Starts with an empty favourites list and empty recents list.

## 15. Filtering Behaviour

Filtering is immediate and case-insensitive substring matching.

The following searches should work:

```text
uk          united      manch
nether      nl          berlin
germ ber
```

The app supports aliases for common country abbreviations:

```json
{"uk": "United Kingdom", "gb": "United Kingdom", "nl": "Netherlands",
 "de": "Germany", "ch": "Switzerland", "us": "United States"}
```

### Focus and filter interaction

Tab cycles through three focus areas: Saved pane → Filter input → Locations list → Saved pane.

- Startup focus lands on the Saved pane (most common workflow: reconnect a recent node).
- Press Tab once to reach the filter input; typing narrows the locations list.
- Press Tab again to reach the locations list for keyboard navigation.
- Press Tab again to return to the Saved pane.
- The right pane border highlights whenever either the filter input or the locations list has focus.

## 16. Status Display

The status bar shows current VPN state between the panes and the help line.

Examples:

```text
Status: Connected, United Kingdom / Manchester
Status: Disconnected
Status: Connecting to Netherlands, fastest...
Status: Connection failed
```

## 17. Error Handling

Common failure messages shown in the status bar:

```text
NordVPN CLI not found. Install NordVPN CLI first.
NordVPN is not logged in. Run: nordvpn login
Connection failed. Press s to refresh status, or r to retry.
No cities found for this country.
```

Errors appear in the status line, not as a full crash unless the error is unrecoverable.

## 18. Implementation

The primary implementation is Go, using:

```text
Go 1.21+
Bubble Tea  — TUI event loop and model/update/view pattern
Lipgloss    — layout composition and border styling
JSON files  — config and state storage
```

The Python/Textual prototype (`nordtui.py`) has been retired. The Go binary is the only maintained implementation.

## 19. Project Structure

```text
nordtui/
├── nordtui/
│   ├── main.go       Go source (single file)
│   ├── go.mod
│   ├── go.sum
│   └── nordtui       compiled binary
└── nordvpn_tui_project_design_v_0_1_1.md
```

## 20. Installation

Build from source:

```text
cd nordtui
go build -o nordtui .
cp nordtui ~/.local/bin/nordtui
```

No external dependencies beyond the Go module dependencies declared in `go.mod`.

## 21. Minimum Viable Version

Version 0.1.0 delivered:

- Two-pane TUI.
- Cached country and city list.
- Instant filtering.
- Connect to quick, country, or city target.
- Save favourites.
- Record recent targets.
- Disconnect.
- Show current status.

Version 0.1.2 delivered:

- Panel swap: Saved/Recent on the left (startup focus), Locations + filter on the right.
- Filter embedded inside the right pane rather than as a full-width bar.
- Tab cycle updated: Saved → Filter → Locations → Saved.

Version 0.1.3 delivered:

- CLI flags: `-v`/`--version` and `-h`/`--help`.

Version 0.1.8 delivered:

- Specialty server groups (Double VPN, Onion over VPN, P2P, Dedicated IP) at the top of the locations list.
- `g` keybind to toggle group visibility on/off.
- Groups participate in recents and favourites like any other target.
- `group` added as a target type; `Group` field added to serialised targets.

Version 0.1.10 delivered:

- Per-group country allowlists to prevent connecting to unsupported countries.
- Double VPN drill-down now shows only: Canada, Denmark, France, Hong Kong, Netherlands, Sweden, Switzerland, Taiwan, United Kingdom, United States.
- Onion over VPN shows only: Netherlands, Sweden.
- Groups without a known list (P2P, Dedicated IP) fall back to the full cached country list.

Version 0.1.9 delivered:

- Selecting a specialty group entry drills down to a country picker instead of connecting immediately.
- Country list reuses the cached countries; each entry connects via `nordvpn connect --group <Group> <Country>`.
- Esc exits the country picker and returns to the main locations list.
- Group+country targets participate in recents and favourites.

## 22. Later Features

Possible future features:

- Advanced exact-server browsing.
- Latency display if available.
- Auto-refresh status while connecting.
- Import GUI history if available.
- Custom aliases.
- Per-target notes.
- Tray or desktop launcher integration.
- Optional notification when connected.
- Colour theme settings.
