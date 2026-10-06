package besticon

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsPublicIP(t *testing.T) {
	tests := map[string]bool{
		"8.8.8.8":                true,
		"2001:4860:4860::8888":   true,
		"::ffff:8.8.8.8":         true,
		"127.0.0.1":              false,
		"10.96.0.10":             false,
		"172.16.0.1":             false,
		"192.168.1.1":            false,
		"169.254.169.254":        false,
		"100.64.0.1":             false,
		"0.0.0.0":                false,
		"255.255.255.255":        false,
		"::1":                    false,
		"fd00::1":                false,
		"fe80::1":                false,
		"::ffff:169.254.169.254": false,
		"64:ff9b::a9fe:a9fe":     false,
	}

	for address, want := range tests {
		if got := isPublicIP(net.ParseIP(address)); got != want {
			t.Errorf("isPublicIP(%s) = %v, want %v", address, got, want)
		}
	}
}

func TestDefaultHTTPClientRefusesNonPublicAddresses(t *testing.T) {
	requested := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = true
	}))
	defer server.Close()

	_, err := New().Get(server.URL)

	if !errors.Is(err, errNonPublicAddress) {
		t.Fatalf("expected errNonPublicAddress, got %v", err)
	}
	if requested {
		t.Fatal("request reached the local server")
	}
}
