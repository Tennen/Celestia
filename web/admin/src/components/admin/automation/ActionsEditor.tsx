import { Button } from '../../ui/button';
import { Input } from '../../ui/input';
import { Textarea } from '../../ui/textarea';
import {
  buildActionTemplates,
  findDevice,
  getStateChangedConditionDeviceId,
  prettyActionParams,
  type AutomationActionTemplate,
} from '../../../lib/automation';
import type { Automation, DeviceView } from '../../../lib/types';
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
      description="Choose the commands Core should execute when the rule is satisfied."
      action={
        <Button
          variant="secondary"
          size="sm"
          onClick={() =>
            onChange((current) => {
              const actions = [
                ...current.actions,
                {
                  device_id: current.actions[0]?.device_id || getStateChangedConditionDeviceId(current),
                  label: '',
                  action: '',
                  params: {},
                },
              ];
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
          const actionDevice = findDevice(devices, action.device_id);
          const templates = buildActionTemplates(actionDevice);
          return (
            <div key={`${action.device_id}-${index}`} className="automation-rule">
              <div className="automation-rule__header">
                <div className="automation-section__heading">
                  <h4 className="automation-rule__title">Action {index + 1}</h4>
                  <p className="muted">Dispatch a command to an existing device.</p>
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
                  <label>Device</label>
                  <select
                    className="select"
                    value={action.device_id}
                    onChange={(e) =>
                      onChange((current) => {
                        const actions = [...current.actions];
                        actions[index] = { ...actions[index], device_id: e.target.value };
                        return { ...current, actions };
                      })
                    }
                  >
                    {devices.map((device) => (
                      <option key={device.device.id} value={device.device.id}>
                        {device.device.name}
                      </option>
                    ))}
                  </select>
                </div>
                <div className="automation-field">
                  <label>Behavior</label>
                  <select
                    className="select"
                    value=""
                    onChange={(e) => onApplyTemplate(index, templates.find((item) => item.key === e.target.value) ?? null)}
                  >
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
                  <Input
                    value={action.action}
                    onChange={(e) =>
                      onChange((current) => {
                        const actions = [...current.actions];
                        actions[index] = { ...actions[index], action: e.target.value };
                        return { ...current, actions };
                      })
                    }
                    placeholder="push_voice_message"
                  />
                </div>
                <div className="automation-field">
                  <label>Label</label>
                  <Input
                    value={action.label ?? ''}
                    onChange={(e) =>
                      onChange((current) => {
                        const actions = [...current.actions];
                        actions[index] = { ...actions[index], label: e.target.value };
                        return { ...current, actions };
                      })
                    }
                    placeholder="Suggested · Voice push"
                  />
                </div>
                </div>
                <div className="automation-field">
                <label>Params JSON</label>
                <Textarea
                  value={actionParamDrafts[index] ?? prettyActionParams(action.params)}
                  onChange={(e) => onParamDraftChange(index, e.target.value)}
                  rows={6}
                />
              </div>
              </div>
            </div>
          );
        })}
      </div>
    </AutomationSection>
  );
}
