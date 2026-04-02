package client

// CameraConfig holds the per-camera connection parameters used by CameraClient.
// It is defined here so that the client package has no dependency on internal/app.
type CameraConfig struct {
	Name                     string
	EntryID                  string
	DeviceID                 string
	Host                     string
	Port                     int
	Username                 string
	Password                 string
	Channel                  int
	RTSPPort                 int
	RTSPPath                 string
	PTZDefaultSpeed          int
	PTZStepMS                int
	SDKLibDir                string
	MaxStreamSessions        int
	StreamIdleTimeoutSeconds int
	WebRTCNATIP              string
	WebRTCInterface          string
}

// SDK / connection defaults shared between the client and the app config parser.
const (
	DefaultSDKPort   = 8000
	DefaultChannel   = 1
	DefaultRTSPPort  = 554
	DefaultRTSPPath  = "/Streaming/Channels/{channel}01"
	DefaultPTZSpeed  = 4
	DefaultPTZStepMS = 400
)
