package pluginutil

import "testing"

func TestResolveJSWorkerSocketEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		wantNetwork string
		wantAddress string
		expectErr   bool
	}{
		{
			name:        "unix socket path",
			raw:         "/tmp/auralogic-jsworker.sock",
			wantNetwork: "unix",
			wantAddress: "/tmp/auralogic-jsworker.sock",
		},
		{
			name:        "loopback ipv4 tcp",
			raw:         "tcp://127.0.0.1:17345",
			wantNetwork: "tcp",
			wantAddress: "127.0.0.1:17345",
		},
		{
			name:        "loopback ipv6 tcp",
			raw:         "tcp://[::1]:17345",
			wantNetwork: "tcp",
			wantAddress: "[::1]:17345",
		},
		{
			name:        "localhost tcp",
			raw:         "tcp://plugin.localhost:17345",
			wantNetwork: "tcp",
			wantAddress: "plugin.localhost:17345",
		},
		{
			name:      "wildcard tcp rejected",
			raw:       "tcp://0.0.0.0:17345",
			expectErr: true,
		},
		{
			name:      "remote tcp rejected",
			raw:       "tcp://192.168.1.10:17345",
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			network, address, err := ResolveJSWorkerSocketEndpoint(tc.raw)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveJSWorkerSocketEndpoint(%q) returned error: %v", tc.raw, err)
			}
			if network != tc.wantNetwork || address != tc.wantAddress {
				t.Fatalf("expected (%s, %s), got (%s, %s)", tc.wantNetwork, tc.wantAddress, network, address)
			}
		})
	}
}
