import {
  buildStateKeyOptions,
  coerceMatchValueForOperator,
  findDevice,
  stateOperators,
  triggerFromOperators,
} from '../../../lib/automation';
import type { Automation, AutomationMatchOperator, DeviceView } from '../../../lib/types';
import { StateValueField } from './StateValueField';

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
                    from: { ...current.trigger.from, value: coerceMatchValueForOperator(current.trigger.from.operator, value) },
                    to: { ...current.trigger.to, value: coerceMatchValueForOperator(current.trigger.to.operator, value) },
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
              onChange((current) => {
                const operator = e.target.value as AutomationMatchOperator;
                return {
                  ...current,
                  trigger: {
                    ...current.trigger,
                    from: {
                      ...current.trigger.from,
                      operator,
                      value: coerceMatchValueForOperator(operator, current.trigger.from.value),
                    },
                  },
                };
              })
            }
          >
            {triggerFromOperators.map((operator) => (
              <option key={operator} value={operator}>
                {operator}
              </option>
            ))}
          </select>
          <StateValueField
            device={selectedDevice}
            stateKey={draft.trigger.state_key}
            operator={draft.trigger.from.operator}
            value={draft.trigger.from.value}
            placeholder="ready"
            onChange={(value) =>
              onChange((current) => ({
                ...current,
                trigger: { ...current.trigger, from: { ...current.trigger.from, value } },
              }))
            }
          />
        </div>
        <div className="config-field-list__item">
          <strong>To</strong>
          <select
            className="select"
            value={draft.trigger.to.operator}
            onChange={(e) =>
              onChange((current) => {
                const operator = e.target.value as AutomationMatchOperator;
                return {
                  ...current,
                  trigger: {
                    ...current.trigger,
                    to: {
                      ...current.trigger.to,
                      operator,
                      value: coerceMatchValueForOperator(operator, current.trigger.to.value),
                    },
                  },
                };
              })
            }
          >
            {stateOperators.map((operator) => (
              <option key={operator} value={operator}>
                {operator}
              </option>
            ))}
          </select>
          <StateValueField
            device={selectedDevice}
            stateKey={draft.trigger.state_key}
            operator={draft.trigger.to.operator}
            value={draft.trigger.to.value}
            placeholder="ready"
            onChange={(value) =>
              onChange((current) => ({
                ...current,
                trigger: { ...current.trigger, to: { ...current.trigger.to, value } },
              }))
            }
          />
        </div>
      </div>
    </div>
  );
}
