import { Button } from '../../ui/button';
import {
  buildStateKeyOptions,
  coerceMatchValueForOperator,
  findDevice,
  stateOperators,
} from '../../../lib/automation';
import type { Automation, AutomationMatchOperator, DeviceView } from '../../../lib/types';
import { AutomationSection } from './AutomationSection';
import { StateValueField } from './StateValueField';

type Props = {
  draft: Automation;
  devices: DeviceView[];
  onChange: (updater: (current: Automation) => Automation) => void;
};

export function ConditionsEditor({ draft, devices, onChange }: Props) {
  return (
    <AutomationSection
      title="Conditions"
      description="Add optional checks that must pass before the automation runs."
      action={
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
                  match: { operator: current.trigger.to.operator, value: current.trigger.to.value },
                },
              ],
            }))
          }
        >
          Add Condition
        </Button>
      }
    >
      <div className="automation-logic-row">
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
      {(draft.conditions ?? []).length > 0 ? (
        <div className="automation-rule-list">
        {(draft.conditions ?? []).map((condition, index) => {
          const conditionDevice = findDevice(devices, condition.device_id);
          return (
            <div key={`${condition.device_id}-${index}`} className="automation-rule">
              <div className="automation-rule__header">
                <div className="automation-section__heading">
                  <h4 className="automation-rule__title">Condition {index + 1}</h4>
                  <p className="muted">Gate the trigger with another device state match.</p>
                </div>
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
              <div className="automation-rule__body">
                <div className="automation-field-grid">
                  <div className="automation-field">
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
                          match: {
                            ...conditions[index].match,
                            value: coerceMatchValueForOperator(conditions[index].match.operator, value),
                          },
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
                <div className="automation-field">
                  <label>State Key</label>
                  <select
                    className="select"
                    value={condition.state_key}
                    onChange={(e) =>
                      onChange((current) => {
                        const conditions = [...(current.conditions ?? [])];
                        const device = findDevice(devices, conditions[index].device_id);
                        const value = device?.state.state?.[e.target.value];
                        conditions[index] = {
                          ...conditions[index],
                          state_key: e.target.value,
                          match: {
                            ...conditions[index].match,
                            value: coerceMatchValueForOperator(conditions[index].match.operator, value),
                          },
                        };
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
                <div className="automation-field-grid">
                  <div className="automation-field">
                  <label>Operator</label>
                  <select
                    className="select"
                    value={condition.match.operator}
                    onChange={(e) =>
                      onChange((current) => {
                        const conditions = [...(current.conditions ?? [])];
                        const operator = e.target.value as AutomationMatchOperator;
                        conditions[index] = {
                          ...conditions[index],
                          match: {
                            ...conditions[index].match,
                            operator,
                            value: coerceMatchValueForOperator(operator, conditions[index].match.value),
                          },
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
                <div className="automation-field">
                  <label>Value</label>
                  <StateValueField
                    device={conditionDevice}
                    stateKey={condition.state_key}
                    operator={condition.match.operator}
                    value={condition.match.value}
                    placeholder="optional"
                    onChange={(value) =>
                      onChange((current) => {
                        const conditions = [...(current.conditions ?? [])];
                        conditions[index] = {
                          ...conditions[index],
                          match: { ...conditions[index].match, value },
                        };
                        return { ...current, conditions };
                      })
                    }
                  />
                </div>
              </div>
              </div>
            </div>
          );
        })}
        </div>
      ) : (
        <div className="automation-empty">
          No extra conditions. Trigger alone will fire the actions.
        </div>
      )}
    </AutomationSection>
  );
}
