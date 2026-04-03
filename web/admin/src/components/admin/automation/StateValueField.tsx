import { Input } from '../../ui/input';
import {
  buildStateValueOptions,
  coerceStateOptionValue,
  formatStateValueInput,
  operatorNeedsValue,
  parseStateValueInput,
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

  const options = buildStateValueOptions(device, stateKey);
  if (options.length > 0) {
    return (
      <select
        className="select"
        value={String(value ?? '')}
        onChange={(e) => onChange(coerceStateOptionValue(device, stateKey, e.target.value))}
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
      onChange={(e) => onChange(parseStateValueInput(e.target.value))}
      placeholder={placeholder}
    />
  );
}
