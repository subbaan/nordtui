package main

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
)

func TestParseVPNStatusConnected(t *testing.T) {
	status := parseVPNStatus(`Status: Connected
Hostname: uk1234.nordvpn.com
Country: United Kingdom
City: Manchester
`)

	if !status.known || !status.connected {
		t.Fatalf("expected a known connected status, got %#v", status)
	}
	if status.country != "United Kingdom" || status.city != "Manchester" {
		t.Fatalf("unexpected location: %#v", status)
	}
	if status.text != "Connected — United Kingdom / Manchester — (uk1234.nordvpn.com)" {
		t.Fatalf("unexpected display text: %q", status.text)
	}
}

func TestCacheNoticeDoesNotHideConnection(t *testing.T) {
	m := model{filter: textinput.New(), showGroups: true}
	m.vpn = vpnStatus{
		text:      "Connected — United Kingdom / Manchester",
		known:     true,
		connected: true,
		country:   "United Kingdom",
		city:      "Manchester",
	}

	updated, _ := m.Update(msgCacheLoaded{countries: []string{"United_Kingdom"}})
	got := updated.(model).renderStatus()
	if !strings.Contains(got, "Connected — United Kingdom / Manchester") {
		t.Fatalf("connection was hidden after cache load: %q", got)
	}
	if !strings.Contains(got, "Ready — 1 countries loaded.") {
		t.Fatalf("cache notice missing from status: %q", got)
	}
}

func TestTargetIsConnectedNormalizesCountryNames(t *testing.T) {
	m := model{vpn: vpnStatus{
		known: true, connected: true,
		country: "United Kingdom", city: "Manchester",
	}}

	tests := []struct {
		name   string
		target Target
		want   bool
	}{
		{name: "country", target: Target{Type: tCountry, Country: "United_Kingdom"}, want: true},
		{name: "exact city", target: Target{Type: tCity, Country: "United_Kingdom", City: "Manchester"}, want: true},
		{name: "other city", target: Target{Type: tCity, Country: "United_Kingdom", City: "London"}, want: false},
		{name: "other country", target: Target{Type: tCountry, Country: "Germany"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := m.targetIsConnected(tt.target); got != tt.want {
				t.Fatalf("targetIsConnected() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConnectedCountryHasVisibleListMarker(t *testing.T) {
	m := model{
		filtTargets: []Target{{Type: tCountry, Label: "United_Kingdom, fastest", Country: "United_Kingdom"}},
		vpn: vpnStatus{
			known: true, connected: true, country: "United Kingdom",
		},
	}

	if got := m.renderLocs(40, 4); !strings.Contains(got, "● United_Kingdom, fastest") {
		t.Fatalf("connected marker missing from location list: %q", got)
	}
}

func groupNames(targets []Target) []string {
	var names []string
	for _, t := range targets {
		names = append(names, t.Group)
	}
	return names
}

func TestAvailableSpecialtyGroupsFollowsCLI(t *testing.T) {
	tests := []struct {
		name   string
		groups []string
		want   string
	}{
		{name: "nordvpn 5.x", groups: []string{"Dedicated_IP", "Double_VPN", "Onion_Over_VPN", "P2P", "Standard_VPN_Servers"},
			want: "Double_VPN Onion_Over_VPN P2P Dedicated_IP"},
		{name: "nordvpn 6.0", groups: []string{"Dedicated_IP", "Double_VPN", "Obfuscated", "Onion_Over_VPN", "Standard_VPN_Servers"},
			want: "Double_VPN Onion_Over_VPN Dedicated_IP Obfuscated"},
		{name: "not fetched yet", groups: nil, want: "Double_VPN Onion_Over_VPN Dedicated_IP"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := strings.Join(groupNames(availableSpecialtyGroups(tt.groups)), " "); got != tt.want {
				t.Fatalf("availableSpecialtyGroups() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseVPNStatusShowsSpecialtyGroup(t *testing.T) {
	status := parseVPNStatus(`Status: Connected
Server: Sweden #12
Hostname: se12.nordvpn.com
Country: Sweden
City: Stockholm
Group: Double VPN
Current technology: NORDLYNX
`)
	if status.group != "Double VPN" {
		t.Fatalf("group not parsed: %#v", status)
	}
	if status.text != "Connected — Double VPN — Sweden / Stockholm — (se12.nordvpn.com)" {
		t.Fatalf("unexpected display text: %q", status.text)
	}
}

func TestCacheWithoutGroupsIsStale(t *testing.T) {
	fresh := time.Now().UTC().Format(time.RFC3339)
	if !cacheIsStale(Cache{UpdatedAt: fresh}) {
		t.Fatal("cache without a groups list should be refreshed")
	}
	if cacheIsStale(Cache{UpdatedAt: fresh, Groups: []string{}}) {
		t.Fatal("fresh cache with groups should not be stale")
	}
}

func TestP2PFallbackArgs(t *testing.T) {
	got := strings.Join(p2pFallbackArgs(Target{Type: tGroup, Group: "P2P", Country: "Netherlands"}), " ")
	if got != "connect Netherlands" {
		t.Fatalf("p2pFallbackArgs() = %q", got)
	}
	if got := strings.Join(p2pFallbackArgs(Target{Type: tGroup, Group: "P2P"}), " "); got != "connect" {
		t.Fatalf("p2pFallbackArgs() without country = %q", got)
	}
}
