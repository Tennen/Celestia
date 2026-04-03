import { Button } from '../../ui/button';
import { Input } from '../../ui/input';
import { Textarea } from '../../ui/textarea';
import { buildActionTemplates, findDevice, prettyActionParams, type AutomationActionTemplate } from '../../../lib/automation';
import type { Automation, DeviceView } from '../../../lib/types';

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
    <div className="config-field-list__item">
      <div className="section-title">
        <strong>Actions</strong>
        <Button
          variant="secondary"
          size="sm"
          onClick={() =>
            onChange((current) => {
              const actions = [
                ...current.actions,
                {
                  device_id: current.actions[0]?.device_id || current.trigger.device_id,
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
      </div>
      <div className="stack">
        {draft.actions.map((action, index) => {
          const actionDevice = findDevice(devices, action.device_id);
          const templates = buildActionTemplates(actionDevice);
          return (
            <div key={`${action.device_id}-${index}`} className="config-field-list__item">
              <div className="grid grid--two">
                <div className="stack">
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
                <div className="stack">
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
              <div className="grid grid--two">
                <div className="stack">
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
                <div className="stack">
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
              <div className="stack">
                <label>Params JSON</label>
                <Textarea
                  value={actionParamDrafts[index] ?? prettyActionParams(action.params)}
                  onChange={(e) => onParamDraftChange(index, e.target.value)}
                  rows={6}
                />
              </div>
              <div className="button-row">
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
            </div>
          );
        })}
      </div>
    </div>
  );
}
