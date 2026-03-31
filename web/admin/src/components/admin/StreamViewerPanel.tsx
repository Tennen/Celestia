import { useStreamSession } from '../../hooks/useStreamSession';

interface Props {
  deviceId: string;
}

export function StreamViewerPanel({ deviceId }: Props) {
  const { state, errorMessage, videoRef, startStream, stopStream } = useStreamSession(deviceId);

  if (state === 'idle') {
    return (
      <div className="stream-viewer">
        <button type="button" onClick={() => void startStream()}>
          Live View
        </button>
      </div>
    );
  }

  if (state === 'connecting') {
    return (
      <div className="stream-viewer">
        <p>Connecting...</p>
      </div>
    );
  }

  if (state === 'error') {
    return (
      <div className="stream-viewer stream-viewer--error">
        <p>{errorMessage ?? 'Stream error'}</p>
        <button type="button" onClick={() => void startStream()}>
          Retry
        </button>
      </div>
    );
  }

  // active state
  return (
    <div className="stream-viewer stream-viewer--active">
      <video ref={videoRef} autoPlay playsInline muted style={{ width: '100%' }} />
      <button type="button" onClick={() => void stopStream()}>
        Stop
      </button>
    </div>
  );
}
