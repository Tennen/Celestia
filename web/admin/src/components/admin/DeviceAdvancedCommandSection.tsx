import * as Collapsible from '@radix-ui/react-collapsible';
import { useEffect, useState } from 'react';
import { ChevronDown } from 'lucide-react';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { Textarea } from '../ui/textarea';

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
        <Collapsible.Root
          open={!advancedCommandCollapsed}
          onOpenChange={(open) => setAdvancedCommandCollapsed(!open)}
        >
          <Collapsible.Trigger asChild>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="collapse-toggle"
              aria-controls="advanced-command-panel"
            >
              <span>{advancedCommandCollapsed ? 'Show' : 'Hide'}</span>
              <ChevronDown
                className={`collapse-toggle__icon ${!advancedCommandCollapsed ? 'is-expanded' : ''}`}
              />
            </Button>
          </Collapsible.Trigger>
          <Collapsible.Content id="advanced-command-panel" className="pt-4">
            <div className="stack">
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
                  <Input
                    value={selectedAction}
                    onChange={(event) => onSelectedActionChange(event.target.value)}
                  />
                </div>
                <div>
                  <label>Actor</label>
                  <Input value={actor} onChange={(event) => onActorChange(event.target.value)} />
                </div>
                <div className="grid__full">
                  <label>Params JSON</label>
                  <Textarea
                    rows={6}
                    value={commandParams}
                    onChange={(event) => onCommandParamsChange(event.target.value)}
                  />
                </div>
              </div>
              <div className="button-row">
                <Button onClick={onSendCommand}>Send Advanced Command</Button>
              </div>
            </div>
          </Collapsible.Content>
        </Collapsible.Root>
      </div>
    </div>
  );
}
