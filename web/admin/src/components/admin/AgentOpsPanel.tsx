import { useEffect, useMemo, useState } from 'react';
import { Image, Play, Plus, Save, TerminalSquare, Trash2 } from 'lucide-react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../ui/tabs';
import { Textarea } from '../ui/textarea';
import {
  createEvolutionGoal,
  renderAgentMarkdown,
  runAgentCapability,
  runAgentCodex,
  runAgentSearch,
  runEvolutionGoal,
  runTerminalCommand,
  saveAgentSettings,
  transcribeAgentSpeech,
  type AgentSnapshot,
} from '../../lib/agent';
import { AgentCapabilitiesCard } from './AgentCapabilitiesCard';
import { Field, FieldGrid, ToggleField, numberValue, parseOptionalNumber } from './AgentFormFields';
import type { AgentRunner } from './AgentWorkspace';

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

const newId = (prefix: string) => `${prefix}-${Date.now()}`;
const textOf = (value: unknown) => (typeof value === 'string' ? value : '');
const valueText = (value: unknown) => (typeof value === 'number' && Number.isFinite(value) ? String(value) : textOf(value));

export function AgentOpsPanel({ snapshot, busy, onRun }: Props) {
  const firstProvider = (snapshot.settings.search_engines?.[0] as SearchProvider | undefined) ?? emptySearchProvider();
  const [searchProvider, setSearchProvider] = useState<SearchProvider>(firstProvider);
  const [searchQuery, setSearchQuery] = useState('');
  const [searchSites, setSearchSites] = useState('');
  const [searchRecency, setSearchRecency] = useState('month');
  const [searchMaxItems, setSearchMaxItems] = useState('8');
  const [evolutionGoal, setEvolutionGoal] = useState('');
  const [commitMessage, setCommitMessage] = useState('');
  const [evolutionCommand, setEvolutionCommand] = useState(snapshot.settings.evolution.command ?? '');
  const [evolutionCwd, setEvolutionCwd] = useState(snapshot.settings.evolution.cwd ?? '');
  const [evolutionTimeout, setEvolutionTimeout] = useState(numberValue(snapshot.settings.evolution.timeout_ms));
  const [evolutionModel, setEvolutionModel] = useState(snapshot.settings.evolution.codex_model ?? '');
  const [evolutionReasoning, setEvolutionReasoning] = useState(snapshot.settings.evolution.codex_reasoning ?? '');
  const [maxFixAttempts, setMaxFixAttempts] = useState(numberValue(snapshot.settings.evolution.max_fix_attempts));
  const [autoCommit, setAutoCommit] = useState(snapshot.settings.evolution.auto_commit === true);
  const [autoPush, setAutoPush] = useState(snapshot.settings.evolution.auto_push === true);
  const [structureReview, setStructureReview] = useState(snapshot.settings.evolution.structure_review === true);
  const [terminalCommand, setTerminalCommand] = useState('');
  const [terminalCwd, setTerminalCwd] = useState(snapshot.settings.terminal.cwd ?? '');
  const [audioPath, setAudioPath] = useState('');
  const [codexPrompt, setCodexPrompt] = useState('');
  const [codexModel, setCodexModel] = useState('');
  const [codexReasoning, setCodexReasoning] = useState('');
  const [codexCwd, setCodexCwd] = useState('');
  const [codexTimeout, setCodexTimeout] = useState('');
  const [markdownInput, setMarkdownInput] = useState('# Celestia\n\n- md2img render');
  const [markdownMode, setMarkdownMode] = useState<'long-image' | 'multi-page'>('long-image');
  const [markdownOutputDir, setMarkdownOutputDir] = useState('');

  const runnableEvolutionGoal = useMemo(
    () => snapshot.evolution.goals.find((goal) => goal.status !== 'succeeded') ?? snapshot.evolution.goals[0],
    [snapshot.evolution.goals],
  );

  useEffect(() => {
    setSearchProvider((snapshot.settings.search_engines?.[0] as SearchProvider | undefined) ?? emptySearchProvider());
    setEvolutionCommand(snapshot.settings.evolution.command ?? '');
    setEvolutionCwd(snapshot.settings.evolution.cwd ?? '');
    setEvolutionTimeout(numberValue(snapshot.settings.evolution.timeout_ms));
    setEvolutionModel(snapshot.settings.evolution.codex_model ?? '');
    setEvolutionReasoning(snapshot.settings.evolution.codex_reasoning ?? '');
    setMaxFixAttempts(numberValue(snapshot.settings.evolution.max_fix_attempts));
    setAutoCommit(snapshot.settings.evolution.auto_commit === true);
    setAutoPush(snapshot.settings.evolution.auto_push === true);
    setStructureReview(snapshot.settings.evolution.structure_review === true);
    setTerminalCwd(snapshot.settings.terminal.cwd ?? '');
  }, [snapshot]);

  const saveSearchProvider = () => {
    const id = textOf(searchProvider.id).trim() || newId('search');
    const config = searchProvider.config ?? {};
    const next = { ...searchProvider, id, enabled: searchProvider.enabled !== false, config };
    const engines = replaceRecord(snapshot.settings.search_engines ?? [], next);
    onRun('settings-save', () => saveAgentSettings({ ...snapshot.settings, search_engines: engines }), false);
  };

  const deleteSearchProvider = () => {
    const engines = (snapshot.settings.search_engines ?? []).filter((item) => textOf(item.id) !== textOf(searchProvider.id));
    onRun('settings-save', () => saveAgentSettings({ ...snapshot.settings, search_engines: engines }), false);
  };

  const runSearch = () => {
    const sites = searchSites
      .split(',')
      .map((site) => site.trim())
      .filter(Boolean);
    onRun(
      'search',
      () =>
        runAgentSearch({
          engine_selector: textOf(searchProvider.id),
          max_items: parseOptionalNumber(searchMaxItems),
          timeout_ms: 12000,
          plans: [{ label: 'manual', query: searchQuery, sites, recency: searchRecency }],
        }),
      false,
    );
  };

  const saveEvolutionSettings = () => {
    onRun(
      'settings-save',
      () =>
        saveAgentSettings({
          ...snapshot.settings,
          evolution: {
            ...snapshot.settings.evolution,
            command: evolutionCommand.trim() || undefined,
            cwd: evolutionCwd.trim() || undefined,
            timeout_ms: parseOptionalNumber(evolutionTimeout),
            codex_model: evolutionModel.trim() || undefined,
            codex_reasoning: evolutionReasoning.trim() || undefined,
            max_fix_attempts: parseOptionalNumber(maxFixAttempts),
            auto_commit: autoCommit,
            auto_push: autoPush,
            structure_review: structureReview,
          },
        }),
      false,
    );
  };

  return (
    <Tabs defaultValue="search">
      <TabsList className="flex-wrap justify-start">
        <TabsTrigger value="search">Search</TabsTrigger>
        <TabsTrigger value="capabilities">Capabilities</TabsTrigger>
        <TabsTrigger value="evolution">Evolution</TabsTrigger>
        <TabsTrigger value="terminal">Terminal</TabsTrigger>
        <TabsTrigger value="speech">Speech</TabsTrigger>
        <TabsTrigger value="codex">Codex</TabsTrigger>
        <TabsTrigger value="markdown">Markdown</TabsTrigger>
      </TabsList>

      <TabsContent value="search" className="grid grid--two">
        <Card className="panel">
          <CardHeader>
            <CardTitle>Search Providers</CardTitle>
            <CardDescription>{snapshot.settings.search_engines?.length ?? 0} configured providers</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <div className="button-row">
              {(snapshot.settings.search_engines ?? []).map((item) => (
                <Button key={textOf(item.id)} variant={textOf(item.id) === textOf(searchProvider.id) ? 'default' : 'secondary'} onClick={() => setSearchProvider(item as SearchProvider)}>
                  {textOf(item.name) || textOf(item.id)}
                </Button>
              ))}
              <Button variant="secondary" onClick={() => setSearchProvider(emptySearchProvider(newId('search')))}>
                <Plus className="mr-2 h-4 w-4" />
                New
              </Button>
            </div>
            <ToggleField label="Provider enabled" checked={searchProvider.enabled !== false} onChange={(enabled) => setSearchProvider({ ...searchProvider, enabled })} />
            <FieldGrid>
              <Field label="ID" value={textOf(searchProvider.id)} onChange={(id) => setSearchProvider({ ...searchProvider, id })} />
              <Field label="Name" value={textOf(searchProvider.name)} onChange={(name) => setSearchProvider({ ...searchProvider, name })} />
              <Field label="Type" value={textOf(searchProvider.type)} onChange={(type) => setSearchProvider({ ...searchProvider, type })} />
              <ConfigField provider={searchProvider} name="endpoint" label="Endpoint" onChange={setSearchProvider} />
              <ConfigField provider={searchProvider} name="apiKey" label="API Key" onChange={setSearchProvider} />
              <ConfigField provider={searchProvider} name="engine" label="Engine" onChange={setSearchProvider} />
              <ConfigField provider={searchProvider} name="searchSource" label="Search source" onChange={setSearchProvider} />
              <ConfigField provider={searchProvider} name="edition" label="Edition" onChange={setSearchProvider} />
              <ConfigField provider={searchProvider} name="hl" label="Language" onChange={setSearchProvider} />
              <ConfigField provider={searchProvider} name="gl" label="Region" onChange={setSearchProvider} />
              <ConfigField provider={searchProvider} name="num" label="Num" onChange={setSearchProvider} />
              <ConfigField provider={searchProvider} name="topK" label="Top K" onChange={setSearchProvider} />
              <ConfigField provider={searchProvider} name="recencyFilter" label="Recency filter" onChange={setSearchProvider} />
            </FieldGrid>
            <div className="button-row">
              <Button onClick={saveSearchProvider} disabled={busy === 'settings-save'}>
                <Save className="mr-2 h-4 w-4" />
                Save Provider
              </Button>
              <Button variant="danger" onClick={deleteSearchProvider} disabled={!searchProvider.id}>
                <Trash2 className="mr-2 h-4 w-4" />
                Delete
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card className="panel">
          <CardHeader>
            <CardTitle>Run Search</CardTitle>
            <CardDescription>Calls the configured search engine through the agent runtime</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <Field label="Query" value={searchQuery} onChange={setSearchQuery} />
            <Field label="Sites" value={searchSites} placeholder="example.com, another.com" onChange={setSearchSites} />
            <FieldGrid>
              <Field label="Recency" value={searchRecency} onChange={setSearchRecency} />
              <Field label="Max items" value={searchMaxItems} onChange={setSearchMaxItems} />
            </FieldGrid>
            <Button onClick={runSearch} disabled={!searchQuery.trim()}>
              <Play className="mr-2 h-4 w-4" />
              Search
            </Button>
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="capabilities" className="grid grid--two">
        <AgentCapabilitiesCard
          capabilities={snapshot.capabilities}
          busy={busy === 'capability'}
          onRun={(name, input) => onRun('capability', () => runAgentCapability(name, { input }), false)}
        />
      </TabsContent>

      <TabsContent value="evolution" className="grid grid--two">
        <Card className="panel">
          <CardHeader>
            <CardTitle>Evolution Operator</CardTitle>
            <CardDescription>{snapshot.evolution.goals.length} goals</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <Textarea value={evolutionGoal} onChange={(event) => setEvolutionGoal(event.target.value)} placeholder="Goal" />
            <Field label="Commit message" value={commitMessage} onChange={setCommitMessage} />
            <div className="button-row">
              <Button onClick={() => onRun('evolution', () => createEvolutionGoal({ goal: evolutionGoal, commit_message: commitMessage || undefined }))} disabled={!evolutionGoal.trim()}>
                Queue
              </Button>
              <Button variant="secondary" disabled={!runnableEvolutionGoal} onClick={() => onRun('evolution', () => runEvolutionGoal(runnableEvolutionGoal!.id))}>
                <Play className="mr-2 h-4 w-4" />
                Run
              </Button>
            </div>
            {runnableEvolutionGoal ? (
              <div className="rounded-md border border-border-light p-3 text-sm">
                <Badge tone={runnableEvolutionGoal.status === 'succeeded' ? 'good' : 'neutral'}>{runnableEvolutionGoal.status}</Badge>
                <p className="mt-2">{runnableEvolutionGoal.goal}</p>
              </div>
            ) : null}
          </CardContent>
        </Card>

        <Card className="panel">
          <CardHeader>
            <CardTitle>Evolution Settings</CardTitle>
            <CardDescription>Codex runner settings used by queued goals</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <FieldGrid>
              <Field label="Command" value={evolutionCommand} onChange={setEvolutionCommand} />
              <Field label="Cwd" value={evolutionCwd} onChange={setEvolutionCwd} />
              <Field label="Timeout ms" value={evolutionTimeout} onChange={setEvolutionTimeout} />
              <Field label="Codex model" value={evolutionModel} onChange={setEvolutionModel} />
              <Field label="Reasoning" value={evolutionReasoning} onChange={setEvolutionReasoning} />
              <Field label="Max fix attempts" value={maxFixAttempts} onChange={setMaxFixAttempts} />
            </FieldGrid>
            <ToggleField label="Auto commit" checked={autoCommit} onChange={setAutoCommit} />
            <ToggleField label="Auto push" checked={autoPush} onChange={setAutoPush} />
            <ToggleField label="Structure review" checked={structureReview} onChange={setStructureReview} />
            <Button onClick={saveEvolutionSettings}>
              <Save className="mr-2 h-4 w-4" />
              Save Settings
            </Button>
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="terminal" className="grid grid--two">
        <Card className="panel">
          <CardHeader>
            <CardTitle>Terminal</CardTitle>
            <CardDescription>Requires terminal to be enabled in runtime settings</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <Field label="Command" value={terminalCommand} onChange={setTerminalCommand} />
            <Field label="Cwd" value={terminalCwd} onChange={setTerminalCwd} />
            <Button onClick={() => onRun('terminal', () => runTerminalCommand({ command: terminalCommand, cwd: terminalCwd || undefined }), false)} disabled={!terminalCommand.trim()}>
              <TerminalSquare className="mr-2 h-4 w-4" />
              Run
            </Button>
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="speech" className="grid grid--two">
        <Card className="panel">
          <CardHeader>
            <CardTitle>Speech Transcription</CardTitle>
            <CardDescription>Runs configured STT command</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <Field label="Audio path" value={audioPath} onChange={setAudioPath} />
            <Button onClick={() => onRun('stt', () => transcribeAgentSpeech({ audio_path: audioPath }), false)} disabled={!audioPath.trim()}>
              <Play className="mr-2 h-4 w-4" />
              Transcribe
            </Button>
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="codex" className="grid grid--two">
        <Card className="panel">
          <CardHeader>
            <CardTitle>Codex Runner</CardTitle>
            <CardDescription>Runs codex exec from the agent runtime</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <Textarea value={codexPrompt} onChange={(event) => setCodexPrompt(event.target.value)} placeholder="Codex prompt" />
            <FieldGrid>
              <Field label="Model" value={codexModel} onChange={setCodexModel} />
              <Field label="Reasoning effort" value={codexReasoning} onChange={setCodexReasoning} />
              <Field label="Cwd" value={codexCwd} onChange={setCodexCwd} />
              <Field label="Timeout ms" value={codexTimeout} onChange={setCodexTimeout} />
            </FieldGrid>
            <Button
              onClick={() =>
                onRun('codex', () =>
                  runAgentCodex({
                    prompt: codexPrompt,
                    model: codexModel || undefined,
                    reasoning_effort: codexReasoning || undefined,
                    cwd: codexCwd || undefined,
                    timeout_ms: parseOptionalNumber(codexTimeout),
                  }), false)
              }
              disabled={!codexPrompt.trim()}
            >
              <TerminalSquare className="mr-2 h-4 w-4" />
              Run Codex
            </Button>
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="markdown" className="grid grid--two">
        <Card className="panel">
          <CardHeader>
            <CardTitle>Markdown Renderer</CardTitle>
            <CardDescription>Renders markdown through md2img</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <Textarea value={markdownInput} onChange={(event) => setMarkdownInput(event.target.value)} />
            <Field label="Output directory" value={markdownOutputDir} onChange={setMarkdownOutputDir} />
            <div className="button-row">
              <Button variant={markdownMode === 'long-image' ? 'default' : 'secondary'} onClick={() => setMarkdownMode('long-image')}>
                Long Image
              </Button>
              <Button variant={markdownMode === 'multi-page' ? 'default' : 'secondary'} onClick={() => setMarkdownMode('multi-page')}>
                Multi Page
              </Button>
              <Button onClick={() => onRun('md2img', () => renderAgentMarkdown({ markdown: markdownInput, mode: markdownMode, output_dir: markdownOutputDir || undefined }), false)}>
                <Image className="mr-2 h-4 w-4" />
                Render
              </Button>
            </div>
          </CardContent>
        </Card>
      </TabsContent>
    </Tabs>
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

function numericConfig(name: string, value: string) {
  return ['num', 'topK'].includes(name) ? parseOptionalNumber(value) : value;
}

function emptySearchProvider(id = ''): SearchProvider {
  return { id, name: '', type: 'serpapi', enabled: true, config: { endpoint: '', apiKey: '', engine: 'google_news', hl: 'zh-cn', gl: 'cn' } };
}

function replaceRecord<T extends Record<string, unknown>>(items: T[], next: T) {
  const id = textOf(next.id);
  return items.some((item) => textOf(item.id) === id) ? items.map((item) => (textOf(item.id) === id ? next : item)) : [...items, next];
}
