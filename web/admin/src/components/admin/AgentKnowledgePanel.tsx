import { useEffect, useMemo, useState } from 'react';
import { Plus, Save, Trash2 } from 'lucide-react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { saveAgentSettings, type AgentKnowledgeBase, type AgentSnapshot } from '../../lib/agent';
import { Field, FieldGrid, SelectField, ToggleField, numberValue, parseOptionalNumber } from './AgentFormFields';
import type { AgentRunner } from './AgentWorkspace';
import { SelectableListItem } from './shared/SelectableListItem';

type Props = {
  snapshot: AgentSnapshot;
  busy: string;
  onRun: AgentRunner;
};

export function AgentKnowledgePanel({ snapshot, busy, onRun }: Props) {
  const config = snapshot.settings.knowledge ?? { enabled: false };
  const [enabled, setEnabled] = useState(config.enabled === true);
  const [base, setBase] = useState<AgentKnowledgeBase>((config.bases ?? [])[0] ?? emptyBase());
  const [defaultBaseId, setDefaultBaseId] = useState(config.default_base_id ?? '');
  const [agentProviderId, setAgentProviderId] = useState(config.agent_provider_id ?? '');
  const [timeout, setTimeoutValue] = useState(numberValue(config.timeout_ms));
  const agentProviders = useMemo(
    () => snapshot.settings.agent_providers,
    [snapshot.settings.agent_providers],
  );
  const providerOptions = useMemo(
    () => [
      { value: '', label: 'select agent provider' },
      ...agentProviders.map((provider) => ({
        value: provider.id,
        label: `${provider.name || provider.id}${provider.model ? ` · ${provider.model}` : ''}`,
      })),
    ],
    [agentProviders],
  );

  useEffect(() => {
    const next = snapshot.settings.knowledge ?? { enabled: false };
    setEnabled(next.enabled === true);
    setBase((next.bases ?? [])[0] ?? emptyBase());
    setDefaultBaseId(next.default_base_id ?? '');
    setAgentProviderId(next.agent_provider_id ?? '');
    setTimeoutValue(numberValue(next.timeout_ms));
  }, [snapshot]);

  const save = () => {
    const bases = replaceBase(config.bases ?? [], normalizeBase(base));
    const nextDefault = defaultBaseId || bases[0]?.id || '';
    onRun(
      'settings-save',
      () =>
        saveAgentSettings({
          ...snapshot.settings,
          knowledge: {
            enabled,
            default_base_id: nextDefault || undefined,
            bases,
            agent_provider_id: agentProviderId.trim() || undefined,
            timeout_ms: parseOptionalNumber(timeout),
          },
        }),
      false,
    );
  };

  const saveSettingsOnly = () => {
    onRun(
      'settings-save',
      () =>
        saveAgentSettings({
          ...snapshot.settings,
          knowledge: {
            enabled,
            default_base_id: defaultBaseId || undefined,
            bases: config.bases ?? [],
            agent_provider_id: agentProviderId.trim() || undefined,
            timeout_ms: parseOptionalNumber(timeout),
          },
        }),
      false,
    );
  };

  const deleteBase = () => {
    const bases = (config.bases ?? []).filter((item) => item.id !== base.id);
    const nextDefault = defaultBaseId === base.id ? bases[0]?.id ?? '' : defaultBaseId;
    onRun(
      'settings-save',
      () =>
        saveAgentSettings({
          ...snapshot.settings,
          knowledge: {
            enabled,
            default_base_id: nextDefault || undefined,
            bases,
            agent_provider_id: agentProviderId.trim() || undefined,
            timeout_ms: parseOptionalNumber(timeout),
          },
        }),
      false,
    );
  };

  const baseOptions = [
    { value: '', label: 'select default base' },
    ...(config.bases ?? []).map((item) => ({ value: item.id, label: `${item.name || item.id} (@${item.id})` })),
  ];

  return (
    <div className="grid grid--two">
      <Card className="panel">
        <CardHeader>
          <CardTitle>Knowledge Bases</CardTitle>
          <CardDescription>Slash commands run Codex CLI against one configured host folder at a time</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <div className="list-stack">
            {(config.bases ?? []).map((item) => (
              <SelectableListItem
                key={item.id}
                title={`${item.name || item.id} (@${item.id})`}
                description={item.base_dir}
                selected={item.id === base.id}
                badges={<Badge tone={item.enabled ? 'good' : 'neutral'} size="xxs">{item.enabled ? 'enabled' : 'disabled'}</Badge>}
                onClick={() => setBase(item)}
              />
            ))}
            {(config.bases ?? []).length === 0 ? <div className="detail">No knowledge bases configured.</div> : null}
          </div>
          <div className="button-row">
            <Button variant="secondary" onClick={() => setBase(emptyBase())}>
              <Plus className="mr-2 h-4 w-4" />
              New Base
            </Button>
          </div>
          <ToggleField label="Base enabled" checked={base.enabled !== false} onChange={(checked) => setBase({ ...base, enabled: checked })} />
          <FieldGrid>
            <Field label="Base ID" value={base.id} placeholder="ops" onChange={(id) => setBase({ ...base, id })} />
            <Field label="Name" value={base.name} placeholder="Operations KB" onChange={(name) => setBase({ ...base, name })} />
            <Field label="Base directory" value={base.base_dir} placeholder="/Users/me/Documents/Knowledge/Ops" onChange={(base_dir) => setBase({ ...base, base_dir })} />
          </FieldGrid>
          <div className="button-row">
            <Button onClick={save} disabled={busy === 'settings-save' || !base.base_dir.trim()}>
              <Save className="mr-2 h-4 w-4" />
              Save Base
            </Button>
            <Button variant="danger" disabled={!base.id} onClick={deleteBase}>
              <Trash2 className="mr-2 h-4 w-4" />
              Delete
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card className="panel">
        <CardHeader>
          <CardTitle>Knowledge Runtime</CardTitle>
          <CardDescription>Global /kb behavior shared by all configured knowledge bases</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <ToggleField label="Enable /kb slash command" checked={enabled} onChange={setEnabled} />
          <FieldGrid>
            <SelectField label="Default base" value={defaultBaseId} options={baseOptions} onChange={setDefaultBaseId} />
            <SelectField label="Agent provider" value={agentProviderId} options={providerOptions} onChange={setAgentProviderId} />
            <Field label="Timeout ms" value={timeout} placeholder="600000" onChange={setTimeoutValue} />
          </FieldGrid>
          <Button onClick={saveSettingsOnly} disabled={busy === 'settings-save'}>
            <Save className="mr-2 h-4 w-4" />
            Save Runtime
          </Button>
        </CardContent>
      </Card>

      <Card className="panel grid__full">
        <CardHeader>
          <CardTitle>WeCom Slash Commands</CardTitle>
          <CardDescription>Incoming WeCom text enters ProjectInput, then knowledge answers are rendered to images.</CardDescription>
        </CardHeader>
        <CardContent className="stack text-sm">
          <code>/kb ask &lt;question&gt;</code>
          <code>/kb @&lt;base-id&gt; ask &lt;question&gt;</code>
          <code>/kb @&lt;base-id&gt; &lt;question&gt;</code>
          <code>/kb &lt;question&gt;</code>
          <code>/kb new [@base-id] [question]</code>
          <code>/kb list</code>
          <code>/kb status [@base-id]</code>
          <p className="text-muted-foreground">
            Answers are saved as Markdown under <code>.answers</code> in the selected base directory and sent to WeCom as rendered images. Render failures fail the command instead of falling back to text.
          </p>
        </CardContent>
      </Card>
    </div>
  );
}

function emptyBase(): AgentKnowledgeBase {
  return { id: '', name: '', base_dir: '', enabled: true };
}

function normalizeBase(base: AgentKnowledgeBase): AgentKnowledgeBase {
  const id = base.id.trim() || slugId(base.name || base.base_dir, 'kb');
  return {
    ...base,
    id,
    name: base.name.trim(),
    base_dir: base.base_dir.trim(),
    enabled: base.enabled !== false,
  };
}

function replaceBase(items: AgentKnowledgeBase[], next: AgentKnowledgeBase) {
  return items.some((item) => item.id === next.id) ? items.map((item) => (item.id === next.id ? next : item)) : [...items, next];
}

function slugId(raw: string, prefix: string) {
  const slug = raw
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9._-]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 48);
  return slug || `${prefix}-${Date.now()}`;
}
