import { useEffect, useState } from 'react';
import { RefreshCw } from 'lucide-react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '../ui/card';
import { ScrollArea } from '../ui/scroll-area';
import { agentPanelLabel, type AgentPanelId } from '../../lib/agent-admin';
import { fetchAgentSnapshot, stableJSON, type AgentSnapshot } from '../../lib/agent';
import { AgentContentPanel } from './AgentContentPanel';
import { AgentOpsPanel } from './AgentOpsPanel';
import { AgentRuntimePanel } from './AgentRuntimePanel';
import { AgentWeComPanel } from './AgentWeComPanel';

export type AgentRunner = (label: string, action: () => Promise<unknown>, refresh?: boolean) => void;

type Props = {
  activePanel: AgentPanelId;
};

export function AgentWorkspace({ activePanel }: Props) {
  const [snapshot, setSnapshot] = useState<AgentSnapshot | null>(null);
  const [busy, setBusy] = useState('load');
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [resultText, setResultText] = useState('');

  const syncSnapshot = (next: AgentSnapshot) => {
    setSnapshot(next);
  };

  const load = async () => {
    setBusy('load');
    setError('');
    try {
      syncSnapshot(await fetchAgentSnapshot());
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load agent state');
    } finally {
      setBusy('');
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const run: AgentRunner = async (label, action, refresh = true) => {
    setBusy(label);
    setError('');
    setNotice('');
    try {
      const output = await action();
      setResultText(stableJSON(output));
      setNotice(label.includes('save') ? 'Saved' : 'Done');
      if (refresh) {
        syncSnapshot(await fetchAgentSnapshot());
      } else if (output && typeof output === 'object' && 'settings' in output) {
        syncSnapshot(output as AgentSnapshot);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Request failed');
    } finally {
      setBusy('');
    }
  };

  if (!snapshot) {
    return (
      <Card className="panel">
        <CardContent className="flex items-center gap-2 p-6">
          <RefreshCw className="h-4 w-4 animate-spin" />
          Loading agent runtime
        </CardContent>
      </Card>
    );
  }

  return (
    <ScrollArea className="detail-scroll">
      <div className="detail-stack">
        <div className="section-title">
          <div>
            <p className="eyebrow">Celestia Agent Runtime</p>
            <h2 className="text-2xl font-semibold tracking-tight">{agentPanelLabel(activePanel)}</h2>
          </div>
          <div className="toolbar">
            <Badge tone={snapshot.settings.llm_providers.length ? 'good' : 'warn'}>
              {snapshot.settings.llm_providers.length} LLM
            </Badge>
            <Badge tone={snapshot.settings.wecom.enabled ? 'good' : 'neutral'}>WeCom</Badge>
            <Badge tone={snapshot.settings.md2img?.enabled ? 'good' : 'neutral'}>md2img</Badge>
            <Badge tone={snapshot.settings.terminal.enabled ? 'warn' : 'neutral'}>Terminal</Badge>
            <Button variant="secondary" onClick={() => void load()} disabled={busy === 'load'}>
              <RefreshCw className={`mr-2 h-4 w-4 ${busy === 'load' ? 'animate-spin' : ''}`} />
              Refresh
            </Button>
          </div>
        </div>

        {error ? <div className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm">{error}</div> : null}
        {notice ? <div className="rounded-md border border-primary/20 bg-primary/5 p-3 text-sm">{notice}</div> : null}

        {activePanel === 'runtime' ? <AgentRuntimePanel snapshot={snapshot} busy={busy} onRun={run} /> : null}
        {activePanel === 'wecom' ? <AgentWeComPanel snapshot={snapshot} busy={busy} onRun={run} /> : null}
        {activePanel === 'content' ? <AgentContentPanel snapshot={snapshot} busy={busy} onRun={run} /> : null}
        {activePanel === 'ops' ? <AgentOpsPanel snapshot={snapshot} busy={busy} onRun={run} /> : null}

        {resultText ? (
          <Card className="panel log-panel">
            <CardHeader>
              <CardTitle>Last Result</CardTitle>
            </CardHeader>
            <CardContent>
              <pre className="max-h-72 overflow-auto rounded-md bg-secondary/60 p-3 text-xs">{resultText}</pre>
            </CardContent>
          </Card>
        ) : null}
      </div>
    </ScrollArea>
  );
}
