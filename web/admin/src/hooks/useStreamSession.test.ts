/**
 * Unit tests for useStreamSession hook
 * Requirements: 7.4, 7.7
 */
import { renderHook, act } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { useStreamSession } from './useStreamSession';

// RTCPeerConnection is not available in jsdom — provide a minimal mock
const mockPc = {
  createOffer: vi.fn().mockResolvedValue({ type: 'offer', sdp: 'offer-sdp' }),
  setLocalDescription: vi.fn().mockResolvedValue(undefined),
  setRemoteDescription: vi.fn().mockResolvedValue(undefined),
  addIceCandidate: vi.fn().mockResolvedValue(undefined),
  close: vi.fn(),
  ontrack: null as ((e: RTCTrackEvent) => void) | null,
  onicecandidate: null as ((e: RTCPeerConnectionIceEvent) => void) | null,
};

// Minimal EventSource mock
class MockEventSource {
  static instances: MockEventSource[] = [];
  listeners: Record<string, ((e: MessageEvent) => void)[]> = {};
  onerror: (() => void) | null = null;
  onmessage: (() => void) | null = null;
  close = vi.fn();

  constructor(public url: string) {
    MockEventSource.instances.push(this);
  }

  addEventListener(type: string, handler: (e: MessageEvent) => void) {
    if (!this.listeners[type]) this.listeners[type] = [];
    this.listeners[type].push(handler);
  }
}

beforeEach(() => {
  vi.resetAllMocks();
  MockEventSource.instances = [];

  // Reset mock implementations
  mockPc.createOffer.mockResolvedValue({ type: 'offer', sdp: 'offer-sdp' });
  mockPc.setLocalDescription.mockResolvedValue(undefined);
  mockPc.setRemoteDescription.mockResolvedValue(undefined);
  mockPc.addIceCandidate.mockResolvedValue(undefined);
  mockPc.close.mockReset();
  mockPc.ontrack = null;
  mockPc.onicecandidate = null;

  global.RTCPeerConnection = vi.fn().mockImplementation(() => mockPc) as unknown as typeof RTCPeerConnection;
  global.EventSource = MockEventSource as unknown as typeof EventSource;
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('useStreamSession', () => {
  /**
   * Test 1: error state is set and PeerConnection is closed when offer request fails
   * Requirements: 7.4, 7.7
   */
  it('sets error state and closes PeerConnection when offer request fails', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      json: async () => ({ error: 'internal server error' }),
    } as Response);

    const { result } = renderHook(() => useStreamSession('device-1'));

    expect(result.current.state).toBe('idle');

    await act(async () => {
      await result.current.startStream();
    });

    expect(result.current.state).toBe('error');
    expect(result.current.errorMessage).toBeTruthy();
    expect(mockPc.close).toHaveBeenCalled();
  });

  /**
   * Test 2: stopStream sends DELETE and closes PeerConnection
   * Requirements: 7.4, 7.7
   */
  it('sends DELETE and closes PeerConnection on stopStream', async () => {
    const sessionId = 'test-session-123';

    global.fetch = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      if (typeof url === 'string' && url.includes('/stream/offer')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: async () => ({ session_id: sessionId, sdp: 'answer-sdp' }),
        } as Response);
      }
      // DELETE or ICE
      return Promise.resolve({
        ok: true,
        status: 204,
        json: async () => ({}),
      } as Response);
    });

    const { result } = renderHook(() => useStreamSession('device-2'));

    await act(async () => {
      await result.current.startStream();
    });

    await act(async () => {
      await result.current.stopStream();
    });

    const fetchMock = global.fetch as ReturnType<typeof vi.fn>;
    const calls = fetchMock.mock.calls as [string, RequestInit | undefined][];
    const deleteCall = calls.find(
      ([url, init]) => typeof url === 'string' && url.includes(`/stream/${sessionId}`) && init?.method === 'DELETE',
    );

    expect(deleteCall).toBeDefined();
    expect(mockPc.close).toHaveBeenCalled();
    expect(result.current.state).toBe('idle');
  });
});
