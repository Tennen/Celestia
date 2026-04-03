import { Button } from '../../ui/button';
import { Input } from '../../ui/input';
import type { Automation } from '../../../lib/types';
import { AutomationSection } from './AutomationSection';

type Props = {
  draft: Automation;
  onChange: (updater: (current: Automation) => Automation) => void;
};

export function TimeWindowEditor({ draft, onChange }: Props) {
  return (
    <AutomationSection
      title="Time Window"
      description="Optionally limit this automation to a daily time range."
      action={
        <Button
          variant="secondary"
          size="sm"
          onClick={() =>
            onChange((current) => ({
              ...current,
              time_window: current.time_window ? null : { start: '08:00', end: '22:00' },
            }))
          }
        >
          {draft.time_window ? 'Remove Time Window' : 'Add Time Window'}
        </Button>
      }
    >
      {draft.time_window ? (
        <div className="automation-field-grid">
          <div className="automation-field">
            <label>Start</label>
            <Input
              type="time"
              value={draft.time_window.start}
              onChange={(e) =>
                onChange((current) => ({
                  ...current,
                  time_window: { ...(current.time_window ?? { start: '', end: '' }), start: e.target.value },
                }))
              }
            />
          </div>
          <div className="automation-field">
            <label>End</label>
            <Input
              type="time"
              value={draft.time_window.end}
              onChange={(e) =>
                onChange((current) => ({
                  ...current,
                  time_window: { ...(current.time_window ?? { start: '', end: '' }), end: e.target.value },
                }))
              }
            />
          </div>
        </div>
      ) : (
        <div className="automation-empty">Runs all day when no time window is configured.</div>
      )}
    </AutomationSection>
  );
}
