import { Button } from '../../ui/button';
import {
  buildStateKeyOptions,
  coerceMatchValueForOperator,
  createDefaultCondition,
  findDevice,
  getConditionKind,
  getConditionScope,
  getPrimaryConditionDeviceId,
  stateOperators,
  transitionFromOperators,
} from '../../../lib/automation';
import type {
  Automation,
  AutomationCondition,
  AutomationConditionKind,
  AutomationConditionScope,
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

function transitionFromValue(condition: AutomationCondition, fallback: unknown): AutomationStateMatch {
  const value = condition.from?.value ?? fallback;
  return {
    operator: condition.from?.operator ?? 'not_equals',
    value: coerceMatchValueForOperator(condition.from?.operator ?? 'not_equals', value),
  };
}

function transitionToValue(condition: AutomationCondition, fallback: unknown): AutomationStateMatch {
  const operator = condition.to?.operator ?? condition.match?.operator ?? 'equals';
  const value = condition.to?.value ?? condition.match?.value ?? fallback;
  return {
    operator,
    value: coerceMatchValueForOperator(operator, value),
  };
}

function matchValue(condition: AutomationCondition, fallback: unknown): AutomationStateMatch {
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
  const kind = getConditionKind(condition);
  if (kind === 'transition') {
    return {
      ...condition,
      device_id: deviceId,
      state_key: nextStateKey,
      from: transitionFromValue(condition, fallback),
      to: transitionToValue(condition, fallback),
      match: undefined,
    };
  }
  return {
    ...condition,
    device_id: deviceId,
    state_key: nextStateKey,
    match: matchValue(condition, fallback),
    from: undefined,
    to: undefined,
  };
}

function withConditionScope(
  condition: AutomationCondition,
  devices: DeviceView[],
  scope: AutomationConditionScope,
): AutomationCondition {
  const device = findDevice(devices, condition.device_id);
  const nextStateKey = condition.state_key || buildStateKeyOptions(device)[0]?.value || '';
  const fallback = stateValue(device, nextStateKey);
  const kind = scope === 'state' ? 'match' : getConditionKind(condition);
  if (kind === 'transition') {
    return {
      scope,
      kind,
      device_id: condition.device_id,
      state_key: nextStateKey,
      from: transitionFromValue(condition, fallback),
      to: transitionToValue(condition, fallback),
    };
  }
  return {
    scope,
    kind: 'match',
    device_id: condition.device_id,
    state_key: nextStateKey,
    match: matchValue(condition, fallback),
  };
}

function withConditionKind(
  condition: AutomationCondition,
  devices: DeviceView[],
  kind: AutomationConditionKind,
): AutomationCondition {
  const device = findDevice(devices, condition.device_id);
  const nextStateKey = condition.state_key || buildStateKeyOptions(device)[0]?.value || '';
  const fallback = stateValue(device, nextStateKey);
  if (kind === 'transition') {
    return {
      scope: 'event',
      kind,
      device_id: condition.device_id,
      state_key: nextStateKey,
      from: transitionFromValue(condition, fallback),
      to: transitionToValue(condition, fallback),
    };
  }
  return {
    scope: getConditionScope(condition),
    kind,
    device_id: condition.device_id,
    state_key: nextStateKey,
    match: matchValue(condition, fallback),
  };
}

function updateCondition(
  draft: Automation,
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
  const eventConditionCount = (draft.conditions ?? []).filter((condition) => getConditionScope(condition) === 'event').length;

  return (
    <AutomationSection
      title="Conditions"
      description="Event conditions trigger the automation when any one matches. State conditions are extra gates combined by Condition Logic."
      action={
        <Button
          variant="secondary"
          size="sm"
          onClick={() =>
            onChange((current) => ({
              ...current,
              conditions: [
                ...(current.conditions ?? []),
                createDefaultCondition(devices, { deviceId: getPrimaryConditionDeviceId(current) }),
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
          <option value="all">all state conditions</option>
          <option value="any">any state condition</option>
        </select>
      </div>
      <div className="automation-rule-list">
        {(draft.conditions ?? []).map((condition, index) => {
          const scope = getConditionScope(condition);
          const kind = getConditionKind(condition);
          const conditionDevice = findDevice(devices, condition.device_id);
          const isOnlyEventCondition = scope === 'event' && eventConditionCount <= 1;
          return (
            <div key={`${condition.device_id}-${index}`} className="automation-rule">
              <div className="automation-rule__header">
                <div className="automation-section__heading">
                  <h4 className="automation-rule__title">Condition {index + 1}</h4>
                  <p className="muted">
                    {scope === 'event'
                      ? kind === 'transition'
                        ? 'Trigger when one device state changes from one value to another.'
                        : 'Trigger when a device state key changes and its new value matches.'
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
                  disabled={(draft.conditions ?? []).length <= 1 || isOnlyEventCondition}
                >
                  Remove
                </Button>
              </div>

              <div className="automation-rule__body">
                <div className="automation-field-grid">
                  <div className="automation-field">
                    <label>Scope</label>
                    <select
                      className="select"
                      value={scope}
                      onChange={(e) =>
                        updateCondition(draft, onChange, index, (current) =>
                          withConditionScope(current, devices, e.target.value as AutomationConditionScope),
                        )
                      }
                    >
                      <option value="event">Trigger Event</option>
                      <option value="state" disabled={isOnlyEventCondition}>
                        State Gate
                      </option>
                    </select>
                  </div>
                  <div className="automation-field">
                    <label>Match Type</label>
                    <select
                      className="select"
                      value={kind}
                      onChange={(e) =>
                        updateCondition(draft, onChange, index, (current) =>
                          withConditionKind(current, devices, e.target.value as AutomationConditionKind),
                        )
                      }
                      disabled={scope === 'state'}
                    >
                      <option value="transition">State Changed</option>
                      <option value="match">State Is</option>
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
                        updateCondition(draft, onChange, index, (current) =>
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
                        updateCondition(draft, onChange, index, (current) =>
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

                {kind === 'transition' ? (
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
                              updateCondition(draft, onChange, index, (current) => ({
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
                              updateCondition(draft, onChange, index, (current) => ({
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
                              updateCondition(draft, onChange, index, (current) => ({
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
                              updateCondition(draft, onChange, index, (current) => ({
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
                          updateCondition(draft, onChange, index, (current) => ({
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
                        placeholder={scope === 'event' ? 'new value' : 'current value'}
                        onChange={(value) =>
                          updateCondition(draft, onChange, index, (current) => ({
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
