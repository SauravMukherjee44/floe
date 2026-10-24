package main

import (
	"errors"
	"net"
	"strings"
	"testing"
)

// Reuse the command harness so completion goes through execute's link guard
// and captures both direct printing and Cobra's output writers.
func runCompletionCLI(t *testing.T, args ...string) (string, string) {
	t.Helper()
	// Completing a repeated --iface parses the earlier occurrence. The shared
	// harness excludes this slice flag, so restore it before any transfer test.
	ifaceFlag := rootCmd.PersistentFlags().Lookup("iface")
	prevIfaces, prevChanged := append([]string(nil), flagIface...), ifaceFlag.Changed
	t.Cleanup(func() {
		flagIface = prevIfaces
		ifaceFlag.Changed = prevChanged
	})
	stdout, stderr, err := runUpdateCLI(t, append([]string{"__complete"}, args...)...)
	if err != nil {
		t.Fatalf("completion failed: %v (stdout %q, stderr %q)", err, stdout, stderr)
	}
	return stdout, stderr
}

func TestCompletionArgumentsAndFlags(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"receive code", []string{"receive", ""}, ":4\n"},
		{"request link", []string{"send", "--to", ""}, ":4\n"},
		{"server", []string{"--server", ""}, ":4\n"},
		{"web", []string{"--web", ""}, ":4\n"},
		{"inherited server", []string{"receive", "--server", ""}, ":4\n"},
		{"inherited web", []string{"send", "--web", ""}, ":4\n"},
		{"version", []string{"version", ""}, ":4\n"},
		{"update", []string{"update", ""}, ":4\n"},
		{"output short", []string{"receive", "-o", ""}, ":16\n"},
		{"output long", []string{"receive", "--output", ""}, ":16\n"},
		{"output prefix", []string{"receive", "--output=Down"}, ":16\n"},
		{"send paths", []string{"send", ""}, ":0\n"},
		{"more send paths", []string{"send", "first.txt", ""}, ":0\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, stderr := runCompletionCLI(t, tt.args...)
			if out != tt.want {
				t.Fatalf("completion printed %q, want %q (stderr %q)", out, tt.want, stderr)
			}
		})
	}
}

func stubCompletionInterfaces(t *testing.T, interfaces []net.Interface, err error) {
	t.Helper()
	prev := listInterfaces
	t.Cleanup(func() { listInterfaces = prev })
	listInterfaces = func() ([]net.Interface, error) { return interfaces, err }
}

func TestCompletionInterfaces(t *testing.T) {
	stubCompletionInterfaces(t, []net.Interface{
		{Name: "lo0"},
		{Name: "Ethernet 2"},
		{Name: "vEthernet (WSL)"},
		{Name: "Wi-Fi"},
	}, nil)
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"all names", []string{"--iface", ""}, "lo0\nEthernet 2\nvEthernet (WSL)\nWi-Fi\n:4\n"},
		{"case insensitive substring", []string{"--iface", "THER"}, "Ethernet 2\nvEthernet (WSL)\n:4\n"},
		{"parentheses", []string{"--iface", "(wsl)"}, "vEthernet (WSL)\n:4\n"},
		{"spaces", []string{"--iface", "net 2"}, "Ethernet 2\n:4\n"},
		{"no match", []string{"--iface", "missing"}, ":4\n"},
		{"inherited", []string{"send", "--iface", "fi"}, "Wi-Fi\n:4\n"},
		{"equals", []string{"--iface=lo"}, "lo0\n:4\n"},
		{"repeated flag", []string{"--iface", "lo0", "--iface", "fi"}, "Wi-Fi\n:4\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, stderr := runCompletionCLI(t, tt.args...)
			if out != tt.want {
				t.Fatalf("completion printed %q, want %q (stderr %q)", out, tt.want, stderr)
			}
		})
	}
}

func TestCompletionInterfacesUnavailable(t *testing.T) {
	for _, tt := range []struct {
		name       string
		interfaces []net.Interface
		err        error
	}{
		{name: "empty list"},
		{name: "lookup failed", interfaces: []net.Interface{{Name: "partial result"}}, err: errors.New("private diagnostic")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stubCompletionInterfaces(t, tt.interfaces, tt.err)
			out, stderr := runCompletionCLI(t, "--iface", "")
			if out != ":4\n" || strings.Contains(stderr, "private diagnostic") {
				t.Fatalf("completion printed stdout %q, stderr %q; want no names, no error details and :4", out, stderr)
			}
		})
	}
}

func TestCompletionDoesNotEchoRoomLink(t *testing.T) {
	const link = "https://floe.one/#room=11111111-2222-4333-8444-555555555555"
	for _, cmd := range []string{"receive", "version", "update"} {
		t.Run(cmd, func(t *testing.T) {
			calls := stubNetwork(t, "")
			updates := stubUpdate(t, "1.11.0", "", "")
			out, stderr := runCompletionCLI(t, cmd, link)
			if out != ":4\n" || stderr != "Completion ended with directive: ShellCompDirectiveNoFileComp\n" {
				t.Fatalf("completion printed stdout %q, stderr %q; want only :4 and no link outcome", out, stderr)
			}
			if calls.ice.Load() != 0 || calls.connect.Load() != 0 || updates.lookups != 0 || len(updates.applied) != 0 {
				t.Fatal("completion ran a transfer or update")
			}
		})
	}
}
