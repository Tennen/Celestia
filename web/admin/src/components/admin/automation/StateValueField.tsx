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
  if (options.length > 0) {
    return (
      <select
        className="select"
        value={multiple ? (Array.isArray(value) ? value.map((item) => String(item)) : []) : String(value ?? '')}
        multiple={multiple}
        size={multiple ? Math.min(Math.max(options.length, 3), 6) : undefined}
        onChange={(e) =>
          onChange(
            multiple
              ? coerceStateOptionValues(
                  device,
                  stateKey,
                  Array.from(e.currentTarget.selectedOptions).map((option) => option.value),
                )
              : coerceStateOptionValue(device, stateKey, e.target.value),
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
