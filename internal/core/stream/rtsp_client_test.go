package stream

import (
	"testing"

	"github.com/bluenviron/gortsplib/v4"
)

func TestParseRTSPTransport(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want RTSPTransport
	}{
		{name: "default empty", raw: "", want: RTSPTransportUDP},
		{name: "default invalid", raw: "quic", want: RTSPTransportUDP},
		{name: "tcp mixed case", raw: " TCP ", want: RTSPTransportTCP},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseRTSPTransport(tt.raw); got != tt.want {
				t.Fatalf("ParseRTSPTransport(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestNewRTSPClientSetsTCPTransport(t *testing.T) {
	client := newRTSPClient(RTSPTransportTCP)
	if client.Transport == nil {
		t.Fatal("Transport = nil, want TCP transport")
	}
	if *client.Transport != gortsplib.TransportTCP {
		t.Fatalf("Transport = %v, want %v", *client.Transport, gortsplib.TransportTCP)
	}
}

func TestNewRTSPClientLeavesUDPDefaultUnset(t *testing.T) {
	client := newRTSPClient(RTSPTransportUDP)
	if client.Transport != nil {
		t.Fatalf("Transport = %v, want nil for UDP default", *client.Transport)
	}
}
