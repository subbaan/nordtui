package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const version = "0.1.11"

// ── Types ──────────────────────────────────────────────────────────────────────

type targetType string

const (
	tQuick   targetType = "quick"
	tCountry targetType = "country"
	tCity    targetType = "city"
	tServer  targetType = "server"
	tGroup   targetType = "group"
)

type Target struct {
	Type    targetType `json:"type"`
	Label   string     `json:"label"`
	Country string     `json:"country,omitempty"`
	City    string     `json:"city,omitempty"`
	Server  string     `json:"server,omitempty"`
	Group   string     `json:"group,omitempty"`
}

func (t Target) cliArgs() []string {
	switch t.Type {
	case tCountry:
		return []string{"connect", t.Country}
	case tCity:
		return []string{"connect", t.Country, t.City}
	case tServer:
		return []string{"connect", t.Server}
	case tGroup:
		if t.Country != "" {
			return []string{"connect", "--group", t.Group, t.Country}
		}
		return []string{"connect", t.Group}
	default:
		return []string{"connect"}
	}
}

type RecentItem struct {
	Target   Target
	LastUsed string
}

type Config struct {
	Version    string   `json:"version"`
	MaxRecents int      `json:"max_recents"`
	Favourites []Target `json:"favourites"`
}

type Cache struct {
	Version   string              `json:"version"`
	UpdatedAt string              `json:"updated_at"`
	Countries []string            `json:"countries"`
	Cities    map[string][]string `json:"cities"`
}

// savedEntry is one row in the right pane — either a section header or a target.
type savedEntry struct {
	target  Target
	starred bool
	header  bool
	text    string // used when header == true
}

type focusArea int

const (
	focusFilter focusArea = iota
	focusLocs
	focusSaved
)

// ── Messages ───────────────────────────────────────────────────────────────────

type msgCheck struct{ cliOK, loggedIn bool }
type msgStatus string
type msgConnectResult string
type msgDisconnectResult string
type msgCacheLoaded struct {
	countries []string
	cities    map[string][]string
}
type msgErr string

// ── Styles ─────────────────────────────────────────────────────────────────────

var (
	styleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62"))

	styleActiveBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("86"))

	styleCursor = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)

	styleDimCursor = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	styleSection = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Bold(true)

	styleStar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220"))

	styleStatus = lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("250")).
			Padding(0, 1)

	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86"))

	styleHelp = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)

// ── Specialty groups ───────────────────────────────────────────────────────────

var specialtyGroups = []Target{
	{Type: tGroup, Label: "Double VPN",     Group: "Double_VPN"},
	{Type: tGroup, Label: "Onion over VPN", Group: "Onion_Over_VPN"},
	{Type: tGroup, Label: "P2P",            Group: "P2P"},
	{Type: tGroup, Label: "Dedicated IP",   Group: "Dedicated_IP"},
}

// groupCountries lists the countries available for each specialty group.
// Groups not in this map fall back to the full cached country list.
var groupCountries = map[string][]string{
	"Double_VPN": {
		"Canada", "Denmark", "France", "Hong_Kong", "Netherlands",
		"Sweden", "Switzerland", "Taiwan", "United_Kingdom", "United_States",
	},
	"Onion_Over_VPN": {
		"Netherlands", "Sweden",
	},
}

// ── Aliases ────────────────────────────────────────────────────────────────────

var aliases = map[string]string{
	"uk": "united kingdom", "gb": "united kingdom",
	"nl": "netherlands",    "de": "germany",
	"ch": "switzerland",    "us": "united states",
	"fr": "france",         "se": "sweden",
	"no": "norway",         "dk": "denmark",
	"jp": "japan",          "au": "australia",
	"ca": "canada",         "sg": "singapore",
}

// ── XDG paths ──────────────────────────────────────────────────────────────────

func xdgConfig() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "nordtui")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "nordtui")
}

func xdgState() string {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "nordtui")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "nordtui")
}

// ── Storage ────────────────────────────────────────────────────────────────────

func loadConfig() Config {
	os.MkdirAll(xdgConfig(), 0o755)
	data, err := os.ReadFile(filepath.Join(xdgConfig(), "config.json"))
	if err != nil {
		return Config{Version: version, MaxRecents: 20}
	}
	var cfg Config
	if json.Unmarshal(data, &cfg) != nil || cfg.MaxRecents == 0 {
		cfg.MaxRecents = 20
	}
	return cfg
}

func saveConfig(cfg Config) {
	os.MkdirAll(xdgConfig(), 0o755)
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(filepath.Join(xdgConfig(), "config.json"), data, 0o644)
}

func loadRecents() []RecentItem {
	data, err := os.ReadFile(filepath.Join(xdgState(), "recent.json"))
	if err != nil {
		return nil
	}
	var wrapper struct {
		Items []struct {
			Type     targetType `json:"type"`
			Label    string     `json:"label"`
			Country  string     `json:"country"`
			City     string     `json:"city"`
			Server   string     `json:"server"`
			Group    string     `json:"group"`
			LastUsed string     `json:"last_used"`
		} `json:"items"`
	}
	if json.Unmarshal(data, &wrapper) != nil {
		return nil
	}
	items := make([]RecentItem, len(wrapper.Items))
	for i, w := range wrapper.Items {
		items[i] = RecentItem{
			Target:   Target{Type: w.Type, Label: w.Label, Country: w.Country, City: w.City, Server: w.Server, Group: w.Group},
			LastUsed: w.LastUsed,
		}
	}
	return items
}

func saveRecents(items []RecentItem) {
	os.MkdirAll(xdgState(), 0o755)
	type flatItem struct {
		Type     targetType `json:"type"`
		Label    string     `json:"label"`
		Country  string     `json:"country,omitempty"`
		City     string     `json:"city,omitempty"`
		Server   string     `json:"server,omitempty"`
		Group    string     `json:"group,omitempty"`
		LastUsed string     `json:"last_used"`
	}
	wrapper := struct {
		Version string     `json:"version"`
		Items   []flatItem `json:"items"`
	}{Version: version}
	for _, r := range items {
		wrapper.Items = append(wrapper.Items, flatItem{
			Type: r.Target.Type, Label: r.Target.Label,
			Country: r.Target.Country, City: r.Target.City,
			Server: r.Target.Server, Group: r.Target.Group, LastUsed: r.LastUsed,
		})
	}
	data, _ := json.MarshalIndent(wrapper, "", "  ")
	os.WriteFile(filepath.Join(xdgState(), "recent.json"), data, 0o644)
}

func loadCache() Cache {
	data, err := os.ReadFile(filepath.Join(xdgState(), "cache.json"))
	if err != nil {
		return Cache{}
	}
	var c Cache
	json.Unmarshal(data, &c)
	return c
}

func saveCache(countries []string, cities map[string][]string) {
	os.MkdirAll(xdgState(), 0o755)
	c := Cache{
		Version:   version,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Countries: countries,
		Cities:    cities,
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	os.WriteFile(filepath.Join(xdgState(), "cache.json"), data, 0o644)
}

func cacheIsStale(c Cache) bool {
	if c.UpdatedAt == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, c.UpdatedAt)
	if err != nil {
		return true
	}
	return time.Since(t) > 24*time.Hour
}

// ── NordVPN commands (run as tea.Cmd background tasks) ─────────────────────────

func cmdCheck() tea.Cmd {
	return func() tea.Msg {
		if _, err := exec.LookPath("nordvpn"); err != nil {
			return msgCheck{false, false}
		}
		out, _ := exec.Command("nordvpn", "status").CombinedOutput()
		loggedIn := !strings.Contains(strings.ToLower(string(out)), "not logged in")
		return msgCheck{true, loggedIn}
	}
}

func cmdStatus() tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("nordvpn", "status").Output()
		if err != nil {
			return msgStatus("Status unavailable")
		}
		return msgStatus(parseStatusOutput(string(out)))
	}
}

func parseStatusOutput(out string) string {
	fields := make(map[string]string)
	for _, line := range strings.Split(out, "\n") {
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:idx]))
		val := strings.TrimSpace(line[idx+1:])
		if _, exists := fields[key]; !exists {
			fields[key] = val
		}
	}
	switch strings.ToLower(fields["status"]) {
	case "connected":
		parts := []string{"Connected"}
		if country := fields["country"]; country != "" {
			loc := country
			if city := fields["city"]; city != "" {
				loc = country + " / " + city
			}
			parts = append(parts, loc)
		}
		if host := fields["hostname"]; host != "" {
			parts = append(parts, "("+host+")")
		}
		return strings.Join(parts, " — ")
	case "disconnected":
		return "Disconnected"
	case "":
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) > 0 {
			return lines[0]
		}
	}
	return "Status: " + fields["status"]
}

func cmdConnect(t Target) tea.Cmd {
	return func() tea.Msg {
		args := t.cliArgs()
		out, _ := exec.Command("nordvpn", args...).CombinedOutput()
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		return msgConnectResult(lines[0])
	}
}

func cmdDisconnect() tea.Cmd {
	return func() tea.Msg {
		out, _ := exec.Command("nordvpn", "disconnect").CombinedOutput()
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		return msgDisconnectResult(lines[0])
	}
}

func cmdFetchCache() tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("nordvpn", "countries").Output()
		if err != nil || len(out) == 0 {
			return msgErr("Failed to fetch countries — check nordvpn CLI")
		}
		countries := parseNvpnList(string(out))
		if len(countries) == 0 {
			return msgErr("No countries returned")
		}
		cities := make(map[string][]string)
		for _, country := range countries {
			co, cerr := exec.Command("nordvpn", "cities", country).Output()
			if cerr == nil && len(co) > 0 {
				if cs := parseNvpnList(string(co)); len(cs) > 0 {
					cities[country] = cs
				}
			}
		}
		saveCache(countries, cities)
		return msgCacheLoaded{countries: countries, cities: cities}
	}
}

func parseNvpnList(out string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, line := range strings.Split(out, "\n") {
		for _, part := range strings.Split(line, ",") {
			part = strings.TrimSpace(part)
			if len(part) >= 2 && part[0] >= 'A' && part[0] <= 'Z' && !seen[part] {
				seen[part] = true
				result = append(result, part)
			}
		}
	}
	sort.Strings(result)
	return result
}

// ── Location list builder ──────────────────────────────────────────────────────

func buildGroupCountryTargets(g Target, fallback []string) []Target {
	src := fallback
	if known, ok := groupCountries[g.Group]; ok {
		src = known
	}
	targets := make([]Target, 0, len(src))
	for _, country := range src {
		targets = append(targets, Target{
			Type:    tGroup,
			Label:   g.Label + " / " + strings.ReplaceAll(country, "_", " "),
			Group:   g.Group,
			Country: country,
		})
	}
	return targets
}

func buildLocations(cache Cache, showGroups bool) []Target {
	var targets []Target
	if showGroups {
		targets = append(targets, specialtyGroups...)
	}
	targets = append(targets, Target{Type: tQuick, Label: "Quick connect"})
	for _, country := range cache.Countries {
		targets = append(targets, Target{Type: tCountry, Label: country + ", fastest", Country: country})
		for _, city := range cache.Cities[country] {
			targets = append(targets, Target{
				Type: tCity, Label: country + " / " + city, Country: country, City: city,
			})
		}
	}
	return targets
}

func (m model) filterSource() []Target {
	if m.groupDrill != nil {
		return m.drillTargets
	}
	return m.allTargets
}

func filterTargets(targets []Target, query string) []Target {
	if query == "" {
		return targets
	}
	q := strings.ToLower(query)
	if expanded, ok := aliases[q]; ok {
		q = expanded
	}
	tokens := strings.Fields(q)
	var result []Target
	for _, t := range targets {
		label := strings.ToLower(t.Label)
		ok := true
		for _, tok := range tokens {
			if !strings.Contains(label, tok) {
				ok = false
				break
			}
		}
		if ok {
			result = append(result, t)
		}
	}
	return result
}

func buildSaved(cfg Config, recents []RecentItem) []savedEntry {
	var entries []savedEntry
	if len(cfg.Favourites) > 0 {
		entries = append(entries, savedEntry{header: true, text: "── Favourites"})
		for _, t := range cfg.Favourites {
			entries = append(entries, savedEntry{target: t, starred: true})
		}
	}
	if len(recents) > 0 {
		entries = append(entries, savedEntry{header: true, text: "── Recent"})
		for _, r := range recents {
			entries = append(entries, savedEntry{target: r.Target})
		}
	}
	return entries
}

// ── Model ──────────────────────────────────────────────────────────────────────

type model struct {
	filter      textinput.Model
	focus       focusArea
	allTargets  []Target
	filtTargets []Target
	locsIdx     int
	locsOffset  int
	savedEntries []savedEntry
	savedIdx    int
	savedOffset int
	config      Config
	recents     []RecentItem
	cache       Cache
	showGroups   bool
	groupDrill   *Target
	drillTargets []Target
	lastConn     *Target
	status      string
	width       int
	height      int
}

func newModel() model {
	ti := textinput.New()
	ti.Placeholder = "type to filter…"
	ti.Prompt = "Filter: "
	ti.CharLimit = 100
	ti.Width = 28 // updated on first tea.WindowSizeMsg

	cfg := loadConfig()
	recents := loadRecents()
	cache := loadCache()
	all := buildLocations(cache, true)
	saved := buildSaved(cfg, recents)

	firstIdx := 0
	for i, e := range saved {
		if !e.header {
			firstIdx = i
			break
		}
	}

	return model{
		filter:       ti,
		focus:        focusSaved,
		allTargets:   all,
		filtTargets:  all,
		savedEntries: saved,
		savedIdx:     firstIdx,
		config:       cfg,
		recents:      recents,
		cache:        cache,
		showGroups:   true,
		status:       "Initialising…",
		width:        80,
		height:       24,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, cmdCheck())
}

// ── Update ─────────────────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		half := msg.Width / 2
		m.filter.Width = (msg.Width - half) - 12
		return m, nil

	case msgCheck:
		if !msg.cliOK {
			m.status = "nordvpn CLI not found — install NordVPN first."
			return m, nil
		}
		if !msg.loggedIn {
			m.status = "Not logged in — run: nordvpn login"
			return m, nil
		}
		if cacheIsStale(m.cache) {
			m.status = "First run — fetching locations… (this takes a minute)"
			return m, tea.Batch(cmdStatus(), cmdFetchCache())
		}
		return m, cmdStatus()

	case msgStatus:
		m.status = string(msg)
		return m, nil

	case msgConnectResult:
		m.status = string(msg)
		return m, cmdStatus()

	case msgDisconnectResult:
		m.status = string(msg)
		return m, cmdStatus()

	case msgCacheLoaded:
		m.cache = Cache{Countries: msg.countries, Cities: msg.cities}
		m.groupDrill = nil
		m.drillTargets = nil
		m.allTargets = buildLocations(m.cache, m.showGroups)
		m.filtTargets = filterTargets(m.filterSource(), m.filter.Value())
		m.locsIdx, m.locsOffset = 0, 0
		m.status = fmt.Sprintf("Ready — %d countries loaded.", len(msg.countries))
		return m, nil

	case msgErr:
		m.status = string(msg)
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Delegate remaining messages to textinput when filter is focused.
	if m.focus == focusFilter {
		var cmd tea.Cmd
		prev := m.filter.Value()
		m.filter, cmd = m.filter.Update(msg)
		if m.filter.Value() != prev {
			m.filtTargets = filterTargets(m.filterSource(), m.filter.Value())
			m.locsIdx, m.locsOffset = 0, 0
		}
		return m, cmd
	}

	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// ── Always-on keys ────────────────────────────────────────────────────────
	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		if m.groupDrill != nil {
			m.groupDrill = nil
			m.drillTargets = nil
			m.filter.SetValue("")
			m.filtTargets = m.allTargets
			m.locsIdx, m.locsOffset = 0, 0
			m.status = "Ready"
			return m, nil
		}
		if m.filter.Value() != "" {
			m.filter.SetValue("")
			m.filtTargets = m.allTargets
			m.locsIdx, m.locsOffset = 0, 0
		}
		m.focus = focusFilter
		m.filter.Focus()
		return m, nil
	case "tab":
		m = m.cycleFocus()
		return m, nil
	}

	// ── Filter input ──────────────────────────────────────────────────────────
	if m.focus == focusFilter {
		// Let q / action keys pass through to textinput when typing.
		var cmd tea.Cmd
		prev := m.filter.Value()
		m.filter, cmd = m.filter.Update(msg)
		if m.filter.Value() != prev {
			m.filtTargets = filterTargets(m.filterSource(), m.filter.Value())
			m.locsIdx, m.locsOffset = 0, 0
		}
		return m, cmd
	}

	// ── Action keys (pane focus) ──────────────────────────────────────────────
	switch key {
	case "enter":
		return m.doConnect()
	case "f":
		return m.toggleFav(), nil
	case "d":
		m.status = "Disconnecting…"
		return m, cmdDisconnect()
	case "r":
		if m.lastConn != nil {
			t := *m.lastConn
			m.status = fmt.Sprintf("Connecting to %s…", t.Label)
			return m, cmdConnect(t)
		}
		m.status = "No previous connection."
		return m, nil
	case "s":
		return m, cmdStatus()
	case "g":
		m.showGroups = !m.showGroups
		m.groupDrill = nil
		m.drillTargets = nil
		m.allTargets = buildLocations(m.cache, m.showGroups)
		m.filtTargets = filterTargets(m.filterSource(), m.filter.Value())
		m.locsIdx, m.locsOffset = 0, 0
		if m.showGroups {
			m.status = "Specialty servers shown"
		} else {
			m.status = "Specialty servers hidden"
		}
		return m, nil
	case "ctrl+r":
		m.status = "Refreshing location cache…"
		return m, cmdFetchCache()
	case "ctrl+d":
		return m.deleteRecent(), nil
	case "?":
		m.status = "Enter:connect  Tab:pane  f:★  d:disc  r:reconnect  g:groups  Ctrl+R:refresh  Ctrl+D:del  q:quit"
		return m, nil
	case "q":
		return m, tea.Quit
	}

	// ── Pane navigation ───────────────────────────────────────────────────────
	listH := m.listHeight()
	locsItemH := listH - 1 // one row of locs pane used by embedded filter

	switch m.focus {
	case focusLocs:
		switch key {
		case "up", "k":
			if m.locsIdx > 0 {
				m.locsIdx--
				if m.locsIdx < m.locsOffset {
					m.locsOffset = m.locsIdx
				}
			}
		case "down", "j":
			if m.locsIdx < len(m.filtTargets)-1 {
				m.locsIdx++
				if m.locsIdx >= m.locsOffset+locsItemH {
					m.locsOffset = m.locsIdx - locsItemH + 1
				}
			}
		default:
			// Printable non-action char → redirect to filter
			if isTypable(key) {
				m.focus = focusFilter
				m.filter.Focus()
				m.filter.SetValue(m.filter.Value() + key)
				m.filtTargets = filterTargets(m.filterSource(), m.filter.Value())
				m.locsIdx, m.locsOffset = 0, 0
			}
		}

	case focusSaved:
		switch key {
		case "up", "k":
			m.savedIdx = m.prevSaved(m.savedIdx)
			if m.savedIdx < m.savedOffset {
				m.savedOffset = m.savedIdx
			}
		case "down", "j":
			m.savedIdx = m.nextSaved(m.savedIdx)
			if m.savedIdx >= m.savedOffset+listH {
				m.savedOffset = m.savedIdx - listH + 1
			}
		default:
			if isTypable(key) {
				m.focus = focusFilter
				m.filter.Focus()
				m.filter.SetValue(m.filter.Value() + key)
				m.filtTargets = filterTargets(m.filterSource(), m.filter.Value())
				m.locsIdx, m.locsOffset = 0, 0
			}
		}
	}

	return m, nil
}

func isTypable(key string) bool {
	if len(key) != 1 {
		return false
	}
	c := key[0]
	// Exclude keys bound to actions
	return c >= 0x20 && c < 0x7f && !strings.ContainsRune("fdrascgq?", rune(c))
}

func (m model) cycleFocus() model {
	switch m.focus {
	case focusSaved:
		m.focus = focusFilter
		m.filter.Focus()
	case focusFilter:
		m.focus = focusLocs
		m.filter.Blur()
	case focusLocs:
		m.focus = focusSaved
	}
	return m
}

// ── Actions ────────────────────────────────────────────────────────────────────

func (m model) selectedTarget() *Target {
	if m.focus == focusSaved {
		if m.savedIdx >= 0 && m.savedIdx < len(m.savedEntries) {
			e := m.savedEntries[m.savedIdx]
			if !e.header {
				t := e.target
				return &t
			}
		}
		return nil
	}
	if m.locsIdx >= 0 && m.locsIdx < len(m.filtTargets) {
		t := m.filtTargets[m.locsIdx]
		return &t
	}
	return nil
}

func (m model) doConnect() (model, tea.Cmd) {
	t := m.selectedTarget()
	if t == nil {
		return m, nil
	}
	// Bare group entry → drill down to country picker instead of connecting.
	if t.Type == tGroup && t.Country == "" {
		m.groupDrill = t
		m.drillTargets = buildGroupCountryTargets(*t, m.cache.Countries)
		m.filter.SetValue("")
		m.filtTargets = m.drillTargets
		m.locsIdx, m.locsOffset = 0, 0
		m.focus = focusLocs
		m.filter.Blur()
		m.status = fmt.Sprintf("%s — select a country  (Esc: back)", t.Label)
		return m, nil
	}
	copied := *t
	m.lastConn = &copied
	m.status = fmt.Sprintf("Connecting to %s…", t.Label)
	m = m.pushRecent(*t)
	return m, cmdConnect(*t)
}

func (m model) pushRecent(t Target) model {
	newList := m.recents[:0]
	for _, r := range m.recents {
		if r.Target.Label != t.Label {
			newList = append(newList, r)
		}
	}
	m.recents = append([]RecentItem{{Target: t, LastUsed: time.Now().UTC().Format(time.RFC3339)}}, newList...)
	if len(m.recents) > m.config.MaxRecents {
		m.recents = m.recents[:m.config.MaxRecents]
	}
	saveRecents(m.recents)
	m.savedEntries = buildSaved(m.config, m.recents)
	return m
}

func (m model) toggleFav() model {
	t := m.selectedTarget()
	if t == nil {
		return m
	}

	favSet := make(map[string]bool, len(m.config.Favourites))
	for _, f := range m.config.Favourites {
		favSet[f.Label] = true
	}

	adding := !favSet[t.Label]
	if adding {
		m.config.Favourites = append(m.config.Favourites, *t)
	} else {
		newFavs := m.config.Favourites[:0]
		for _, f := range m.config.Favourites {
			if f.Label != t.Label {
				newFavs = append(newFavs, f)
			}
		}
		m.config.Favourites = newFavs
	}
	saveConfig(m.config)
	m.savedEntries = buildSaved(m.config, m.recents)

	if adding {
		for i, e := range m.savedEntries {
			if !e.header && e.starred && e.target.Label == t.Label {
				m.savedIdx = i
				break
			}
		}
	} else {
		// Stay at the same index; walk backwards past any header that is now
		// at the cursor position after the entry was removed.
		if m.savedIdx >= len(m.savedEntries) {
			m.savedIdx = len(m.savedEntries) - 1
		}
		for m.savedIdx >= 0 && m.savedEntries[m.savedIdx].header {
			m.savedIdx--
		}
		if m.savedIdx < 0 {
			m.savedIdx = m.firstSaved()
		}
	}

	// Scroll just enough to keep the cursor visible; never reset to top.
	listH := m.listHeight()
	if m.savedIdx < m.savedOffset {
		m.savedOffset = m.savedIdx
	} else if m.savedIdx >= m.savedOffset+listH {
		m.savedOffset = m.savedIdx - listH + 1
	}
	return m
}

func (m model) deleteRecent() model {
	if m.focus != focusSaved || m.savedIdx < 0 || m.savedIdx >= len(m.savedEntries) {
		return m
	}
	e := m.savedEntries[m.savedIdx]
	if e.header || e.starred {
		return m
	}
	newRecents := m.recents[:0]
	for _, r := range m.recents {
		if r.Target.Label != e.target.Label {
			newRecents = append(newRecents, r)
		}
	}
	m.recents = newRecents
	saveRecents(m.recents)
	m.savedEntries = buildSaved(m.config, m.recents)
	// Re-position cursor
	if m.savedIdx >= len(m.savedEntries) {
		m.savedIdx = len(m.savedEntries) - 1
	}
	for m.savedIdx >= 0 && m.savedEntries[m.savedIdx].header {
		m.savedIdx--
	}
	if m.savedIdx < 0 {
		m.savedIdx = m.firstSaved()
	}
	return m
}

// ── Saved pane navigation helpers ─────────────────────────────────────────────

func (m model) firstSaved() int {
	for i, e := range m.savedEntries {
		if !e.header {
			return i
		}
	}
	return 0
}

func (m model) nextSaved(from int) int {
	for i := from + 1; i < len(m.savedEntries); i++ {
		if !m.savedEntries[i].header {
			return i
		}
	}
	return from
}

func (m model) prevSaved(from int) int {
	for i := from - 1; i >= 0; i-- {
		if !m.savedEntries[i].header {
			return i
		}
	}
	return from
}

// ── View ───────────────────────────────────────────────────────────────────────

// listHeight is the number of rows available inside each pane border.
func (m model) listHeight() int {
	// title(1) + pane borders(2) + status(1) + help(1) = 5
	h := m.height - 5
	if h < 3 {
		return 3
	}
	return h
}

func (m model) View() string {
	listH := m.listHeight()
	half := m.width / 2

	// Title
	title := styleTitle.Render(fmt.Sprintf(" NordVPN TUI  v%s", version))

	// Saved pane on left, locations pane (with embedded filter) on right
	savedPane := m.renderSaved(half, listH)
	locsPane := m.renderLocs(m.width-half, listH)
	panes := lipgloss.JoinHorizontal(lipgloss.Top, savedPane, locsPane)

	// Status bar
	status := styleStatus.Width(m.width).Render(m.status)

	// Help line
	help := styleHelp.Render(
		" Enter:connect  Tab:switch  f:★  d:disc  r:reconnect  g:groups  Ctrl+R:refresh  q:quit",
	)

	return lipgloss.JoinVertical(lipgloss.Left, title, panes, status, help)
}

func (m model) renderLocs(outerW, listH int) string {
	innerW := outerW - 2
	if innerW < 4 {
		innerW = 4
	}

	// Filter input occupies the first row; list gets the remaining rows.
	filterLine := m.filter.View()
	itemH := listH - 1
	if itemH < 1 {
		itemH = 1
	}

	var sb strings.Builder
	end := m.locsOffset + itemH
	if end > len(m.filtTargets) {
		end = len(m.filtTargets)
	}

	for i := m.locsOffset; i < end; i++ {
		label := m.filtTargets[i].Label
		line := truncate("  "+label, innerW)
		if i == m.locsIdx {
			line = truncate("> "+label, innerW)
			if m.focus == focusLocs {
				line = styleCursor.Render(line)
			} else {
				line = styleDimCursor.Render(line)
			}
		}
		sb.WriteString(line + "\n")
	}

	// Pad remaining rows
	for i := end - m.locsOffset; i < itemH; i++ {
		sb.WriteString("\n")
	}

	content := filterLine + "\n" + sb.String()

	s := styleBorder
	if m.focus == focusLocs || m.focus == focusFilter {
		s = styleActiveBorder
	}
	return s.Width(innerW).Height(listH).Render(content)
}

func (m model) renderSaved(outerW, listH int) string {
	innerW := outerW - 2
	if innerW < 4 {
		innerW = 4
	}

	var sb strings.Builder
	end := m.savedOffset + listH
	if end > len(m.savedEntries) {
		end = len(m.savedEntries)
	}

	for i := m.savedOffset; i < end; i++ {
		e := m.savedEntries[i]
		var line string
		switch {
		case e.header:
			line = styleSection.Render(truncate(e.text, innerW))
		case i == m.savedIdx:
			prefix := "> "
			if e.starred {
				prefix = ">★ "
			}
			if m.focus == focusSaved {
				line = styleCursor.Render(truncate(prefix+e.target.Label, innerW))
			} else {
				line = styleDimCursor.Render(truncate(prefix+e.target.Label, innerW))
			}
		case e.starred:
			star := styleStar.Render("★")
			rest := truncate(e.target.Label, innerW-4)
			line = "  " + star + " " + rest
		default:
			line = truncate("    "+e.target.Label, innerW)
		}
		sb.WriteString(line + "\n")
	}

	for i := end - m.savedOffset; i < listH; i++ {
		sb.WriteString("\n")
	}

	s := styleBorder
	if m.focus == focusSaved {
		s = styleActiveBorder
	}
	return s.Width(innerW).Height(listH).Render(sb.String())
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max < 1 {
		return ""
	}
	return s[:max-1] + "…"
}

// ── Entry point ────────────────────────────────────────────────────────────────

func main() {
	for _, arg := range os.Args[1:] {
		switch arg {
		case "-v", "--version":
			fmt.Println(version)
			return
		case "-h", "--help":
			fmt.Printf("nordtui v%s — NordVPN TUI location selector\n\n", version)
			fmt.Println("Usage: nordtui [options]")
			fmt.Println()
			fmt.Println("Options:")
			fmt.Println("  -h, --help     Show this help and exit")
			fmt.Println("  -v, --version  Print version and exit")
			fmt.Println()
			fmt.Println("Navigation:")
			fmt.Println("  Tab            Cycle focus: Recent/Saved → Filter → Locations")
			fmt.Println("  ↑/↓  k/j       Move selection")
			fmt.Println("  Enter          Connect to selected target")
			fmt.Println("  /              Jump to filter input")
			fmt.Println("  g              Toggle specialty server groups")
			fmt.Println("  q  Ctrl+C      Quit")
			fmt.Println()
			fmt.Printf("Config: %s/config.json\n", xdgConfig())
			fmt.Printf("State:  %s/recent.json\n", xdgState())
			return
		}
	}
	p := tea.NewProgram(newModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
