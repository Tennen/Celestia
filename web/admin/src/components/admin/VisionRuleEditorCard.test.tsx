import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import type { VisionRule } from '../../lib/types';
import { VisionRuleEditorCard } from './VisionRuleEditorCard';

function buildRule(): VisionRule {
  return {
    id: 'feeder-zone',
    name: 'Feeder Zone',
    enabled: true,
    recognition_enabled: true,
    camera_device_id: 'hikvision:camera:entry-1',
    rtsp_source: { url: 'rtsp://camera/live' },
    entity_selector: { kind: 'label', value: 'cat' },
    zone: { x: 0.1, y: 0.2, width: 0.3, height: 0.4 },
    stay_threshold_seconds: 5,
  };
}

describe('VisionRuleEditorCard', () => {
  it('keeps the zone editor collapsed until explicitly expanded', async () => {
    const user = userEvent.setup();

    render(
      <VisionRuleEditorCard
        catalog={null}
        catalogMismatch={false}
        cameraDevices={[]}
        loading={false}
        onSaveRule={vi.fn()}
        onRemoveRule={vi.fn()}
        onSelectRuleId={vi.fn()}
        onUpdateRule={vi.fn()}
        onViewHistory={vi.fn()}
        saving={false}
        selectedRule={buildRule()}
      />,
    );

    expect(screen.queryByText(/drag on the frame to redraw the target zone/i)).toBeNull();
    expect(screen.getByText(/live rtsp preview stays disconnected until you expand the editor/i)).not.toBeNull();

    await user.click(screen.getByRole('button', { name: /show zone editor/i }));

    expect(screen.getByText(/drag on the frame to redraw the target zone/i)).not.toBeNull();
  });
});
