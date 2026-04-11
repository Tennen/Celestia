package stream

import (
	"strings"

	"github.com/bluenviron/gortsplib/v4"
)

type RTSPTransport string

const (
	RTSPTransportUDP RTSPTransport = "udp"
	RTSPTransportTCP RTSPTransport = "tcp"
)

func ParseRTSPTransport(raw string) RTSPTransport {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(RTSPTransportTCP):
		return RTSPTransportTCP
	default:
		return RTSPTransportUDP
	}
}

func (t RTSPTransport) String() string {
	return string(ParseRTSPTransport(string(t)))
}

func newRTSPClient(transport RTSPTransport) *gortsplib.Client {
	client := &gortsplib.Client{}
	if ParseRTSPTransport(string(transport)) == RTSPTransportTCP {
		tcp := gortsplib.TransportTCP
		client.Transport = &tcp
	}
	return client
}
