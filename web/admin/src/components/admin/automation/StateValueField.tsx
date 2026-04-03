import { Input } from '../../ui/input';
import {
  buildStateValueOptions,
  coerceStateOptionValue,
  coerceStateOptionValues,
  formatStateValueInput,
  operatorAllowsMultipleValues,
  operatorNeedsValue,
  parseStateValueInput,
  parseStateValuesInput,
} from '../../../lib/automation';
import type { AutomationMatchOperator, DeviceView } from '../../../lib/types';
import { cn } from '../../../lib/utils';

type Props = {
  device: DeviceView | null;
  stateKey: string;
  operator: AutomationMatchOperator;
  value: unknown;
  placeholder?: string;
  onChange: (value: unknown) => void;
};

export function StateValueField({ device, stateKey, operator, value, placeholder, onChange }: Props) {
  if (!operatorNeedsValue(operator)) {
    return <Input value="" placeholder={placeholder} disabled />;
  }

  const multiple = operatorAllowsMultipleValues(operator);
  const options = buildStateValueOptions(device, stateKey);
  const selectedValues = Array.isArray(value) ? value.map((item) => String(item)) : [];
  if (options.length > 0) {
    if (multiple) {
      return (
        <div className="automation-choice-list">
          {options.map((option) => {
            const checked = selectedValues.includes(option.value);
            return (
              <button
                key={`${stateKey}-${option.value}`}
                type="button"
                className={cn('automation-choice', checked && 'is-selected')}
                onClick={() => {
                  const nextValues = checked
                    ? selectedValues.filter((item) => item !== option.value)
                    : [...selectedValues, option.value];
                  onChange(coerceStateOptionValues(device, stateKey, nextValues));
                }}
              >
                <span className={cn('automation-choice__dot', checked && 'is-selected')} />
                <span className="automation-choice__label">{option.label}</span>
              </button>
            );
          })}
        </div>
      );
    }

    return (
      <select
        className="select"
        value={String(value ?? '')}
        onChange={(e) =>
          onChange(
            coerceStateOptionValue(device, stateKey, e.target.value),
          )
        }
      >
        {options.map((option) => (
          <option key={`${stateKey}-${option.value}`} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    );
  }

  return (
    <Input
      value={formatStateValueInput(value)}
      onChange={(e) => onChange(multiple ? parseStateValuesInput(e.target.value) : parseStateValueInput(e.target.value))}
      placeholder={multiple ? 'A, B, C' : placeholder}
    />
  );
}
