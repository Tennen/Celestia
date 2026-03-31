import { Button } from '../ui/button';
import { useStreamSession } from '../../hooks/useStreamSession';

interface Props {
  deviceId: string;
}

export function StreamViewerPanel({ deviceId }: Props) {
  const { state, errorMessage, videoRef, startStream, stopStream } = useStreamSession(deviceId);

  if (state === 'idle') {
    return (
      <div className="stream-viewer">
        <Button variant="secondary" size="sm" onClick={() => void startStream()}>
          Live View
        </Button>
      </div>
    );
  }

  if (state === 'connecting') {
    return (
      <div className="stream-viewer">
        <Button variant="secondary" size="sm" disabled>
          Connecting…
        </Button>
      </div>
    );
  }

  if (state === 'error') {
    return (
      <div className="stream-viewer stream-viewer--error">
        <p className="muted">{errorMessage ?? 'Stream error'}</p>
        <Button variant="secondary" size="sm" onClick={() => void startStream()}>
          Retry
        </Button>
      </div>
    );
  }

  // active
  return (
    <div className="stream-viewer stream-viewer--active">
      <video ref={videoRef} autoPlay playsInline muted style={{ width: '100%' }} />
      <Button variant="ghost" size="sm" onClick={() => void stopStream()}>
        Stop
      </Button>
    </div>
  );
}
