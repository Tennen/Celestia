import { cn } from '../../lib/utils';

type Props = {
  checked: boolean;
  pending?: boolean;
  unknown?: boolean;
  disabled?: boolean;
  label: string;
  onChange: (checked: boolean) => void;
};

export function ToggleSwitch({
  checked,
  pending = false,
  unknown = false,
  disabled = false,
  label,
  onChange,
}: Props) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      className={cn(
        'toggle-switch',
        checked && 'is-checked',
        pending && 'is-pending',
        unknown && 'is-unknown',
      )}
      disabled={disabled}
      onClick={() => onChange(!checked)}
    >
      <span className="toggle-switch__track">
        <span className="toggle-switch__thumb" />
      </span>
    </button>
  );
}
