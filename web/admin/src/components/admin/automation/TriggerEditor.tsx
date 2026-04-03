import { Input } from '../../ui/input';
import {
  buildStateKeyOptions,
  findDevice,
  formatStateValueInput,
  operatorNeedsValue,
  parseStateValueInput,
  stateOperators,
  triggerFromOperators,
} from '../../../lib/automation';
import type { Automation, AutomationMatchOperator, DeviceView } from '../../../lib/types';

type Props = {
  draft: Automation;
  devices: DeviceView[];
  onChange: (updater: (current: Automation) => Automation) => void;
};

export function TriggerEditor({ draft, devices, onChange }: Props) {
  const selectedDevice = findDevice(devices, draft.trigger.device_id);

  return (
    <div className="config-field-list__item">
      <strong>Trigger</strong>
      <div className="grid grid--two">
        <div className="stack">
          <label>Device</label>
          <select
            className="select"
            value={draft.trigger.device_id}
            onChange={(e) =>
              onChange((current) => {
                const device = findDevice(devices, e.target.value);
                const stateKey = buildStateKeyOptions(device)[0]?.value ?? '';
                const value = stateKey ? device?.state.state?.[stateKey] : '';
                return {
                  ...current,
                  trigger: {
                    device_id: e.target.value,
                    state_key: stateKey,
                    from: { operator: 'not_equals', value },
                    to: { operator: 'equals', value },
                  },
                };
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
          <label>State Key</label>
          <select
            className="select"
            value={draft.trigger.state_key}
            onChange={(e) =>
              onChange((current) => {
                const device = findDevice(devices, current.trigger.device_id);
                const value = device?.state.state?.[e.target.value];
                return {
                  ...current,
                  trigger: {
                    ...current.trigger,
                    state_key: e.target.value,
                    from: { ...current.trigger.from, value },
                    to: { ...current.trigger.to, value },
                  },
                };
              })
            }
          >
            {buildStateKeyOptions(selectedDevice).map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </div>
      </div>
      <div className="grid grid--two">
        <div className="config-field-list__item">
          <strong>From</strong>
          <select
            className="select"
            value={draft.trigger.from.operator}
            onChange={(e) =>
              onChange((current) => ({
                ...current,
                trigger: { ...current.trigger, from: { ...current.trigger.from, operator: e.target.value as AutomationMatchOperator } },
              }))
            }
          >
            {triggerFromOperators.map((operator) => (
              <option key={operator} value={operator}>
                {operator}
              </option>
            ))}
          </select>
          {operatorNeedsValue(draft.trigger.from.operator) ? (
            <Input
              value={formatStateValueInput(draft.trigger.from.value)}
              onChange={(e) =>
                onChange((current) => ({
                  ...current,
                  trigger: { ...current.trigger, from: { ...current.trigger.from, value: parseStateValueInput(e.target.value) } },
                }))
              }
              placeholder="ready"
            />
          ) : null}
        </div>
        <div className="config-field-list__item">
          <strong>To</strong>
          <select
            className="select"
            value={draft.trigger.to.operator}
            onChange={(e) =>
              onChange((current) => ({
                ...current,
                trigger: { ...current.trigger, to: { ...current.trigger.to, operator: e.target.value as AutomationMatchOperator } },
              }))
            }
          >
            {stateOperators.map((operator) => (
              <option key={operator} value={operator}>
                {operator}
              </option>
            ))}
          </select>
          {operatorNeedsValue(draft.trigger.to.operator) ? (
            <Input
              value={formatStateValueInput(draft.trigger.to.value)}
              onChange={(e) =>
                onChange((current) => ({
                  ...current,
                  trigger: { ...current.trigger, to: { ...current.trigger.to, value: parseStateValueInput(e.target.value) } },
                }))
              }
              placeholder="ready"
            />
          ) : null}
        </div>
      </div>
    </div>
  );
}
