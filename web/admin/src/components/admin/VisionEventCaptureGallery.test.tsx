import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { VisionEventCaptureGallery, visionEventCapturesFromPayload } from './VisionEventCaptureGallery';

vi.mock('../../lib/api', () => ({
  visionCaptureURL: (captureId: string) => `/captures/${captureId}`,
}));

function capture(captureId: string, phase: string) {
  return {
    capture_id: captureId,
    event_id: 'evt-1',
    phase,
    captured_at: '2026-04-20T08:00:00Z',
    content_type: 'image/jpeg',
    size_bytes: 128,
  };
}

describe('VisionEventCaptureGallery', () => {
  it('keeps more than three captures and arbitrary phase labels', () => {
    const captures = visionEventCapturesFromPayload({
      captures: [
        capture('evt-1:start', 'start'),
        capture('evt-1:middle', 'middle'),
        capture('evt-1:end', 'end'),
        capture('evt-1:sample-1', 'sample-1'),
      ],
    });

    expect(captures).toHaveLength(4);
    expect(captures[3].phase).toBe('sample-1');

    const { container } = render(<VisionEventCaptureGallery captures={captures} />);

    expect(screen.getAllByRole('link')).toHaveLength(4);
    expect(screen.getByText('sample-1')).toBeTruthy();
    expect(container.firstElementChild?.className).toContain('vision-capture-gallery--scroll');
  });
});
