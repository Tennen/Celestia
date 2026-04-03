import { Button } from '../../ui/button';
import { Input } from '../../ui/input';
import {
  buildStateKeyOptions,
  findDevice,
  formatStateValueInput,
  operatorNeedsValue,
  parseStateValueInput,
  stateOperators,
} from '../../../lib/automation';
import type { Automation, AutomationMatchOperator, DeviceView } from '../../../lib/types';

type Props = {
  draft: Automation;
  devices: DeviceView[];
  onChange: (updater: (current: Automation) => Automation) => void;
};

export function ConditionsEditor({ draft, devices, onChange }: Props) {
  return (
    <div className="config-field-list__item">
      <div className="section-title">
        <strong>Conditions</strong>
        <Button
          variant="secondary"
          size="sm"
          onClick={() =>
            onChange((current) => ({
              ...current,
              conditions: [
                ...(current.conditions ?? []),
                {
                  device_id: current.trigger.device_id,
                  state_key: current.trigger.state_key,
                  match: { operator: 'equals', value: current.trigger.to.value },
                },
              ],
            }))
          }
        >
          Add Condition
        </Button>
      </div>
      <div className="stack">
        <label>Condition Logic</label>
        <select
          className="select"
          value={draft.condition_logic}
          onChange={(e) => onChange((current) => ({ ...current, condition_logic: e.target.value as Automation['condition_logic'] }))}
        >
          <option value="all">all</option>
          <option value="any">any</option>
        </select>
      </div>
      <div className="stack">
        {(draft.conditions ?? []).map((condition, index) => {
          const conditionDevice = findDevice(devices, condition.device_id);
          return (
            <div key={`${condition.device_id}-${index}`} className="config-field-list__item">
              <div className="grid grid--two">
                <div className="stack">
                  <label>Device</label>
                  <select
                    className="select"
                    value={condition.device_id}
                    onChange={(e) =>
                      onChange((current) => {
                        const conditions = [...(current.conditions ?? [])];
                        const device = findDevice(devices, e.target.value);
                        const stateKey = buildStateKeyOptions(device)[0]?.value ?? '';
                        const value = stateKey ? device?.state.state?.[stateKey] : '';
                        conditions[index] = {
                          ...conditions[index],
                          device_id: e.target.value,
                          state_key: stateKey,
                          match: { ...conditions[index].match, value },
                        };
                        return { ...current, conditions };
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
                    value={condition.state_key}
                    onChange={(e) =>
                      onChange((current) => {
                        const conditions = [...(current.conditions ?? [])];
                        conditions[index] = { ...conditions[index], state_key: e.target.value };
                        return { ...current, conditions };
                      })
                    }
                  >
                    {buildStateKeyOptions(conditionDevice).map((option) => (
                      <option key={option.value} value={option.value}>
                        {option.label}
                      </option>
                    ))}
                  </select>
                </div>
              </div>
              <div className="grid grid--two">
                <div className="stack">
                  <label>Operator</label>
                  <select
                    className="select"
                    value={condition.match.operator}
                    onChange={(e) =>
                      onChange((current) => {
                        const conditions = [...(current.conditions ?? [])];
                        conditions[index] = {
                          ...conditions[index],
                          match: { ...conditions[index].match, operator: e.target.value as AutomationMatchOperator },
                        };
                        return { ...current, conditions };
                      })
                    }
                  >
                    {stateOperators.map((operator) => (
                      <option key={operator} value={operator}>
                        {operator}
                      </option>
                    ))}
                  </select>
                </div>
                <div className="stack">
                  <label>Value</label>
                  <Input
                    value={formatStateValueInput(condition.match.value)}
                    onChange={(e) =>
                      onChange((current) => {
                        const conditions = [...(current.conditions ?? [])];
                        conditions[index] = {
                          ...conditions[index],
                          match: { ...conditions[index].match, value: parseStateValueInput(e.target.value) },
                        };
                        return { ...current, conditions };
                      })
                    }
                    placeholder="optional"
                    disabled={!operatorNeedsValue(condition.match.operator)}
                  />
                </div>
              </div>
              <div className="button-row">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() =>
                    onChange((current) => ({
                      ...current,
                      conditions: (current.conditions ?? []).filter((_, itemIndex) => itemIndex !== index),
                    }))
                  }
                >
                  Remove
                </Button>
              </div>
            </div>
          );
        })}
        {(draft.conditions ?? []).length === 0 ? <p className="muted">No extra conditions. Trigger alone will fire the actions.</p> : null}
      </div>
    </div>
  );
}
