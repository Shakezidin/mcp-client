package observability

import "testing"

func TestResolveEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		port     int
		want     string
	}{
		{name: "empty endpoint uses default host and port", endpoint: "", port: 4317, want: "localhost:4317"},
		{name: "host only appends configured port", endpoint: "otel-collector", port: 4317, want: "otel-collector:4317"},
		{name: "host port is preserved", endpoint: "otel-collector:4317", port: 4318, want: "otel-collector:4317"},
		{name: "ipv4 host appends configured port", endpoint: "127.0.0.1", port: 4318, want: "127.0.0.1:4318"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveEndpoint(tt.endpoint, tt.port)
			if got != tt.want {
				t.Fatalf("resolveEndpoint(%q, %d) = %q, want %q", tt.endpoint, tt.port, got, tt.want)
			}
		})
	}
}

func TestBuildOTLPDialOptions(t *testing.T) {
	opts := buildOTLPDialOptions(true)
	if len(opts) == 0 {
		t.Fatal("expected dial options to be configured")
	}
}
