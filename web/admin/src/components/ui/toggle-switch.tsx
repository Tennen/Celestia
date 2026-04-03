import { Switch } from './switch';
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
    <Switch
      checked={checked}
      aria-label={label}
      className={cn(
        pending && 'cursor-progress data-[state=checked]:bg-primary/70 data-[state=unchecked]:bg-primary/30',
        unknown && 'data-[state=unchecked]:bg-warning/60',
      )}
      disabled={disabled}
      onCheckedChange={onChange}
    />
  );
}
