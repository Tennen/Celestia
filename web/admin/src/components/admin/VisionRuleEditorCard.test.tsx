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
    behavior: '',
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
        catalogMatchesDraft={false}
        catalogMatchesSaved={false}
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

  it('shows specific entities from the current settings draft catalog before settings are saved', () => {
    render(
      <VisionRuleEditorCard
        catalog={{
          service_ws_url: 'ws://vision-draft.example/api/v1/capabilities/vision_entity_stay_zone',
          schema_version: 'celestia.vision.catalog.v1',
          model_name: 'yolo11m-coco',
          service_version: '1.2.0',
          fetched_at: '2026-04-12T01:00:00Z',
          entities: [
            { kind: 'label', value: 'cat', display_name: 'Cat' },
            { kind: 'label', value: 'dog', display_name: 'Dog' },
          ],
        }}
        catalogMatchesDraft={true}
        catalogMatchesSaved={false}
        cameraDevices={[]}
        loading={false}
        onSaveRule={vi.fn()}
        onRemoveRule={vi.fn()}
        onSelectRuleId={vi.fn()}
        onUpdateRule={vi.fn()}
        onViewHistory={vi.fn()}
        saving={false}
        selectedRule={{
          ...buildRule(),
          entity_selector: { kind: 'label', value: '' },
        }}
      />,
    );

    expect(screen.getByRole('option', { name: 'All Entities In Zone' })).not.toBeNull();
    expect(screen.getByRole('option', { name: 'Cat' })).not.toBeNull();
    expect(screen.getByRole('option', { name: 'Dog' })).not.toBeNull();
    expect(
      screen.getByText(/save recognition settings before relying on these specific entity choices in rule saves/i),
    ).not.toBeNull();
  });

  it('renders an editable behavior hint field for semantic fallback prompts', () => {
    render(
      <VisionRuleEditorCard
        catalog={null}
        catalogMatchesDraft={false}
        catalogMatchesSaved={false}
        cameraDevices={[]}
        loading={false}
        onSaveRule={vi.fn()}
        onRemoveRule={vi.fn()}
        onSelectRuleId={vi.fn()}
        onUpdateRule={vi.fn()}
        onViewHistory={vi.fn()}
        saving={false}
        selectedRule={{
          ...buildRule(),
          behavior: 'eating',
        }}
      />,
    );

    expect(screen.getByDisplayValue('eating')).not.toBeNull();
    expect(screen.getByText(/optional semantic hint/i)).not.toBeNull();
  });
});
