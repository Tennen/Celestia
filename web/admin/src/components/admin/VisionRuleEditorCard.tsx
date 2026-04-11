import { cameraRTSPSourceURL, cameraLabel } from '../../lib/capability';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardHeader } from '../ui/card';
import { Input } from '../ui/input';
import { Switch } from '../ui/switch';
import type { DeviceView, VisionEntityCatalog, VisionEntityDescriptor, VisionRule } from '../../lib/types';
import { CardHeading } from './shared/CardHeading';
import { ZoneBoxEditor } from './ZoneBoxEditor';

type Props = {
  catalog: VisionEntityCatalog | null;
  catalogMismatch: boolean;
  cameraDevices: DeviceView[];
  loading: boolean;
  onSaveRule: () => void;
  onRemoveRule: (ruleId: string) => void;
  onSelectRuleId: (ruleId: string) => void;
  onUpdateRule: (ruleId: string, updater: (current: VisionRule) => VisionRule) => void;
  saving: boolean;
  selectedRule: VisionRule | null;
};

const ENTITY_OPTION_SEPARATOR = '::';

function entityOptionValue(kind: string, value: string) {
  return `${kind}${ENTITY_OPTION_SEPARATOR}${value}`;
}

function parseEntityOptionValue(value: string) {
  const [kind, ...rest] = value.split(ENTITY_OPTION_SEPARATOR);
  return { kind, value: rest.join(ENTITY_OPTION_SEPARATOR) };
}

function hasEntity(options: VisionEntityDescriptor[], rule: VisionRule) {
  return options.some((item) => item.kind === rule.entity_selector.kind && item.value === rule.entity_selector.value);
}

function buildEntityOptions(catalog: VisionEntityCatalog | null, selectedRule: VisionRule) {
  const options = [...(catalog?.entities ?? [])];
  if (selectedRule.entity_selector.value && !hasEntity(options, selectedRule)) {
    options.push({
      kind: selectedRule.entity_selector.kind || 'label',
      value: selectedRule.entity_selector.value,
      display_name: `${selectedRule.entity_selector.value} (not in current catalog)`,
    });
  }
  return options;
}

export function VisionRuleEditorCard({
  catalog,
  catalogMismatch,
  cameraDevices,
  loading,
  onSaveRule,
  onRemoveRule,
  onSelectRuleId,
  onUpdateRule,
  saving,
  selectedRule,
}: Props) {
  const entityOptions = selectedRule ? buildEntityOptions(catalog, selectedRule) : [];
  const canUseCatalog = Boolean(selectedRule && catalog && !catalogMismatch && entityOptions.length > 0);
  const selectedCameraDevice =
    selectedRule ? cameraDevices.find((device) => device.device.id === selectedRule.camera_device_id) ?? null : null;
  const resolvedRTSPSourceURL = cameraRTSPSourceURL(selectedCameraDevice);
  const displayedRTSPSourceURL = selectedRule?.rtsp_source.url || resolvedRTSPSourceURL;

  return (
    <Card>
      <CardHeader>
        <CardHeading
          title={selectedRule ? selectedRule.name || 'Rule Editor' : 'Rule Editor'}
          description="Bind a camera and RTSP source to a generic entity stay-zone rule. Gateway persists this config, validates against the fetched entity catalog when available, and pushes the normalized rule to the Vision Service."
          aside={
            selectedRule ? (
              <div className="automation-editor__meta">
                <Badge size="xs" tone={selectedRule.enabled ? 'good' : 'neutral'}>
                  {selectedRule.enabled ? 'enabled' : 'disabled'}
                </Badge>
                <Button variant="secondary" size="sm" onClick={onSaveRule} disabled={saving || loading}>
                  {saving ? 'Saving…' : 'Save Rule'}
                </Button>
                <Button variant="danger" size="sm" onClick={() => onRemoveRule(selectedRule.id)}>
                  Delete Rule
                </Button>
              </div>
            ) : null
          }
        />
      </CardHeader>
      <CardContent className="stack">
        {selectedRule ? (
          <>
            <div className="automation-field-grid">
              <div className="automation-field">
                <label>Name</label>
                <Input
                  value={selectedRule.name}
                  onChange={(event) =>
                    onUpdateRule(selectedRule.id, (current) => ({ ...current, name: event.target.value }))
                  }
                  placeholder="Feeder Zone Stay"
                />
              </div>
              <div className="automation-field">
                <label>Rule ID</label>
                <Input
                  value={selectedRule.id}
                  onChange={(event) => {
                    const nextId = event.target.value;
                    onUpdateRule(selectedRule.id, (current) => ({ ...current, id: nextId }));
                    onSelectRuleId(nextId);
                  }}
                  placeholder="feeder-zone-stay"
                />
              </div>
            </div>

            <div className="automation-field-grid">
              <div className="automation-field">
                <label>Camera Device</label>
                <select
                  className="select"
                  value={selectedRule.camera_device_id}
                  onChange={(event) => {
                    const nextCameraDevice = cameraDevices.find((device) => device.device.id === event.target.value) ?? null;
                    onUpdateRule(selectedRule.id, (current) => ({
                      ...current,
                      camera_device_id: event.target.value,
                      rtsp_source: {
                        ...current.rtsp_source,
                        url: cameraRTSPSourceURL(nextCameraDevice),
                      },
                    }));
                  }}
                >
                  {cameraDevices.map((device) => (
                    <option key={device.device.id} value={device.device.id}>
                      {cameraLabel(device)}
                    </option>
                  ))}
                </select>
              </div>
              <div className="automation-field">
                <label>Stay Threshold Seconds</label>
                <Input
                  type="number"
                  min={1}
                  step={1}
                  value={selectedRule.stay_threshold_seconds}
                  onChange={(event) =>
                    onUpdateRule(selectedRule.id, (current) => ({
                      ...current,
                      stay_threshold_seconds: Math.max(1, Number(event.target.value) || 1),
                    }))
                  }
                />
              </div>
            </div>

            {catalogMismatch ? (
              <p className="muted">
                Supported entities were fetched from {catalog?.service_ws_url}. Refresh the catalog again for the current Vision
                Service websocket URL and model before saving rules.
              </p>
            ) : null}
            {!catalog && !catalogMismatch ? (
              <p className="muted">Refresh supported entities from the Vision Service to pick a recognized label such as cat.</p>
            ) : null}
            {catalog && !catalogMismatch && entityOptions.length === 0 ? (
              <p className="muted">The current Vision Service catalog is empty, so there are no recognized entities to select yet.</p>
            ) : null}

            <div className="automation-field">
              <label>Recognizable Entity</label>
              <select
                className="select"
                disabled={!canUseCatalog}
                value={entityOptionValue(selectedRule.entity_selector.kind, selectedRule.entity_selector.value)}
                onChange={(event) => {
                  const next = parseEntityOptionValue(event.target.value);
                  onUpdateRule(selectedRule.id, (current) => ({
                    ...current,
                    entity_selector: { kind: next.kind || 'label', value: next.value },
                  }));
                }}
              >
                <option value={entityOptionValue('label', '')}>
                  {canUseCatalog ? 'Select entity' : 'Refresh supported entities before choosing an entity'}
                </option>
                {entityOptions.map((entity) => (
                  <option key={entityOptionValue(entity.kind, entity.value)} value={entityOptionValue(entity.kind, entity.value)}>
                    {entity.display_name || entity.value}
                  </option>
                ))}
              </select>
              <p className="muted">
                {canUseCatalog
                  ? 'Entity choices come from the current Vision Service model catalog.'
                  : 'Configure the Vision Service and refresh its supported entities before selecting a target entity.'}
              </p>
            </div>

            <div className="automation-field">
              <label>Resolved RTSP Source</label>
              <Input
                value={displayedRTSPSourceURL}
                placeholder="Resolved automatically from the selected camera stream"
                readOnly
              />
              <p className="muted">
                {displayedRTSPSourceURL
                  ? 'Gateway will sync this camera-derived RTSP URL to the downstream Vision Service.'
                  : 'Selected camera does not currently expose an RTSP URL. Configure the camera stream first.'}
              </p>
            </div>

            <div className="automation-field-grid automation-field-grid--compact">
              <div className="automation-field">
                <label>Rule Enabled</label>
                <div className="vision-switch-row">
                  <Switch
                    checked={selectedRule.enabled}
                    onCheckedChange={(checked) =>
                      onUpdateRule(selectedRule.id, (current) => ({ ...current, enabled: checked }))
                    }
                  />
                  <span>{selectedRule.enabled ? 'Enabled' : 'Disabled'}</span>
                </div>
              </div>
              <div className="automation-field">
                <label>Recognition Toggle</label>
                <div className="vision-switch-row">
                  <Switch
                    checked={selectedRule.recognition_enabled}
                    onCheckedChange={(checked) =>
                      onUpdateRule(selectedRule.id, (current) => ({
                        ...current,
                        recognition_enabled: checked,
                      }))
                    }
                  />
                  <span>{selectedRule.recognition_enabled ? 'Vision active' : 'Vision paused'}</span>
                </div>
              </div>
            </div>

            <div className="automation-field">
              <label>Zone Selection</label>
              <ZoneBoxEditor
                cameraDevice={selectedCameraDevice}
                value={selectedRule.zone}
                onChange={(zone) => onUpdateRule(selectedRule.id, (current) => ({ ...current, zone }))}
              />
            </div>
          </>
        ) : (
          <p className="muted">{loading ? 'Loading vision capability…' : 'Select or create a rule to edit it.'}</p>
        )}
      </CardContent>
    </Card>
  );
}
