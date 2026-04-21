import { useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import {
  Bot,
  Image,
  Play,
  RefreshCw,
  Save,
  Send,
  TerminalSquare,
} from 'lucide-react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Input } from '../ui/input';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../ui/tabs';
import { Textarea } from '../ui/textarea';
import {
  addWritingMaterial,
  createEvolutionGoal,
  createWritingTopic,
  fetchAgentSnapshot,
  parseJSONObject,
  publishAgentWeComMenu,
  runAgentCodex,
  runAgentConversation,
  runAgentSearch,
  runAgentTopic,
  runEvolutionGoal,
  runMarketAnalysis,
  runTerminalCommand,
  saveAgentDirectInput,
  saveAgentPush,
  saveAgentSettings,
  saveAgentTopic,
  saveAgentWeComMenu,
  saveMarketPortfolio,
  sendAgentWeComImage,
  sendAgentWeComMessage,
  stableJSON,
  summarizeWritingTopic,
  transcribeAgentSpeech,
  type AgentDirectInputConfig,
  type AgentMarketPortfolio,
  type AgentSettings,
  type AgentSnapshot,
  type AgentTopicSnapshot,
  type AgentWeComMenuConfig,
} from '../../lib/agent';

type Busy =
  | ''
  | 'load'
  | 'settings'
  | 'direct'
  | 'push'
  | 'wecom'
  | 'publish'
  | 'send'
  | 'conversation'
  | 'topic'
  | 'topic-run'
  | 'writing'
  | 'market'
  | 'evolution'
  | 'terminal'
  | 'search'
  | 'stt'
  | 'codex';

export function AgentWorkspace() {
  const [snapshot, setSnapshot] = useState<AgentSnapshot | null>(null);
  const [busy, setBusy] = useState<Busy>('load');
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [settingsDraft, setSettingsDraft] = useState('{}');
  const [directDraft, setDirectDraft] = useState('{}');
  const [pushDraft, setPushDraft] = useState('{}');
  const [wecomDraft, setWecomDraft] = useState('{}');
  const [topicDraft, setTopicDraft] = useState('{}');
  const [marketDraft, setMarketDraft] = useState('{}');
  const [conversationInput, setConversationInput] = useState('');
  const [wecomUser, setWecomUser] = useState('');
  const [wecomText, setWecomText] = useState('');
  const [wecomImage, setWecomImage] = useState('');
  const [wecomImageName, setWecomImageName] = useState('');
  const [writingTitle, setWritingTitle] = useState('');
  const [writingMaterial, setWritingMaterial] = useState('');
  const [marketNotes, setMarketNotes] = useState('');
  const [evolutionGoal, setEvolutionGoal] = useState('');
  const [terminalCommand, setTerminalCommand] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
  const [audioPath, setAudioPath] = useState('');
  const [codexPrompt, setCodexPrompt] = useState('');
  const [resultText, setResultText] = useState('');

  const activeWritingTopic = snapshot?.writing.topics[0] ?? null;
  const runnableEvolutionGoal = useMemo(
    () => snapshot?.evolution.goals.find((goal) => goal.status !== 'succeeded') ?? snapshot?.evolution.goals[0] ?? null,
    [snapshot],
  );

  const syncDrafts = (next: AgentSnapshot) => {
    setSnapshot(next);
    setSettingsDraft(stableJSON(next.settings));
    setDirectDraft(stableJSON(next.direct_input));
    setPushDraft(stableJSON(next.push));
    setWecomDraft(stableJSON(next.wecom_menu.config));
    setTopicDraft(stableJSON(next.topic_summary));
    setMarketDraft(stableJSON(next.market.portfolio));
  };

  const load = async () => {
    setBusy('load');
    setError('');
    try {
      syncDrafts(await fetchAgentSnapshot());
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load agent state');
    } finally {
      setBusy('');
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const run = async (label: Busy, action: () => Promise<unknown>, refresh = true) => {
    setBusy(label);
    setError('');
    setNotice('');
    try {
      const output = await action();
      setResultText(stableJSON(output));
      setNotice('Saved');
      if (refresh) {
        syncDrafts(await fetchAgentSnapshot());
      } else if (output && typeof output === 'object' && 'settings' in output) {
        syncDrafts(output as AgentSnapshot);
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
    <div className="detail-stack">
      <div className="section-title">
        <div>
          <p className="eyebrow">Paimon Runtime Migration</p>
          <h2 className="text-2xl font-semibold tracking-tight">Agent</h2>
        </div>
        <div className="toolbar">
          <Badge tone={snapshot.settings.llm_providers.length ? 'good' : 'warn'}>
            {snapshot.settings.llm_providers.length} LLM
          </Badge>
          <Badge tone={snapshot.settings.wecom.enabled ? 'good' : 'neutral'}>WeCom</Badge>
          <Badge tone={snapshot.settings.terminal.enabled ? 'warn' : 'neutral'}>Terminal</Badge>
          <Button variant="secondary" onClick={() => void load()} disabled={busy === 'load'}>
            <RefreshCw className={`mr-2 h-4 w-4 ${busy === 'load' ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
        </div>
      </div>

      {error ? <div className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm">{error}</div> : null}
      {notice ? <div className="rounded-md border border-primary/20 bg-primary/5 p-3 text-sm">{notice}</div> : null}

      <Tabs defaultValue="runtime" className="min-h-0 flex-1">
        <TabsList className="flex-wrap justify-start">
          <TabsTrigger value="runtime">
            <Bot className="mr-2 h-4 w-4" />
            Runtime
          </TabsTrigger>
          <TabsTrigger value="wecom">WeCom</TabsTrigger>
          <TabsTrigger value="content">Content</TabsTrigger>
          <TabsTrigger value="ops">Ops</TabsTrigger>
        </TabsList>

        <TabsContent value="runtime" className="grid grid--two">
          <EditorCard
            title="Settings"
            description="LLM, STT, WeCom credentials, Terminal, Evolution runner"
            value={settingsDraft}
            onChange={setSettingsDraft}
            onSave={() =>
              run('settings', () => saveAgentSettings(parseJSONObject<AgentSettings>(settingsDraft, 'settings')), false)
            }
            busy={busy === 'settings'}
          />
          <EditorCard
            title="Direct Input"
            description="Fixed text mapping before LLM planning"
            value={directDraft}
            onChange={setDirectDraft}
            onSave={() =>
              run('direct', () => saveAgentDirectInput(parseJSONObject<AgentDirectInputConfig>(directDraft, 'direct input')), false)
            }
            busy={busy === 'direct'}
          />
          <EditorCard
            title="Push Users And Tasks"
            description="WeCom push users and interval tasks"
            value={pushDraft}
            onChange={setPushDraft}
            onSave={() => run('push', () => saveAgentPush(parseJSONObject(pushDraft, 'push')), false)}
            busy={busy === 'push'}
          />
          <Card className="panel">
            <CardHeader>
              <CardTitle>Conversation</CardTitle>
              <CardDescription>{snapshot.conversations.length} retained turns</CardDescription>
            </CardHeader>
            <CardContent className="stack">
              <Textarea value={conversationInput} onChange={(event) => setConversationInput(event.target.value)} />
              <Button
                onClick={() => run('conversation', () => runAgentConversation({ input: conversationInput }))}
                disabled={!conversationInput.trim() || busy === 'conversation'}
              >
                <Play className="mr-2 h-4 w-4" />
                Run
              </Button>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="wecom" className="grid grid--two">
          <EditorCard
            title="Menu"
            description={`${snapshot.wecom_menu.recent_events.length} recent click events`}
            value={wecomDraft}
            onChange={setWecomDraft}
            onSave={() => run('wecom', () => saveAgentWeComMenu(parseJSONObject<AgentWeComMenuConfig>(wecomDraft, 'menu')), false)}
            busy={busy === 'wecom'}
          />
          <Card className="panel">
            <CardHeader>
              <CardTitle>Send And Publish</CardTitle>
              <CardDescription>{snapshot.wecom_menu.validation_errors?.join('; ') || 'Menu payload ready'}</CardDescription>
            </CardHeader>
            <CardContent className="stack">
              <Input value={wecomUser} onChange={(event) => setWecomUser(event.target.value)} placeholder="WeCom user id" />
              <Textarea value={wecomText} onChange={(event) => setWecomText(event.target.value)} placeholder="Message" />
              <Input value={wecomImageName} onChange={(event) => setWecomImageName(event.target.value)} placeholder="Image filename" />
              <Textarea value={wecomImage} onChange={(event) => setWecomImage(event.target.value)} placeholder="Image base64 or data URL" />
              <div className="button-row">
                <Button onClick={() => run('send', () => sendAgentWeComMessage({ to_user: wecomUser, text: wecomText }))}>
                  <Send className="mr-2 h-4 w-4" />
                  Send
                </Button>
                <Button variant="secondary" onClick={() => run('send', () => sendAgentWeComImage({ to_user: wecomUser, base64: wecomImage, filename: wecomImageName || undefined }))}>
                  <Image className="mr-2 h-4 w-4" />
                  Image
                </Button>
                <Button variant="secondary" onClick={() => run('publish', () => publishAgentWeComMenu())}>
                  <Save className="mr-2 h-4 w-4" />
                  Publish
                </Button>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="content" className="grid grid--two">
          <EditorCard
            title="Topic Summary"
            description={`${snapshot.topic_summary.profiles.length} profiles, ${snapshot.topic_summary.runs.length} runs`}
            value={topicDraft}
            onChange={setTopicDraft}
            onSave={() => run('topic', () => saveAgentTopic(parseJSONObject<AgentTopicSnapshot>(topicDraft, 'topic summary')), false)}
            busy={busy === 'topic'}
            extra={<Button variant="secondary" onClick={() => run('topic-run', () => runAgentTopic(snapshot.topic_summary.active_profile_id))}>Run</Button>}
          />
          <Card className="panel">
            <CardHeader>
              <CardTitle>Writing Organizer</CardTitle>
              <CardDescription>{activeWritingTopic ? activeWritingTopic.title : 'No topic selected'}</CardDescription>
            </CardHeader>
            <CardContent className="stack">
              <Input value={writingTitle} onChange={(event) => setWritingTitle(event.target.value)} placeholder="Topic title" />
              <Button onClick={() => run('writing', () => createWritingTopic({ title: writingTitle }))}>Create Topic</Button>
              <Textarea value={writingMaterial} onChange={(event) => setWritingMaterial(event.target.value)} placeholder="Material" />
              <div className="button-row">
                <Button disabled={!activeWritingTopic} onClick={() => run('writing', () => addWritingMaterial(activeWritingTopic!.id, { content: writingMaterial }))}>
                  Add Material
                </Button>
                <Button variant="secondary" disabled={!activeWritingTopic} onClick={() => run('writing', () => summarizeWritingTopic(activeWritingTopic!.id))}>
                  Summarize
                </Button>
              </div>
            </CardContent>
          </Card>
          <EditorCard
            title="Market Portfolio"
            description={`${snapshot.market.portfolio.funds.length} holdings, ${snapshot.market.runs.length} runs`}
            value={marketDraft}
            onChange={setMarketDraft}
            onSave={() => run('market', () => saveMarketPortfolio(parseJSONObject<AgentMarketPortfolio>(marketDraft, 'portfolio')), false)}
            busy={busy === 'market'}
            extra={
              <div className="stack">
                <Textarea value={marketNotes} onChange={(event) => setMarketNotes(event.target.value)} placeholder="Run notes" />
                <Button variant="secondary" onClick={() => run('market', () => runMarketAnalysis({ phase: 'close', notes: marketNotes }))}>Run Close</Button>
              </div>
            }
          />
        </TabsContent>

        <TabsContent value="ops" className="grid grid--two">
          <Card className="panel">
            <CardHeader>
              <CardTitle>Search Engine</CardTitle>
              <CardDescription>{snapshot.settings.search_engines?.length ?? 0} configured providers</CardDescription>
            </CardHeader>
            <CardContent className="stack">
              <Input value={searchQuery} onChange={(event) => setSearchQuery(event.target.value)} placeholder="Search query" />
              <Button
                onClick={() =>
                  run('search', () =>
                    runAgentSearch({
                      max_items: 8,
                      timeout_ms: 12000,
                      plans: [{ label: 'manual', query: searchQuery, recency: 'month' }],
                    }), false)
                }
              >
                <Play className="mr-2 h-4 w-4" />
                Search
              </Button>
            </CardContent>
          </Card>
          <Card className="panel">
            <CardHeader>
              <CardTitle>Evolution Operator</CardTitle>
              <CardDescription>{runnableEvolutionGoal ? runnableEvolutionGoal.status : 'No queued goal'}</CardDescription>
            </CardHeader>
            <CardContent className="stack">
              <Textarea value={evolutionGoal} onChange={(event) => setEvolutionGoal(event.target.value)} />
              <div className="button-row">
                <Button onClick={() => run('evolution', () => createEvolutionGoal({ goal: evolutionGoal }))}>Queue</Button>
                <Button variant="secondary" disabled={!runnableEvolutionGoal} onClick={() => run('evolution', () => runEvolutionGoal(runnableEvolutionGoal!.id))}>
                  Run
                </Button>
              </div>
            </CardContent>
          </Card>
          <Card className="panel">
            <CardHeader>
              <CardTitle>Terminal</CardTitle>
              <CardDescription>Requires terminal.enabled in settings</CardDescription>
            </CardHeader>
            <CardContent className="stack">
              <Input value={terminalCommand} onChange={(event) => setTerminalCommand(event.target.value)} placeholder="Command" />
              <Button onClick={() => run('terminal', () => runTerminalCommand({ command: terminalCommand }), false)}>
                <TerminalSquare className="mr-2 h-4 w-4" />
                Run
              </Button>
            </CardContent>
          </Card>
          <Card className="panel">
            <CardHeader>
              <CardTitle>STT</CardTitle>
              <CardDescription>Runs configured fast-whisper command</CardDescription>
            </CardHeader>
            <CardContent className="stack">
              <Input value={audioPath} onChange={(event) => setAudioPath(event.target.value)} placeholder="Audio file path" />
              <Button onClick={() => run('stt', () => transcribeAgentSpeech({ audio_path: audioPath }), false)}>
                <Play className="mr-2 h-4 w-4" />
                Transcribe
              </Button>
            </CardContent>
          </Card>
          <Card className="panel">
            <CardHeader>
              <CardTitle>Codex Runner</CardTitle>
              <CardDescription>Runs codex exec in workspace-write mode</CardDescription>
            </CardHeader>
            <CardContent className="stack">
              <Textarea value={codexPrompt} onChange={(event) => setCodexPrompt(event.target.value)} />
              <Button onClick={() => run('codex', () => runAgentCodex({ prompt: codexPrompt }), false)}>
                <TerminalSquare className="mr-2 h-4 w-4" />
                Run Codex
              </Button>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

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
  );
}

function EditorCard(props: {
  title: string;
  description: string;
  value: string;
  onChange: (value: string) => void;
  onSave: () => void;
  busy: boolean;
  extra?: ReactNode;
}) {
  return (
    <Card className="panel">
      <CardHeader>
        <CardTitle>{props.title}</CardTitle>
        <CardDescription>{props.description}</CardDescription>
      </CardHeader>
      <CardContent className="stack">
        <Textarea className="min-h-[280px] font-mono text-xs" value={props.value} onChange={(event) => props.onChange(event.target.value)} />
        <div className="button-row">
          <Button onClick={props.onSave} disabled={props.busy}>
            <Save className="mr-2 h-4 w-4" />
            Save
          </Button>
          {props.extra}
        </div>
      </CardContent>
    </Card>
  );
}
