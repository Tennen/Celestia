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
  const sseRef = useRef<EventSource | null>(null);

  const cleanup = useCallback(
    async (sessionId: string | null, pc: RTCPeerConnection | null) => {
      // Close SSE listener
      if (sseRef.current) {
        sseRef.current.close();
        sseRef.current = null;
      }

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

    const pc = new RTCPeerConnection({});
    pcRef.current = pc;

    // When a remote track arrives, attach it to the video element
    pc.ontrack = (event) => {
      if (videoRef.current && event.streams[0]) {
        videoRef.current.srcObject = event.streams[0];
      }
      setState('active');
    };

    try {
      const offer = await pc.createOffer();
      await pc.setLocalDescription(offer);

      // POST offer to server
      const offerRes = await fetch(`${getApiBase()}/devices/${deviceId}/stream/offer`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ sdp: offer.sdp }),
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

      // Send local ICE candidates to the server
      pc.onicecandidate = (event) => {
        if (!event.candidate) return;
        const sid = sessionIdRef.current;
        if (!sid) return;
        void fetch(`${getApiBase()}/devices/${deviceId}/stream/ice`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ session_id: sid, candidate: event.candidate.candidate }),
        });
      };

      // Listen for remote ICE candidates via SSE
      const source = new EventSource(`${getApiBase()}/events/stream`);
      sseRef.current = source;
      source.addEventListener('device.event.occurred', (e: MessageEvent) => {
        try {
          const payload = JSON.parse(e.data as string) as {
            event_type?: string;
            session_id?: string;
            candidate?: string;
          };
          if (
            payload.event_type === 'ice_candidate' &&
            payload.session_id === sessionIdRef.current &&
            payload.candidate
          ) {
            void pc.addIceCandidate({ candidate: payload.candidate });
          }
        } catch {
          // ignore malformed events
        }
      });
      source.onerror = () => {
        source.close();
      };
    } catch (err) {
      pc.close();
      pcRef.current = null;
      setState('error');
      setErrorMessage(err instanceof Error ? err.message : 'Stream error');
    }
  }, [deviceId, stopStream]);

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
