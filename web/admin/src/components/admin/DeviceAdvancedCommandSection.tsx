import { useEffect, useState } from 'react';
import { Button } from '../ui/button';
import { Icon } from '../ui/icon';
import { Input } from '../ui/input';
import { Textarea } from '../ui/textarea';
import { cn } from '../../lib/utils';

type CommandSuggestion = {
  label: string;
  action: string;
  params: Record<string, unknown>;
};

type Props = {
  deviceId: string;
  selectedAction: string;
  onSelectedActionChange: (value: string) => void;
  actor: string;
  onActorChange: (value: string) => void;
  commandParams: string;
  onCommandParamsChange: (value: string) => void;
  commandSuggestions: CommandSuggestion[];
  onApplySuggestion: (action: string, params: Record<string, unknown>) => void;
  onSendCommand: () => void;
};

function ChevronIcon({ expanded }: { expanded: boolean }) {
  return (
    <Icon
      size="md"
      className={cn('collapse-toggle__icon', expanded && 'is-expanded')}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="m6 9 6 6 6-6" />
    </Icon>
  );
}

export function DeviceAdvancedCommandSection({
  deviceId,
  selectedAction,
  onSelectedActionChange,
  actor,
  onActorChange,
  commandParams,
  onCommandParamsChange,
  commandSuggestions,
  onApplySuggestion,
  onSendCommand,
}: Props) {
  const [advancedCommandCollapsed, setAdvancedCommandCollapsed] = useState(true);

  useEffect(() => {
    setAdvancedCommandCollapsed(true);
  }, [deviceId]);

  return (
    <div>
      <div className="section-title section-title--inline">
        <label>Advanced Command</label>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="collapse-toggle"
          onClick={() => setAdvancedCommandCollapsed((current) => !current)}
          aria-expanded={!advancedCommandCollapsed}
          aria-controls="advanced-command-panel"
        >
          <span>{advancedCommandCollapsed ? 'Show' : 'Hide'}</span>
          <ChevronIcon expanded={!advancedCommandCollapsed} />
        </Button>
      </div>
      {!advancedCommandCollapsed ? (
        <div id="advanced-command-panel" className="stack">
          <p className="muted">
            Use this only for vendor-specific operations or parameter tuning. Most day-to-day controls are wrapped
            above. Click a preset below to prefill a known command shape before editing.
          </p>
          <div className="button-row">
            {commandSuggestions.map((suggestion) => (
              <Button
                key={suggestion.label}
                variant="secondary"
                onClick={() => onApplySuggestion(suggestion.action, suggestion.params)}
              >
                Prefill {suggestion.label}
              </Button>
            ))}
          </div>
          <div className="grid grid--detail">
            <div>
              <label>Action</label>
              <Input value={selectedAction} onChange={(event) => onSelectedActionChange(event.target.value)} />
            </div>
            <div>
              <label>Actor</label>
              <Input value={actor} onChange={(event) => onActorChange(event.target.value)} />
            </div>
            <div className="grid__full">
              <label>Params JSON</label>
              <Textarea rows={6} value={commandParams} onChange={(event) => onCommandParamsChange(event.target.value)} />
            </div>
          </div>
          <div className="button-row">
            <Button onClick={onSendCommand}>Send Advanced Command</Button>
          </div>
        </div>
      ) : null}
    </div>
  );
}
