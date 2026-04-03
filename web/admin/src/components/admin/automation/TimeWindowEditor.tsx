import { Button } from '../../ui/button';
import { Input } from '../../ui/input';
import type { Automation } from '../../../lib/types';

type Props = {
  draft: Automation;
  onChange: (updater: (current: Automation) => Automation) => void;
};

export function TimeWindowEditor({ draft, onChange }: Props) {
  return (
    <div className="config-field-list__item">
      <div className="section-title">
        <strong>Time Window</strong>
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
      </div>
      {draft.time_window ? (
        <div className="grid grid--two">
          <div className="stack">
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
          <div className="stack">
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
        <p className="muted">Runs all day when no time window is configured.</p>
      )}
    </div>
  );
}
