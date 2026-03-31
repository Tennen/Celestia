import { useCallback, useEffect, useRef, useState } from 'react';
import { getApiBase } from '../lib/api';

type StreamState = 'idle' | 'connecting' | 'active' | 'error';

interface UseStreamSessionResult {
  state: StreamState;
  errorMessage: string | null;
  videoRef: React.RefObject<HTMLVideoElement>;
  startStream: () => Promise<void>;
  stopStream: () => Promise<void>;
}

export function useStreamSession(deviceId: string): UseStreamSessionResult {
  const [state, setState] = useState<StreamState>('idle');
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const pcRef = useRef<RTCPeerConnection | null>(null);
  const sessionIdRef = useRef<string | null>(null);
  // Holds the MediaStream if ontrack fires before the video element is mounted
  const pendingStreamRef = useRef<MediaStream | null>(null);

  const cleanup = useCallback(
    async (sessionId: string | null, pc: RTCPeerConnection | null) => {
      pendingStreamRef.current = null;

      // Send DELETE to close the server-side session
      if (sessionId) {
        try {
          await fetch(`${getApiBase()}/devices/${deviceId}/stream/${sessionId}`, {
            method: 'DELETE',
          });
        } catch {
          // best-effort
        }
      }

      // Close PeerConnection
      if (pc) {
        pc.close();
      }
    },
    [deviceId],
  );

  const stopStream = useCallback(async () => {
    const sessionId = sessionIdRef.current;
    const pc = pcRef.current;
    sessionIdRef.current = null;
    pcRef.current = null;
    await cleanup(sessionId, pc);
    setState('idle');
    setErrorMessage(null);
  }, [cleanup]);

  const startStream = useCallback(async () => {
    // Tear down any existing session first
    if (pcRef.current || sessionIdRef.current) {
      await stopStream();
    }

    setState('connecting');
    setErrorMessage(null);

    const pc = new RTCPeerConnection({
      iceServers: [{ urls: 'stun:stun.l.google.com:19302' }],
    });
    pcRef.current = pc;

    // Add recvonly transceivers so the browser generates a proper SDP offer
    // with m=video (and m=audio) sections. Without these, createOffer() produces
    // an empty SDP that pion cannot parse.
    //
    // Bind receiver tracks to a MediaStream immediately (go2rtc style) so the
    // video element gets the stream as soon as the connection is established,
    // without relying on the ontrack event timing.
    const stream = new MediaStream([
      pc.addTransceiver('video', { direction: 'recvonly' }).receiver.track,
      pc.addTransceiver('audio', { direction: 'recvonly' }).receiver.track,
    ]);

    // Attach stream to video as soon as ICE connects — don't wait for ontrack
    // which may not fire in all browser/pion configurations.
    const attachStream = () => {
      if (videoRef.current) {
        videoRef.current.srcObject = stream;
        void videoRef.current.play().catch(() => {
          // Autoplay blocked; user interaction required — state still goes active
          // so the video element is visible and the user can click to play.
        });
      } else {
        pendingStreamRef.current = stream;
      }
      setState('active');
    };

    pc.oniceconnectionstatechange = () => {
      if (pc.iceConnectionState === 'connected' || pc.iceConnectionState === 'completed') {
        attachStream();
      }
    };

    pc.ontrack = () => {
      attachStream();
    };

    try {
      const offer = await pc.createOffer();
      await pc.setLocalDescription(offer);

      // Wait for ICE gathering to complete (or 3s timeout) before sending the
      // offer. This means the offer already contains all local candidates, so
      // the server can return a complete answer in one round-trip with no
      // separate trickle-ICE exchange needed.
      const fullOfferSDP = await waitForGathering(pc, 3000);

      // POST offer to server
      const offerRes = await fetch(`${getApiBase()}/devices/${deviceId}/stream/offer`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ sdp: fullOfferSDP }),
      });

      if (!offerRes.ok) {
        let msg = `Offer failed: ${offerRes.status}`;
        try {
          const body = (await offerRes.json()) as { error?: string };
          if (body?.error) msg = body.error;
        } catch {
          // ignore
        }
        pc.close();
        pcRef.current = null;
        setState('error');
        setErrorMessage(msg);
        return;
      }

      const answer = (await offerRes.json()) as { session_id: string; sdp: string };
      sessionIdRef.current = answer.session_id;

      await pc.setRemoteDescription({ type: 'answer', sdp: answer.sdp });

    } catch (err) {
      pc.close();
      pcRef.current = null;
      setState('error');
      setErrorMessage(err instanceof Error ? err.message : 'Stream error');
    }
  }, [deviceId, stopStream]);

  // After state becomes 'active' the video element is mounted.
  // Attach any stream that arrived before the element existed.
  useEffect(() => {
    if (state === 'active' && videoRef.current && pendingStreamRef.current) {
      videoRef.current.srcObject = pendingStreamRef.current;
      void videoRef.current.play().catch(() => {});
      pendingStreamRef.current = null;
    }
  }, [state]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      const sessionId = sessionIdRef.current;
      const pc = pcRef.current;
      sessionIdRef.current = null;
      pcRef.current = null;
      void cleanup(sessionId, pc);
    };
  }, [cleanup]);

  return { state, errorMessage, videoRef, startStream, stopStream };
}

/**
 * Wait for ICE gathering to complete or fall back after `timeoutMs`.
 * Returns the full SDP with all local candidates embedded, matching the
 * go2rtc vanilla-ICE approach: one round-trip, no trickle exchange needed.
 */
function waitForGathering(pc: RTCPeerConnection, timeoutMs: number): Promise<string> {
  return new Promise((resolve) => {
    if (pc.iceGatheringState === 'complete') {
      resolve(pc.localDescription!.sdp);
      return;
    }
    const onStateChange = () => {
      if (pc.iceGatheringState === 'complete') {
        pc.removeEventListener('icegatheringstatechange', onStateChange);
        resolve(pc.localDescription!.sdp);
      }
    };
    pc.addEventListener('icegatheringstatechange', onStateChange);
    setTimeout(() => {
      pc.removeEventListener('icegatheringstatechange', onStateChange);
      resolve(pc.localDescription!.sdp);
    }, timeoutMs);
  });
}
