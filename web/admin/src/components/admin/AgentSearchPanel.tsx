import { useEffect, useState } from 'react';
import { Play, Plus, Save, Trash2 } from 'lucide-react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import {
  runAgentSearch,
  saveAgentSettings,
  type AgentSnapshot,
} from '../../lib/agent';
import { Field, FieldGrid, SelectField, ToggleField, parseOptionalNumber } from './AgentFormFields';
import type { AgentRunner } from './AgentWorkspace';
import { SelectableListItem } from './shared/SelectableListItem';

type Props = {
  snapshot: AgentSnapshot;
  busy: string;
  onRun: AgentRunner;
};

type SearchProvider = Record<string, unknown> & {
  id?: string;
  name?: string;
  type?: string;
  enabled?: boolean;
  config?: Record<string, unknown>;
};

const searchTypes = [
  { value: 'serpapi', label: 'serpapi' },
  { value: 'qianfan', label: 'qianfan' },
];

const qianfanEditions = [
  { value: 'standard', label: 'standard' },
  { value: 'lite', label: 'lite' },
];

const recencyOptions = [
  { value: '', label: 'none' },
  { value: 'week', label: 'week' },
  { value: 'month', label: 'month' },
  { value: 'semiyear', label: 'semiyear' },
  { value: 'year', label: 'year' },
];

const textOf = (value: unknown) => (typeof value === 'string' ? value : '');
const valueText = (value: unknown) => (typeof value === 'number' && Number.isFinite(value) ? String(value) : textOf(value));

export function AgentSearchPanel({ snapshot, busy, onRun }: Props) {
  const firstProvider = (snapshot.settings.search_engines?.[0] as SearchProvider | undefined) ?? emptySearchProvider();
  const [provider, setProvider] = useState<SearchProvider>(firstProvider);
  const [query, setQuery] = useState('');
  const [sites, setSites] = useState('');
  const [maxItems, setMaxItems] = useState('8');

  useEffect(() => {
    setProvider((snapshot.settings.search_engines?.[0] as SearchProvider | undefined) ?? emptySearchProvider());
  }, [snapshot]);

  const saveProvider = () => {
    const id = textOf(provider.id) || slugId(textOf(provider.name) || normalizeType(provider.type), 'search');
    const next = { ...provider, id, type: normalizeType(provider.type), enabled: provider.enabled !== false, config: provider.config ?? {} };
    setProvider(next);
    onRun('settings-save', () => saveAgentSettings({ ...snapshot.settings, search_engines: replaceRecord(snapshot.settings.search_engines ?? [], next) }), false);
  };

  const runSearch = () => {
    onRun(
      'search',
      () =>
        runAgentSearch({
          engine_selector: textOf(provider.id),
          max_items: parseOptionalNumber(maxItems),
          timeout_ms: 12000,
          plans: [{ label: 'manual', query, sites: splitSites(sites), recency: textOf((provider.config ?? {}).recencyFilter) || 'month' }],
        }),
      false,
    );
  };

  const type = normalizeType(provider.type);
  const config = provider.config ?? {};

  return (
    <div className="grid grid--two">
      <Card className="panel">
        <CardHeader>
          <CardTitle>Search Engine Profiles</CardTitle>
          <CardDescription>Global search profiles used by Market and other Agent workflows</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <div className="list-stack">
            {(snapshot.settings.search_engines ?? []).map((item) => (
              <SelectableListItem
                key={textOf(item.id)}
                title={textOf(item.name) || textOf(item.id)}
                description={normalizeType(item.type)}
                selected={textOf(item.id) === textOf(provider.id)}
                badges={<Badge tone={item.enabled === false ? 'neutral' : 'good'} size="xxs">{item.enabled === false ? 'disabled' : 'enabled'}</Badge>}
                onClick={() => {
                  setProvider(item as SearchProvider);
                }}
              />
            ))}
            {(snapshot.settings.search_engines ?? []).length === 0 ? <div className="detail">No search engine profiles configured.</div> : null}
          </div>
          <div className="button-row">
            <Button
              variant="secondary"
              onClick={() => {
                setProvider(emptySearchProvider());
              }}
            >
              <Plus className="mr-2 h-4 w-4" />
              New
            </Button>
          </div>
          <ToggleField label="Provider enabled" checked={provider.enabled !== false} onChange={(enabled) => setProvider({ ...provider, enabled })} />
          <FieldGrid>
            <Field label="Name" value={textOf(provider.name)} onChange={(name) => setProvider({ ...provider, name })} />
            <SelectField
              label="Type"
              value={type}
              options={searchTypes}
              onChange={(nextType) => setProvider({ ...emptySearchProvider(textOf(provider.id), nextType), name: textOf(provider.name), enabled: provider.enabled !== false })}
            />
            <ConfigField provider={provider} name="endpoint" label="Endpoint" onChange={setProvider} />
            <ConfigField provider={provider} name="apiKey" label="API Key" onChange={setProvider} />
            {type === 'qianfan' ? (
              <>
                <ConfigField provider={provider} name="searchSource" label="Search source" onChange={setProvider} />
                <ConfigSelect provider={provider} name="edition" label="Edition" options={qianfanEditions} onChange={setProvider} />
                <ConfigField provider={provider} name="topK" label="Top K" onChange={setProvider} />
                <ConfigSelect provider={provider} name="recencyFilter" label="Recency filter" options={recencyOptions} onChange={setProvider} />
              </>
            ) : (
              <>
                <ConfigField provider={provider} name="engine" label="Engine" onChange={setProvider} />
                <ConfigField provider={provider} name="num" label="Num" onChange={setProvider} />
                <ConfigField provider={provider} name="hl" label="Language" onChange={setProvider} />
                <ConfigField provider={provider} name="gl" label="Region" onChange={setProvider} />
              </>
            )}
          </FieldGrid>
          <div className="button-row">
            <Button onClick={saveProvider} disabled={busy === 'settings-save' || !provider.name}>
              <Save className="mr-2 h-4 w-4" />
              Save Provider
            </Button>
            <Button
              variant="danger"
              disabled={!provider.id}
              onClick={() =>
                onRun(
                  'settings-save',
                  () => saveAgentSettings({ ...snapshot.settings, search_engines: (snapshot.settings.search_engines ?? []).filter((item) => textOf(item.id) !== textOf(provider.id)) }),
                  false,
                )
              }
            >
              <Trash2 className="mr-2 h-4 w-4" />
              Delete
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card className="panel">
        <CardHeader>
          <CardTitle>Search Smoke Run</CardTitle>
          <CardDescription>Quickly validates the selected provider; normal business flows call this internally</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <Field label="Query" value={query} onChange={setQuery} />
          <Field label="Sites" value={sites} placeholder="example.com, another.com" onChange={setSites} />
          <Field label="Max items" value={maxItems} onChange={setMaxItems} />
          <Button onClick={runSearch} disabled={!query.trim()}>
            <Play className="mr-2 h-4 w-4" />
            Search
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}

function ConfigField(props: { provider: SearchProvider; name: string; label: string; onChange: (next: SearchProvider) => void }) {
  const config = props.provider.config ?? {};
  return (
    <Field
      label={props.label}
      value={valueText(config[props.name])}
      onChange={(value) => props.onChange({ ...props.provider, config: { ...config, [props.name]: numericConfig(props.name, value) } })}
    />
  );
}

function ConfigSelect(props: { provider: SearchProvider; name: string; label: string; options: Array<{ value: string; label: string }>; onChange: (next: SearchProvider) => void }) {
  const config = props.provider.config ?? {};
  return (
    <SelectField
      label={props.label}
      value={valueText(config[props.name])}
      options={props.options}
      onChange={(value) => props.onChange({ ...props.provider, config: { ...config, [props.name]: value } })}
    />
  );
}

function emptySearchProvider(id = '', type = 'serpapi'): SearchProvider {
  if (type === 'qianfan') {
    return { id, name: '', type, enabled: true, config: { endpoint: 'https://qianfan.baidubce.com/v2/ai_search/web_search', apiKey: '', searchSource: 'baidu_search_v2', edition: 'standard', topK: 10, recencyFilter: 'month' } };
  }
  return { id, name: '', type: 'serpapi', enabled: true, config: { endpoint: 'https://serpapi.com/search.json', apiKey: '', engine: 'google_news', hl: 'zh-cn', gl: 'cn', num: 10 } };
}

function normalizeType(type: unknown) {
  return type === 'qianfan' ? 'qianfan' : 'serpapi';
}

function numericConfig(name: string, value: string) {
  return ['num', 'topK'].includes(name) ? parseOptionalNumber(value) : value;
}

function replaceRecord<T extends Record<string, unknown>>(items: T[], next: T) {
  const id = textOf(next.id);
  return items.some((item) => textOf(item.id) === id) ? items.map((item) => (textOf(item.id) === id ? next : item)) : [...items, next];
}

function splitSites(raw: string) {
  return raw.split(',').map((site) => site.trim()).filter(Boolean);
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
