package stream

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/format/rtph264"
	"github.com/bluenviron/mediacommon/pkg/codecs/h264"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
)

func forwardRTP(
	ctx context.Context,
	session *Session,
	client *gortsplib.Client,
	videoMedia *description.Media, videoFmt format.Format, videoTrack *webrtc.TrackLocalStaticRTP,
	audioMedia *description.Media, audioFmt format.Format, audioTrack *webrtc.TrackLocalStaticRTP,
) {
	var videoCount uint64

	h264Fmt, isH264 := videoFmt.(*format.H264)

	if isH264 {
		// Decode RTP -> NALUs, inject SPS/PPS before IDR, re-encode -> RTP.
		rtpDec, err := h264Fmt.CreateDecoder()
		if err != nil {
			log.Printf("[stream] session=%s H264 decoder create error: %v", session.ID[:8], err)
			isH264 = false
		}

		rtpEnc := &rtph264.Encoder{
			PayloadType:       h264Fmt.PayloadType(),
			PacketizationMode: h264Fmt.PacketizationMode,
		}
		if err := rtpEnc.Init(); err != nil {
			log.Printf("[stream] session=%s H264 encoder init error: %v", session.ID[:8], err)
			isH264 = false
		}

		if isH264 {
			client.OnPacketRTP(videoMedia, videoFmt, func(pkt *rtp.Packet) {
				select {
				case <-ctx.Done():
					return
				default:
				}
				session.lastActivity = time.Now()

				nalus, err := rtpDec.Decode(pkt)
				if err != nil || len(nalus) == 0 {
					return
				}

				hasIDR := false
				for _, nalu := range nalus {
					if len(nalu) > 0 && h264.NALUType(nalu[0]&0x1f) == h264.NALUTypeIDR {
						hasIDR = true
						break
					}
				}

				if hasIDR {
					var withParams [][]byte
					if len(h264Fmt.SPS) > 0 {
						withParams = append(withParams, h264Fmt.SPS)
					}
					if len(h264Fmt.PPS) > 0 {
						withParams = append(withParams, h264Fmt.PPS)
					}
					nalus = append(withParams, nalus...)
				}

				pkts, err := rtpEnc.Encode(nalus)
				if err != nil {
					return
				}

				for _, p := range pkts {
					p.Timestamp = pkt.Timestamp
					if raw, err := p.Marshal(); err == nil {
						if _, werr := videoTrack.Write(raw); werr != nil && videoCount == 0 {
							log.Printf("[stream] session=%s first video write error: %v", session.ID[:8], werr)
						}
						videoCount++
						if videoCount == 1 || videoCount%300 == 0 {
							log.Printf("[stream] session=%s video RTP count=%d pt=%d hasIDR=%v",
								session.ID[:8], videoCount, p.PayloadType, hasIDR)
						}
					}
				}
			})
		}
	}

	if !isH264 {
		client.OnPacketRTP(videoMedia, videoFmt, func(pkt *rtp.Packet) {
			select {
			case <-ctx.Done():
				return
			default:
			}
			session.lastActivity = time.Now()
			if raw, err := pkt.Marshal(); err == nil {
				if _, werr := videoTrack.Write(raw); werr != nil && videoCount == 0 {
					log.Printf("[stream] session=%s first video write error: %v", session.ID[:8], werr)
				}
				videoCount++
				if videoCount == 1 || videoCount%300 == 0 {
					log.Printf("[stream] session=%s video RTP count=%d pt=%d", session.ID[:8], videoCount, pkt.PayloadType)
				}
			}
		})
	}

	if audioMedia != nil && audioFmt != nil && audioTrack != nil {
		client.OnPacketRTP(audioMedia, audioFmt, func(pkt *rtp.Packet) {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if raw, err := pkt.Marshal(); err == nil {
				_, _ = audioTrack.Write(raw)
			}
		})
	}

	log.Printf("[stream] session=%s forwardRTP started, calling client.Wait()", session.ID[:8])
	waitDone := make(chan struct{})
	go func() {
		defer close(waitDone)
		err := client.Wait()
		log.Printf("[stream] session=%s client.Wait() returned: %v (video=%d)", session.ID[:8], err, videoCount)
	}()

	select {
	case <-ctx.Done():
		client.Close()
		<-waitDone
	case <-waitDone:
	}
}

func findFormats(desc *description.Session) (
	videoMedia *description.Media, videoFmt format.Format,
	audioMedia *description.Media, audioFmt format.Format,
) {
	var h264 *format.H264
	if m := desc.FindFormat(&h264); m != nil {
		videoMedia, videoFmt = m, h264
	}
	var g711 *format.G711
	if m := desc.FindFormat(&g711); m != nil {
		audioMedia, audioFmt = m, g711
	}
	return
}

func addVideoTrack(pc *webrtc.PeerConnection, videoFmt format.Format) (*webrtc.TrackLocalStaticRTP, error) {
	mimeType := webrtc.MimeTypeH264
	clockRate := uint32(90000)
	if f, ok := videoFmt.(*format.H264); ok {
		clockRate = uint32(f.ClockRate())
	}
	track, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: mimeType, ClockRate: clockRate},
		"video", "rtsp-relay",
	)
	if err != nil {
		return nil, err
	}
	if _, err := pc.AddTrack(track); err != nil {
		return nil, err
	}
	return track, nil
}

func addAudioTrack(pc *webrtc.PeerConnection, audioFmt format.Format) (*webrtc.TrackLocalStaticRTP, error) {
	f, ok := audioFmt.(*format.G711)
	if !ok {
		return nil, fmt.Errorf("unsupported audio format: %T", audioFmt)
	}
	mimeType := "audio/PCMU"
	if !f.MULaw {
		mimeType = "audio/PCMA"
	}
	track, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: mimeType, ClockRate: uint32(f.ClockRate()), Channels: uint16(f.ChannelCount)},
		"audio", "rtsp-relay",
	)
	if err != nil {
		return nil, err
	}
	if _, err := pc.AddTrack(track); err != nil {
		return nil, err
	}
	return track, nil
}

// isLANCandidate returns true for IPv4 addresses plausibly reachable by a
// browser on the same LAN. Rejects loopback, link-local, and /30-or-smaller
// subnets (VPN tunnels, USB tethering).
func isLANCandidate(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	if ip4[0] == 127 || (ip4[0] == 169 && ip4[1] == 254) {
		return false
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return true
	}
	for _, iface := range ifaces {
		if iface.Flags&(net.FlagLoopback|net.FlagPointToPoint) != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || !ipNet.Contains(ip4) {
				continue
			}
			ones, _ := ipNet.Mask.Size()
			if ones >= 30 {
				return false
			}
		}
	}
	return true
}
