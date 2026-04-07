import { Button } from '../../ui/button';
import {
  buildStateKeyOptions,
  countStateChangedConditions,
  coerceMatchValueForOperator,
  createDefaultCondition,
  findDevice,
  getConditionType,
  getStateChangedConditionDeviceId,
  stateOperators,
  transitionFromOperators,
} from '../../../lib/automation';
import type {
  Automation,
  AutomationCondition,
  AutomationConditionType,
  AutomationMatchOperator,
  AutomationStateMatch,
  DeviceView,
} from '../../../lib/types';
import { AutomationSection } from './AutomationSection';
import { StateValueField } from './StateValueField';

type Props = {
  draft: Automation;
  devices: DeviceView[];
  onChange: (updater: (current: Automation) => Automation) => void;
};

function stateValue(device: DeviceView | null, stateKey: string) {
  return stateKey ? device?.state.state?.[stateKey] : '';
}

function stateChangedFromValue(condition: AutomationCondition, fallback: unknown): AutomationStateMatch {
  const value = condition.from?.value ?? fallback;
  return {
    operator: condition.from?.operator ?? 'not_equals',
    value: coerceMatchValueForOperator(condition.from?.operator ?? 'not_equals', value),
  };
}

function stateChangedToValue(condition: AutomationCondition, fallback: unknown): AutomationStateMatch {
  const operator = condition.to?.operator ?? condition.match?.operator ?? 'equals';
  const value = condition.to?.value ?? condition.match?.value ?? fallback;
  return {
    operator,
    value: coerceMatchValueForOperator(operator, value),
  };
}

function currentStateMatchValue(condition: AutomationCondition, fallback: unknown): AutomationStateMatch {
  const operator = condition.match?.operator ?? condition.to?.operator ?? 'equals';
  const value = condition.match?.value ?? condition.to?.value ?? fallback;
  return {
    operator,
    value: coerceMatchValueForOperator(operator, value),
  };
}

function withConditionTarget(
  condition: AutomationCondition,
  devices: DeviceView[],
  deviceId: string,
  stateKey?: string,
): AutomationCondition {
  const device = findDevice(devices, deviceId);
  const nextStateKey = stateKey ?? buildStateKeyOptions(device)[0]?.value ?? '';
  const fallback = stateValue(device, nextStateKey);
  if (getConditionType(condition) === 'state_changed') {
    return {
      ...condition,
      device_id: deviceId,
      state_key: nextStateKey,
      from: stateChangedFromValue(condition, fallback),
      to: stateChangedToValue(condition, fallback),
      match: undefined,
    };
  }
  return {
    ...condition,
    device_id: deviceId,
    state_key: nextStateKey,
    match: currentStateMatchValue(condition, fallback),
    from: undefined,
    to: undefined,
  };
}

function withConditionType(
  condition: AutomationCondition,
  devices: DeviceView[],
  type: AutomationConditionType,
): AutomationCondition {
  const device = findDevice(devices, condition.device_id);
  const nextStateKey = condition.state_key || buildStateKeyOptions(device)[0]?.value || '';
  const fallback = stateValue(device, nextStateKey);
  if (type === 'state_changed') {
    return {
      type,
      device_id: condition.device_id,
      state_key: nextStateKey,
      from: stateChangedFromValue(condition, fallback),
      to: stateChangedToValue(condition, fallback),
    };
  }
  return {
    type,
    device_id: condition.device_id,
    state_key: nextStateKey,
    match: currentStateMatchValue(condition, fallback),
  };
}

function updateCondition(
  onChange: Props['onChange'],
  index: number,
  updater: (condition: AutomationCondition) => AutomationCondition,
) {
  onChange((current) => {
    const conditions = [...(current.conditions ?? [])];
    conditions[index] = updater(conditions[index]);
    return { ...current, conditions };
  });
}

export function ConditionsEditor({ draft, devices, onChange }: Props) {
  const stateChangedCount = countStateChangedConditions(draft.conditions);

  return (
    <AutomationSection
      title="Conditions"
      description="Exactly one State Changed condition starts the automation. Current State Is conditions are optional runtime gates combined by Condition Logic."
      action={
        <Button
          variant="secondary"
          size="sm"
          onClick={() =>
            onChange((current) => ({
              ...current,
              conditions: [
                ...(current.conditions ?? []),
                createDefaultCondition(devices, {
                  deviceId: getStateChangedConditionDeviceId(current),
                  type: countStateChangedConditions(current.conditions) === 0 ? 'state_changed' : 'current_state',
                }),
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
          <option value="all">all current-state conditions</option>
          <option value="any">any current-state condition</option>
        </select>
      </div>
      <div className="automation-rule-list">
        {(draft.conditions ?? []).map((condition, index) => {
          const type = getConditionType(condition);
          const conditionDevice = findDevice(devices, condition.device_id);
          const isOnlyStateChangedCondition = type === 'state_changed' && stateChangedCount <= 1;
          const isRequiredStateChangedCondition = type === 'state_changed' && stateChangedCount === 1;
          const canSwitchToStateChanged = type === 'state_changed' || stateChangedCount === 0;
          return (
            <div key={`${condition.device_id}-${index}`} className="automation-rule">
              <div className="automation-rule__header">
                <div className="automation-section__heading">
                  <div className="automation-rule__title-row">
                    <h4 className="automation-rule__title">Condition {index + 1}</h4>
                    {isRequiredStateChangedCondition ? <span className="automation-rule__required">Required trigger</span> : null}
                  </div>
                  <p className="muted">
                    {type === 'state_changed'
                      ? 'Start when the selected device state changes from one value to another.'
                      : 'Require another device state to match before actions run.'}
                  </p>
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
                  disabled={(draft.conditions ?? []).length <= 1 || isOnlyStateChangedCondition}
                >
                  Remove
                </Button>
              </div>

              <div className="automation-rule__body">
                <div className="automation-field-grid">
                  <div className="automation-field">
                    <label>Type</label>
                    <select
                      className="select"
                      value={type}
                      onChange={(e) =>
                        updateCondition(onChange, index, (current) =>
                          withConditionType(current, devices, e.target.value as AutomationConditionType),
                        )
                      }
                    >
                      <option value="state_changed" disabled={!canSwitchToStateChanged}>
                        State Changed
                      </option>
                      <option value="current_state" disabled={isOnlyStateChangedCondition}>
                        Current State Is
                      </option>
                    </select>
                  </div>
                </div>

                <div className="automation-field-grid">
                  <div className="automation-field">
                    <label>Device</label>
                    <select
                      className="select"
                      value={condition.device_id}
                      onChange={(e) =>
                        updateCondition(onChange, index, (current) =>
                          withConditionTarget(current, devices, e.target.value),
                        )
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
                        updateCondition(onChange, index, (current) =>
                          withConditionTarget(current, devices, current.device_id, e.target.value),
                        )
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

                {type === 'state_changed' ? (
                  <div className="automation-match-grid">
                    <div className="automation-match-card">
                      <h4 className="automation-match-card__title">From</h4>
                      <div className="automation-match-card__body">
                        <div className="automation-field">
                          <label>Operator</label>
                          <select
                            className="select"
                            value={condition.from?.operator ?? 'not_equals'}
                            onChange={(e) =>
                              updateCondition(onChange, index, (current) => ({
                                ...current,
                                from: {
                                  operator: e.target.value as AutomationMatchOperator,
                                  value: coerceMatchValueForOperator(
                                    e.target.value as AutomationMatchOperator,
                                    current.from?.value,
                                  ),
                                },
                              }))
                            }
                          >
                            {transitionFromOperators.map((operator) => (
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
                            operator={condition.from?.operator ?? 'not_equals'}
                            value={condition.from?.value}
                            placeholder="previous value"
                            onChange={(value) =>
                              updateCondition(onChange, index, (current) => ({
                                ...current,
                                from: {
                                  operator: current.from?.operator ?? 'not_equals',
                                  value,
                                },
                              }))
                            }
                          />
                        </div>
                      </div>
                    </div>

                    <div className="automation-match-card">
                      <h4 className="automation-match-card__title">To</h4>
                      <div className="automation-match-card__body">
                        <div className="automation-field">
                          <label>Operator</label>
                          <select
                            className="select"
                            value={condition.to?.operator ?? 'equals'}
                            onChange={(e) =>
                              updateCondition(onChange, index, (current) => ({
                                ...current,
                                to: {
                                  operator: e.target.value as AutomationMatchOperator,
                                  value: coerceMatchValueForOperator(
                                    e.target.value as AutomationMatchOperator,
                                    current.to?.value,
                                  ),
                                },
                              }))
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
                            operator={condition.to?.operator ?? 'equals'}
                            value={condition.to?.value}
                            placeholder="new value"
                            onChange={(value) =>
                              updateCondition(onChange, index, (current) => ({
                                ...current,
                                to: {
                                  operator: current.to?.operator ?? 'equals',
                                  value,
                                },
                              }))
                            }
                          />
                        </div>
                      </div>
                    </div>
                  </div>
                ) : (
                  <div className="automation-field-grid">
                    <div className="automation-field">
                      <label>Operator</label>
                      <select
                        className="select"
                        value={condition.match?.operator ?? 'equals'}
                        onChange={(e) =>
                          updateCondition(onChange, index, (current) => ({
                            ...current,
                            match: {
                              operator: e.target.value as AutomationMatchOperator,
                              value: coerceMatchValueForOperator(
                                e.target.value as AutomationMatchOperator,
                                current.match?.value,
                              ),
                            },
                          }))
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
                        operator={condition.match?.operator ?? 'equals'}
                        value={condition.match?.value}
                        placeholder="current value"
                        onChange={(value) =>
                          updateCondition(onChange, index, (current) => ({
                            ...current,
                            match: {
                              operator: current.match?.operator ?? 'equals',
                              value,
                            },
                          }))
                        }
                      />
                    </div>
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </AutomationSection>
  );
}
