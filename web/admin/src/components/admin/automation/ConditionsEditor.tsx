import { Button } from '../../ui/button';
import { Input } from '../../ui/input';
import {
  buildStateKeyOptions,
  coerceMatchValueForOperator,
  countTriggerConditions,
  createDefaultCondition,
  createDefaultTimeCondition,
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
  const operator = condition.from?.operator ?? 'not_equals';
  return {
    operator,
    value: coerceMatchValueForOperator(operator, condition.from?.value ?? fallback),
  };
}

function stateChangedToValue(condition: AutomationCondition, fallback: unknown): AutomationStateMatch {
  const operator = condition.to?.operator ?? condition.match?.operator ?? 'equals';
  return {
    operator,
    value: coerceMatchValueForOperator(operator, condition.to?.value ?? condition.match?.value ?? fallback),
  };
}

function currentStateMatchValue(condition: AutomationCondition, fallback: unknown): AutomationStateMatch {
  const operator = condition.match?.operator ?? condition.to?.operator ?? 'equals';
  return {
    operator,
    value: coerceMatchValueForOperator(operator, condition.match?.value ?? condition.to?.value ?? fallback),
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
      time: undefined,
    };
  }
  return {
    ...condition,
    device_id: deviceId,
    state_key: nextStateKey,
    match: currentStateMatchValue(condition, fallback),
    from: undefined,
    to: undefined,
    time: undefined,
  };
}

function withConditionType(
  condition: AutomationCondition,
  devices: DeviceView[],
  type: AutomationConditionType,
): AutomationCondition {
  if (type === 'time') {
    return createDefaultTimeCondition();
  }
  const deviceId = condition.device_id || devices[0]?.device.id || '';
  const device = findDevice(devices, deviceId);
  const nextStateKey = condition.state_key || buildStateKeyOptions(device)[0]?.value || '';
  const fallback = stateValue(device, nextStateKey);
  if (type === 'state_changed') {
    return {
      type,
      device_id: deviceId,
      state_key: nextStateKey,
      from: stateChangedFromValue(condition, fallback),
      to: stateChangedToValue(condition, fallback),
    };
  }
  return {
    type,
    device_id: deviceId,
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
  const triggerCount = countTriggerConditions(draft.conditions);

  return (
    <AutomationSection
      title="Conditions"
      description="Use exactly one trigger: a device State Changed event or a daily Time trigger. Current State Is conditions are optional gates."
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
                  type: countTriggerConditions(current.conditions) === 0 ? 'state_changed' : 'current_state',
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
          <option value="all">all current-state gates</option>
          <option value="any">any current-state gate</option>
        </select>
      </div>
      <div className="automation-rule-list">
        {(draft.conditions ?? []).map((condition, index) => {
          const type = getConditionType(condition);
          const conditionDevice = findDevice(devices, condition.device_id ?? '');
          const isTrigger = type === 'state_changed' || type === 'time';
          const isOnlyTrigger = isTrigger && triggerCount <= 1;
          const canSwitchToTrigger = isTrigger || triggerCount === 0;
          return (
            <div key={`${type}-${condition.device_id ?? 'time'}-${index}`} className="automation-rule">
              <div className="automation-rule__header">
                <div className="automation-section__heading">
                  <div className="automation-rule__title-row">
                    <h4 className="automation-rule__title">Condition {index + 1}</h4>
                    {isOnlyTrigger ? <span className="automation-rule__required">Required trigger</span> : null}
                  </div>
                  <p className="muted">{conditionDescription(type)}</p>
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
                  disabled={(draft.conditions ?? []).length <= 1 || isOnlyTrigger}
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
                      <option value="state_changed" disabled={!canSwitchToTrigger}>
                        State Changed
                      </option>
                      <option value="time" disabled={!canSwitchToTrigger}>
                        Daily Time
                      </option>
                      <option value="current_state" disabled={isOnlyTrigger}>
                        Current State Is
                      </option>
                    </select>
                  </div>
                </div>

                {type === 'time' ? (
                  <TimeConditionFields condition={condition} onChange={(next) => updateCondition(onChange, index, () => next)} />
                ) : (
                  <>
                    <div className="automation-field-grid">
                      <div className="automation-field">
                        <label>Device</label>
                        <select
                          className="select"
                          value={condition.device_id ?? ''}
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
                          value={condition.state_key ?? ''}
                          onChange={(e) =>
                            updateCondition(onChange, index, (current) =>
                              withConditionTarget(current, devices, current.device_id ?? '', e.target.value),
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
                      <StateChangedFields condition={condition} conditionDevice={conditionDevice} onChange={(next) => updateCondition(onChange, index, () => next)} />
                    ) : (
                      <CurrentStateFields condition={condition} conditionDevice={conditionDevice} onChange={(next) => updateCondition(onChange, index, () => next)} />
                    )}
                  </>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </AutomationSection>
  );
}

function TimeConditionFields(props: { condition: AutomationCondition; onChange: (next: AutomationCondition) => void }) {
  const time = props.condition.time ?? createDefaultTimeCondition().time!;
  return (
    <div className="automation-field-grid">
      <div className="automation-field">
        <label>Schedule</label>
        <select className="select" value={time.schedule || 'daily'} onChange={(event) => props.onChange({ type: 'time', time: { ...time, schedule: event.target.value } })}>
          <option value="daily">Daily</option>
        </select>
      </div>
      <div className="automation-field">
        <label>At</label>
        <Input type="time" value={time.at || '08:00'} onChange={(event) => props.onChange({ type: 'time', time: { ...time, at: event.target.value } })} />
      </div>
      <div className="automation-field">
        <label>Timezone</label>
        <Input value={time.timezone ?? ''} onChange={(event) => props.onChange({ type: 'time', time: { ...time, timezone: event.target.value } })} placeholder="Asia/Shanghai" />
      </div>
    </div>
  );
}

function StateChangedFields(props: { condition: AutomationCondition; conditionDevice: DeviceView | null; onChange: (next: AutomationCondition) => void }) {
  const { condition, conditionDevice, onChange } = props;
  return (
    <div className="automation-match-grid">
      <MatchCard
        title="From"
        device={conditionDevice}
        stateKey={condition.state_key ?? ''}
        operator={condition.from?.operator ?? 'not_equals'}
        value={condition.from?.value}
        operators={transitionFromOperators}
        placeholder="previous value"
        onOperatorChange={(operator) => onChange({ ...condition, from: { operator, value: coerceMatchValueForOperator(operator, condition.from?.value) } })}
        onValueChange={(value) => onChange({ ...condition, from: { operator: condition.from?.operator ?? 'not_equals', value } })}
      />
      <MatchCard
        title="To"
        device={conditionDevice}
        stateKey={condition.state_key ?? ''}
        operator={condition.to?.operator ?? 'equals'}
        value={condition.to?.value}
        operators={stateOperators}
        placeholder="new value"
        onOperatorChange={(operator) => onChange({ ...condition, to: { operator, value: coerceMatchValueForOperator(operator, condition.to?.value) } })}
        onValueChange={(value) => onChange({ ...condition, to: { operator: condition.to?.operator ?? 'equals', value } })}
      />
    </div>
  );
}

function CurrentStateFields(props: { condition: AutomationCondition; conditionDevice: DeviceView | null; onChange: (next: AutomationCondition) => void }) {
  const { condition, conditionDevice, onChange } = props;
  return (
    <div className="automation-field-grid">
      <div className="automation-field">
        <label>Operator</label>
        <select className="select" value={condition.match?.operator ?? 'equals'} onChange={(event) => {
          const operator = event.target.value as AutomationMatchOperator;
          onChange({ ...condition, match: { operator, value: coerceMatchValueForOperator(operator, condition.match?.value) } });
        }}>
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
          stateKey={condition.state_key ?? ''}
          operator={condition.match?.operator ?? 'equals'}
          value={condition.match?.value}
          placeholder="current value"
          onChange={(value) => onChange({ ...condition, match: { operator: condition.match?.operator ?? 'equals', value } })}
        />
      </div>
    </div>
  );
}

function MatchCard(props: {
  title: string;
  device: DeviceView | null;
  stateKey: string;
  operator: AutomationMatchOperator;
  value: unknown;
  operators: AutomationMatchOperator[];
  placeholder: string;
  onOperatorChange: (operator: AutomationMatchOperator) => void;
  onValueChange: (value: unknown) => void;
}) {
  return (
    <div className="automation-match-card">
      <h4 className="automation-match-card__title">{props.title}</h4>
      <div className="automation-match-card__body">
        <div className="automation-field">
          <label>Operator</label>
          <select className="select" value={props.operator} onChange={(event) => props.onOperatorChange(event.target.value as AutomationMatchOperator)}>
            {props.operators.map((operator) => (
              <option key={operator} value={operator}>
                {operator}
              </option>
            ))}
          </select>
        </div>
        <div className="automation-field">
          <label>Value</label>
          <StateValueField
            device={props.device}
            stateKey={props.stateKey}
            operator={props.operator}
            value={props.value}
            placeholder={props.placeholder}
            onChange={props.onValueChange}
          />
        </div>
      </div>
    </div>
  );
}

function conditionDescription(type: AutomationConditionType) {
  if (type === 'time') {
    return 'Start once per local day at the configured time.';
  }
  if (type === 'state_changed') {
    return 'Start when the selected device state changes from one value to another.';
  }
  return 'Require another device state to match before actions run.';
}
