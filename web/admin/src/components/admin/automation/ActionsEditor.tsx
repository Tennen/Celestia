import { Button } from '../../ui/button';
import { Input } from '../../ui/input';
import { Textarea } from '../../ui/textarea';
import {
  buildActionTemplates,
  createDefaultAgentAction,
  findDevice,
  getActionKind,
  getStateChangedConditionDeviceId,
  prettyActionParams,
  type AutomationActionTemplate,
  type AutomationActionKind,
} from '../../../lib/automation';
import type { Automation, AutomationAction, DeviceView } from '../../../lib/types';
import { AutomationSection } from './AutomationSection';

type Props = {
  draft: Automation;
  devices: DeviceView[];
  actionParamDrafts: Record<number, string>;
  onChange: (updater: (current: Automation) => Automation) => void;
  onParamDraftChange: (index: number, value: string) => void;
  onApplyTemplate: (index: number, template: AutomationActionTemplate | null) => void;
  onResetParamDrafts: (actions: Automation['actions']) => void;
};

type TouchpointDraft = {
  type?: string;
  to_user?: string;
  device_id?: string;
  action?: string;
  params?: Record<string, unknown>;
};

export function ActionsEditor({
  draft,
  devices,
  actionParamDrafts,
  onChange,
  onParamDraftChange,
  onApplyTemplate,
  onResetParamDrafts,
}: Props) {
  return (
    <AutomationSection
      title="Actions"
      description="Run a device command, or call the Agent and route its output to WeCom or voice-device touchpoints."
      action={
        <Button
          variant="secondary"
          size="sm"
          onClick={() =>
            onChange((current) => {
              const actions = [...current.actions, defaultDeviceAction(current, devices)];
              onResetParamDrafts(actions);
              return { ...current, actions };
            })
          }
        >
          Add Action
        </Button>
      }
    >
      <div className="automation-rule-list">
        {draft.actions.map((action, index) => {
          const kind = getActionKind(action);
          return (
            <div key={`${kind}-${action.device_id}-${index}`} className="automation-rule">
              <div className="automation-rule__header">
                <div className="automation-section__heading">
                  <h4 className="automation-rule__title">Action {index + 1}</h4>
                  <p className="muted">{kind === 'agent' ? 'Call the Agent as an input/output function.' : 'Dispatch a command to an existing device.'}</p>
                </div>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() =>
                    onChange((current) => {
                      const actions = current.actions.filter((_, itemIndex) => itemIndex !== index);
                      onResetParamDrafts(actions);
                      return { ...current, actions };
                    })
                  }
                >
                  Remove
                </Button>
              </div>
              <div className="automation-rule__body">
                <div className="automation-field-grid">
                  <div className="automation-field">
                    <label>Action Kind</label>
                    <select className="select" value={kind} onChange={(event) => switchActionKind(onChange, onResetParamDrafts, index, event.target.value as AutomationActionKind, devices)}>
                      <option value="device">Device command</option>
                      <option value="agent">Agent function</option>
                    </select>
                  </div>
                  <div className="automation-field">
                    <label>Label</label>
                    <Input value={action.label ?? ''} onChange={(event) => updateAction(onChange, index, (current) => ({ ...current, label: event.target.value }))} placeholder={kind === 'agent' ? 'Daily digest' : 'Suggested · Voice push'} />
                  </div>
                </div>

                {kind === 'agent' ? (
                  <AgentActionEditor action={action} index={index} devices={devices} onChange={onChange} />
                ) : (
                  <DeviceActionEditor
                    action={action}
                    index={index}
                    devices={devices}
                    actionParamDrafts={actionParamDrafts}
                    onChange={onChange}
                    onParamDraftChange={onParamDraftChange}
                    onApplyTemplate={onApplyTemplate}
                  />
                )}
              </div>
            </div>
          );
        })}
      </div>
    </AutomationSection>
  );
}

function DeviceActionEditor(props: {
  action: AutomationAction;
  index: number;
  devices: DeviceView[];
  actionParamDrafts: Record<number, string>;
  onChange: Props['onChange'];
  onParamDraftChange: Props['onParamDraftChange'];
  onApplyTemplate: Props['onApplyTemplate'];
}) {
  const actionDevice = findDevice(props.devices, props.action.device_id);
  const templates = buildActionTemplates(actionDevice);
  return (
    <>
      <div className="automation-field-grid">
        <div className="automation-field">
          <label>Device</label>
          <select className="select" value={props.action.device_id} onChange={(event) => updateAction(props.onChange, props.index, (current) => ({ ...current, device_id: event.target.value }))}>
            {props.devices.map((device) => (
              <option key={device.device.id} value={device.device.id}>
                {device.device.name}
              </option>
            ))}
          </select>
        </div>
        <div className="automation-field">
          <label>Behavior</label>
          <select className="select" value="" onChange={(event) => props.onApplyTemplate(props.index, templates.find((item) => item.key === event.target.value) ?? null)}>
            <option value="">Manual action / keep current</option>
            {templates.map((template) => (
              <option key={template.key} value={template.key}>
                {template.label}
              </option>
            ))}
          </select>
        </div>
      </div>
      <div className="automation-field-grid">
        <div className="automation-field">
          <label>Action</label>
          <Input value={props.action.action} onChange={(event) => updateAction(props.onChange, props.index, (current) => ({ ...current, action: event.target.value }))} placeholder="push_voice_message" />
        </div>
      </div>
      <div className="automation-field">
        <label>Params JSON</label>
        <Textarea value={props.actionParamDrafts[props.index] ?? prettyActionParams(props.action.params)} onChange={(event) => props.onParamDraftChange(props.index, event.target.value)} rows={6} />
      </div>
    </>
  );
}

function AgentActionEditor(props: { action: AutomationAction; index: number; devices: DeviceView[]; onChange: Props['onChange'] }) {
  const params = props.action.params ?? {};
  const touchpoints = actionTouchpoints(props.action);
  return (
    <>
      <div className="automation-field-grid">
        <div className="automation-field">
          <label>Session ID</label>
          <Input value={textParam(params.session_id)} onChange={(event) => updateActionParam(props.onChange, props.index, 'session_id', event.target.value)} placeholder="automation default" />
        </div>
      </div>
      <div className="automation-field">
        <label>Agent Input</label>
        <Textarea value={textParam(params.input)} onChange={(event) => updateActionParam(props.onChange, props.index, 'input', event.target.value)} rows={4} placeholder="/market close or Generate daily digest" />
      </div>
      <div className="automation-rule-list">
        <div className="button-row">
          <Button variant="secondary" size="sm" onClick={() => updateTouchpoints(props.onChange, props.index, [...touchpoints, { type: 'wecom', to_user: '' }])}>
            Add WeCom Touchpoint
          </Button>
          <Button variant="secondary" size="sm" onClick={() => updateTouchpoints(props.onChange, props.index, [...touchpoints, { type: 'device', device_id: defaultVoiceDevice(props.devices), action: 'push_voice_message', params: {} }])}>
            Add Voice Device
          </Button>
        </div>
        {touchpoints.map((touchpoint, touchpointIndex) => (
          <TouchpointEditor
            key={`${touchpoint.type}-${touchpointIndex}`}
            touchpoint={touchpoint}
            devices={props.devices}
            onChange={(next) => updateTouchpoints(props.onChange, props.index, touchpoints.map((item, itemIndex) => (itemIndex === touchpointIndex ? next : item)))}
            onRemove={() => updateTouchpoints(props.onChange, props.index, touchpoints.filter((_, itemIndex) => itemIndex !== touchpointIndex))}
          />
        ))}
      </div>
    </>
  );
}

function TouchpointEditor(props: { touchpoint: TouchpointDraft; devices: DeviceView[]; onChange: (next: TouchpointDraft) => void; onRemove: () => void }) {
  const type = props.touchpoint.type === 'device' ? 'device' : 'wecom';
  const voiceDevices = voiceDeviceOptions(props.devices);
  return (
    <div className="automation-rule">
      <div className="automation-rule__header">
        <div className="automation-section__heading">
          <h4 className="automation-rule__title">{type === 'wecom' ? 'WeCom Touchpoint' : 'Voice Device Touchpoint'}</h4>
        </div>
        <Button variant="ghost" size="sm" onClick={props.onRemove}>
          Remove
        </Button>
      </div>
      <div className="automation-field-grid">
        <div className="automation-field">
          <label>Touchpoint</label>
          <select className="select" value={type} onChange={(event) => props.onChange(event.target.value === 'device' ? { type: 'device', device_id: defaultVoiceDevice(props.devices), action: 'push_voice_message', params: {} } : { type: 'wecom', to_user: '' })}>
            <option value="wecom">WeCom message</option>
            <option value="device">Voice device</option>
          </select>
        </div>
        {type === 'wecom' ? (
          <div className="automation-field">
            <label>WeCom User</label>
            <Input value={props.touchpoint.to_user ?? ''} onChange={(event) => props.onChange({ ...props.touchpoint, type: 'wecom', to_user: event.target.value })} placeholder="@all or user id" />
          </div>
        ) : (
          <>
            <div className="automation-field">
              <label>Device</label>
              <select className="select" value={props.touchpoint.device_id ?? ''} onChange={(event) => props.onChange({ ...props.touchpoint, type: 'device', device_id: event.target.value, action: props.touchpoint.action || 'push_voice_message' })}>
                {voiceDevices.map((device) => (
                  <option key={device.device.id} value={device.device.id}>
                    {device.device.name}
                  </option>
                ))}
              </select>
            </div>
            <div className="automation-field">
              <label>Volume</label>
              <Input value={textParam(props.touchpoint.params?.volume)} onChange={(event) => props.onChange({ ...props.touchpoint, params: { ...(props.touchpoint.params ?? {}), volume: numericValue(event.target.value) } })} placeholder="optional" />
            </div>
          </>
        )}
      </div>
    </div>
  );
}

function switchActionKind(
  onChange: Props['onChange'],
  onResetParamDrafts: Props['onResetParamDrafts'],
  index: number,
  kind: AutomationActionKind,
  devices: DeviceView[],
) {
  onChange((current) => {
    const actions = [...current.actions];
    actions[index] = kind === 'agent' ? createDefaultAgentAction() : defaultDeviceAction(current, devices);
    onResetParamDrafts(actions);
    return { ...current, actions };
  });
}

function updateAction(onChange: Props['onChange'], index: number, updater: (action: AutomationAction) => AutomationAction) {
  onChange((current) => {
    const actions = [...current.actions];
    actions[index] = updater(actions[index]);
    return { ...current, actions };
  });
}

function updateActionParam(onChange: Props['onChange'], index: number, key: string, value: unknown) {
  updateAction(onChange, index, (action) => ({ ...action, kind: 'agent', action: 'agent.run', params: { ...(action.params ?? {}), [key]: value } }));
}

function updateTouchpoints(onChange: Props['onChange'], index: number, touchpoints: TouchpointDraft[]) {
  updateActionParam(onChange, index, 'touchpoints', touchpoints);
}

function defaultDeviceAction(automation: Automation, devices: DeviceView[]): AutomationAction {
  const deviceId = getStateChangedConditionDeviceId(automation) || devices[0]?.device.id || '';
  const device = findDevice(devices, deviceId) ?? devices[0] ?? null;
  const template = buildActionTemplates(device)[0] ?? null;
  return {
    kind: 'device',
    device_id: device?.device.id ?? '',
    label: template?.label ?? '',
    action: template?.action ?? '',
    params: template?.params ?? {},
  };
}

function actionTouchpoints(action: AutomationAction): TouchpointDraft[] {
  const raw = action.params?.touchpoints ?? action.params?.output_touchpoints;
  if (!Array.isArray(raw)) {
    return [];
  }
  return raw.filter((item): item is TouchpointDraft => Boolean(item && typeof item === 'object' && !Array.isArray(item)));
}

function voiceDeviceOptions(devices: DeviceView[]) {
  const candidates = devices.filter((device) => device.device.kind === 'speaker' || device.device.capabilities.includes('voice_push'));
  return candidates.length > 0 ? candidates : devices;
}

function defaultVoiceDevice(devices: DeviceView[]) {
  return voiceDeviceOptions(devices)[0]?.device.id ?? '';
}

function textParam(value: unknown) {
  if (typeof value === 'number' && Number.isFinite(value)) return String(value);
  return typeof value === 'string' ? value : '';
}

function numericValue(value: string) {
  if (!value.trim()) return undefined;
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : value;
}
