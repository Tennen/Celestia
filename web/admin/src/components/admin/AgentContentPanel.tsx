import { useEffect, useMemo, useState } from 'react';
import { Play, Plus, Save, Trash2 } from 'lucide-react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../ui/tabs';
import { Textarea } from '../ui/textarea';
import {
  addWritingMaterial,
  createWritingTopic,
  importMarketPortfolioCodes,
  runAgentTopic,
  runMarketAnalysis,
  saveAgentTopic,
  saveMarketPortfolio,
  summarizeWritingTopic,
  type AgentMarketPortfolio,
  type AgentSnapshot,
} from '../../lib/agent';
import { Field, FieldGrid, ToggleField, numberValue, parseOptionalNumber, requiredNumber } from './AgentFormFields';
import type { AgentRunner } from './AgentWorkspace';

type Props = {
  snapshot: AgentSnapshot;
  busy: string;
  onRun: AgentRunner;
};

type TopicProfile = Record<string, unknown> & { id?: string; name?: string; sources?: TopicSource[] };
type TopicSource = Record<string, unknown> & {
  id?: string;
  name?: string;
  category?: string;
  feed_url?: string;
  weight?: number;
  enabled?: boolean;
};
type Holding = AgentMarketPortfolio['funds'][number];

const newId = (prefix: string) => `${prefix}-${Date.now()}`;
const textOf = (value: unknown) => (typeof value === 'string' ? value : '');
const boolOf = (value: unknown) => value === true;
const numOf = (value: unknown) => (typeof value === 'number' && Number.isFinite(value) ? value : undefined);

export function AgentContentPanel({ snapshot, busy, onRun }: Props) {
  const firstProfile = snapshot.topic_summary.profiles[0] as TopicProfile | undefined;
  const [profileId, setProfileId] = useState(snapshot.topic_summary.active_profile_id || firstProfile?.id || '');
  const [profileDraft, setProfileDraft] = useState<TopicProfile>(firstProfile ?? { id: '', name: '', sources: [] });
  const [sourceDraft, setSourceDraft] = useState<TopicSource>({ id: '', name: '', category: '', feed_url: '', weight: 1, enabled: true });
  const [writingTopicId, setWritingTopicId] = useState(snapshot.writing.topics[0]?.id ?? '');
  const [writingTitle, setWritingTitle] = useState('');
  const [materialTitle, setMaterialTitle] = useState('');
  const [materialContent, setMaterialContent] = useState('');
  const [cash, setCash] = useState(numberValue(snapshot.market.portfolio.cash));
  const [holding, setHolding] = useState<Holding>(snapshot.market.portfolio.funds[0] ?? { code: '', name: '' });
  const [marketCodes, setMarketCodes] = useState('');
  const [marketPhase, setMarketPhase] = useState('close');
  const [marketNotes, setMarketNotes] = useState('');

  const activeProfile = useMemo(
    () => (snapshot.topic_summary.profiles.find((item) => textOf(item.id) === profileId) as TopicProfile | undefined) ?? firstProfile,
    [firstProfile, profileId, snapshot.topic_summary.profiles],
  );
  const activeWritingTopic = snapshot.writing.topics.find((item) => item.id === writingTopicId) ?? snapshot.writing.topics[0];

  useEffect(() => {
    const nextProfile = ((snapshot.topic_summary.profiles.find((item) => textOf(item.id) === snapshot.topic_summary.active_profile_id) ??
      snapshot.topic_summary.profiles[0]) as TopicProfile | undefined) ?? { id: '', name: '', sources: [] };
    setProfileId(textOf(nextProfile.id));
    setProfileDraft(nextProfile);
    setSourceDraft((nextProfile.sources ?? [])[0] ?? { id: '', name: '', category: '', feed_url: '', weight: 1, enabled: true });
    setWritingTopicId(snapshot.writing.topics[0]?.id ?? '');
    setCash(numberValue(snapshot.market.portfolio.cash));
    setHolding(snapshot.market.portfolio.funds[0] ?? { code: '', name: '' });
  }, [snapshot]);

  const saveProfile = () => {
    const id = textOf(profileDraft.id).trim() || newId('profile');
    const next = { ...profileDraft, id, sources: profileDraft.sources ?? [] };
    const profiles = replaceRecord(snapshot.topic_summary.profiles, next);
    onRun(
      'topic-save',
      () => saveAgentTopic({ ...snapshot.topic_summary, active_profile_id: snapshot.topic_summary.active_profile_id || id, profiles }),
      false,
    );
  };

  const deleteProfile = () => {
    const profiles = snapshot.topic_summary.profiles.filter((item) => textOf(item.id) !== textOf(profileDraft.id));
    onRun('topic-save', () => saveAgentTopic({ ...snapshot.topic_summary, active_profile_id: textOf(profiles[0]?.id), profiles }), false);
  };

  const saveSource = () => {
    const selected = activeProfile ?? profileDraft;
    const profileID = textOf(selected.id);
    const sourceID = textOf(sourceDraft.id).trim() || newId('source');
    const nextSource = { ...sourceDraft, id: sourceID, weight: numOf(sourceDraft.weight) ?? 1, enabled: sourceDraft.enabled !== false };
    const profiles = snapshot.topic_summary.profiles.map((item) => {
      if (textOf(item.id) !== profileID) return item;
      const profile = item as TopicProfile;
      return { ...profile, sources: replaceRecord(profile.sources ?? [], nextSource) };
    });
    onRun('topic-save', () => saveAgentTopic({ ...snapshot.topic_summary, profiles }), false);
  };

  const deleteSource = () => {
    const selected = activeProfile ?? profileDraft;
    const sourceID = textOf(sourceDraft.id);
    const profiles = snapshot.topic_summary.profiles.map((item) => {
      if (textOf(item.id) !== textOf(selected.id)) return item;
      const profile = item as TopicProfile;
      return { ...profile, sources: (profile.sources ?? []).filter((source) => textOf(source.id) !== sourceID) };
    });
    onRun('topic-save', () => saveAgentTopic({ ...snapshot.topic_summary, profiles }), false);
  };

  const saveHolding = () => {
    const nextHolding: Holding = {
      code: holding.code.trim(),
      name: holding.name.trim(),
      quantity: parseOptionalNumber(String(holding.quantity ?? '')),
      avg_cost: parseOptionalNumber(String(holding.avg_cost ?? '')),
    };
    const funds = snapshot.market.portfolio.funds.some((item) => item.code === nextHolding.code)
      ? snapshot.market.portfolio.funds.map((item) => (item.code === nextHolding.code ? nextHolding : item))
      : [...snapshot.market.portfolio.funds, nextHolding];
    onRun('market-save', () => saveMarketPortfolio({ ...snapshot.market.portfolio, cash: requiredNumber(cash), funds }), false);
  };

  const deleteHolding = () => {
    onRun(
      'market-save',
      () => saveMarketPortfolio({ ...snapshot.market.portfolio, funds: snapshot.market.portfolio.funds.filter((item) => item.code !== holding.code) }),
      false,
    );
  };

  return (
    <Tabs defaultValue="topic">
      <TabsList className="flex-wrap justify-start">
        <TabsTrigger value="topic">Topic Summary</TabsTrigger>
        <TabsTrigger value="writing">Writing</TabsTrigger>
        <TabsTrigger value="market">Market</TabsTrigger>
      </TabsList>

      <TabsContent value="topic" className="grid grid--two">
        <Card className="panel">
          <CardHeader>
            <CardTitle>Topic Profiles</CardTitle>
            <CardDescription>{snapshot.topic_summary.profiles.length} profiles, {snapshot.topic_summary.runs.length} runs</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <div className="button-row">
              {snapshot.topic_summary.profiles.map((item) => (
                <Button
                  key={textOf(item.id)}
                  variant={textOf(item.id) === profileId ? 'default' : 'secondary'}
                  onClick={() => {
                    const profile = item as TopicProfile;
                    setProfileId(textOf(profile.id));
                    setProfileDraft(profile);
                    setSourceDraft((profile.sources ?? [])[0] ?? { id: '', name: '', category: '', feed_url: '', weight: 1, enabled: true });
                  }}
                >
                  {textOf(item.name) || textOf(item.id)}
                </Button>
              ))}
              <Button variant="secondary" onClick={() => setProfileDraft({ id: newId('profile'), name: '', sources: [] })}>
                <Plus className="mr-2 h-4 w-4" />
                New
              </Button>
            </div>
            <FieldGrid>
              <Field label="Profile ID" value={textOf(profileDraft.id)} onChange={(id) => setProfileDraft({ ...profileDraft, id })} />
              <Field label="Name" value={textOf(profileDraft.name)} onChange={(name) => setProfileDraft({ ...profileDraft, name })} />
            </FieldGrid>
            <div className="button-row">
              <Button onClick={saveProfile} disabled={busy === 'topic-save'}>
                <Save className="mr-2 h-4 w-4" />
                Save Profile
              </Button>
              <Button variant="secondary" disabled={!profileId} onClick={() => onRun('topic-run', () => runAgentTopic(profileId))}>
                <Play className="mr-2 h-4 w-4" />
                Run Profile
              </Button>
              <Button variant="danger" onClick={deleteProfile} disabled={!profileDraft.id}>
                <Trash2 className="mr-2 h-4 w-4" />
                Delete
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card className="panel">
          <CardHeader>
            <CardTitle>Topic Sources</CardTitle>
            <CardDescription>{activeProfile?.sources?.length ?? 0} RSS sources for {activeProfile?.name ?? 'profile'}</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <div className="button-row">
              {(activeProfile?.sources ?? []).map((source) => (
                <Button key={textOf(source.id)} variant={textOf(source.id) === textOf(sourceDraft.id) ? 'default' : 'secondary'} onClick={() => setSourceDraft(source)}>
                  {textOf(source.name) || textOf(source.id)}
                </Button>
              ))}
              <Button variant="secondary" onClick={() => setSourceDraft({ id: newId('source'), name: '', category: '', feed_url: '', weight: 1, enabled: true })}>
                <Plus className="mr-2 h-4 w-4" />
                New
              </Button>
            </div>
            <ToggleField label="Source enabled" checked={sourceDraft.enabled !== false} onChange={(enabled) => setSourceDraft({ ...sourceDraft, enabled })} />
            <FieldGrid>
              <Field label="Source ID" value={textOf(sourceDraft.id)} onChange={(id) => setSourceDraft({ ...sourceDraft, id })} />
              <Field label="Name" value={textOf(sourceDraft.name)} onChange={(name) => setSourceDraft({ ...sourceDraft, name })} />
              <Field label="Category" value={textOf(sourceDraft.category)} onChange={(category) => setSourceDraft({ ...sourceDraft, category })} />
              <Field label="Feed URL" value={textOf(sourceDraft.feed_url)} onChange={(feed_url) => setSourceDraft({ ...sourceDraft, feed_url })} />
              <Field label="Weight" value={String(sourceDraft.weight ?? '')} onChange={(weight) => setSourceDraft({ ...sourceDraft, weight: requiredNumber(weight, 1) })} />
            </FieldGrid>
            <div className="button-row">
              <Button onClick={saveSource} disabled={!activeProfile}>
                <Save className="mr-2 h-4 w-4" />
                Save Source
              </Button>
              <Button variant="danger" onClick={deleteSource} disabled={!sourceDraft.id}>
                <Trash2 className="mr-2 h-4 w-4" />
                Delete
              </Button>
            </div>
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="writing" className="grid grid--two">
        <Card className="panel">
          <CardHeader>
            <CardTitle>Writing Organizer</CardTitle>
            <CardDescription>{snapshot.writing.topics.length} topics</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <div className="button-row">
              {snapshot.writing.topics.map((topic) => (
                <Button key={topic.id} variant={topic.id === writingTopicId ? 'default' : 'secondary'} onClick={() => setWritingTopicId(topic.id)}>
                  {topic.title}
                </Button>
              ))}
            </div>
            <Field label="New topic title" value={writingTitle} onChange={setWritingTitle} />
            <Button onClick={() => onRun('writing', () => createWritingTopic({ title: writingTitle }))} disabled={!writingTitle.trim()}>
              <Plus className="mr-2 h-4 w-4" />
              Create Topic
            </Button>
            <Field label="Material title" value={materialTitle} onChange={setMaterialTitle} />
            <Textarea value={materialContent} onChange={(event) => setMaterialContent(event.target.value)} placeholder="Material content" />
            <div className="button-row">
              <Button disabled={!activeWritingTopic || !materialContent.trim()} onClick={() => onRun('writing', () => addWritingMaterial(activeWritingTopic!.id, { title: materialTitle, content: materialContent }))}>
                Add Material
              </Button>
              <Button variant="secondary" disabled={!activeWritingTopic} onClick={() => onRun('writing', () => summarizeWritingTopic(activeWritingTopic!.id))}>
                Summarize
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card className="panel">
          <CardHeader>
            <CardTitle>{activeWritingTopic?.title ?? 'No topic'}</CardTitle>
            <CardDescription>{activeWritingTopic ? `${activeWritingTopic.materials.length} materials, ${activeWritingTopic.status}` : 'Create a topic to begin'}</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <Badge tone={activeWritingTopic?.last_summarized_at ? 'good' : 'neutral'}>
              {activeWritingTopic?.last_summarized_at ? `Summarized ${activeWritingTopic.last_summarized_at}` : 'Not summarized'}
            </Badge>
            <Textarea readOnly value={activeWritingTopic?.state?.summary ?? ''} placeholder="Summary" />
            <Textarea readOnly value={activeWritingTopic?.state?.outline ?? ''} placeholder="Outline" />
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="market" className="grid grid--two">
        <Card className="panel">
          <CardHeader>
            <CardTitle>Market Portfolio</CardTitle>
            <CardDescription>{snapshot.market.portfolio.funds.length} holdings, {snapshot.market.runs.length} runs</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <Field label="Cash" value={cash} onChange={setCash} />
            <div className="button-row">
              {snapshot.market.portfolio.funds.map((item) => (
                <Button key={item.code} variant={item.code === holding.code ? 'default' : 'secondary'} onClick={() => setHolding(item)}>
                  {item.name || item.code}
                </Button>
              ))}
              <Button variant="secondary" onClick={() => setHolding({ code: '', name: '' })}>
                <Plus className="mr-2 h-4 w-4" />
                New
              </Button>
            </div>
            <FieldGrid>
              <Field label="Code" value={holding.code} onChange={(code) => setHolding({ ...holding, code })} />
              <Field label="Name" value={holding.name} onChange={(name) => setHolding({ ...holding, name })} />
              <Field label="Quantity" value={String(holding.quantity ?? '')} onChange={(quantity) => setHolding({ ...holding, quantity: parseOptionalNumber(quantity) })} />
              <Field label="Average cost" value={String(holding.avg_cost ?? '')} onChange={(avg_cost) => setHolding({ ...holding, avg_cost: parseOptionalNumber(avg_cost) })} />
            </FieldGrid>
            <div className="button-row">
              <Button onClick={saveHolding} disabled={!holding.code.trim()}>
                <Save className="mr-2 h-4 w-4" />
                Save Holding
              </Button>
              <Button variant="danger" onClick={deleteHolding} disabled={!holding.code}>
                <Trash2 className="mr-2 h-4 w-4" />
                Delete
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card className="panel">
          <CardHeader>
            <CardTitle>Market Run</CardTitle>
            <CardDescription>Import codes or run an analysis phase</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <Textarea value={marketCodes} onChange={(event) => setMarketCodes(event.target.value)} placeholder="Fund or security codes, separated by commas or lines" />
            <Button variant="secondary" onClick={() => onRun('market-import', () => importMarketPortfolioCodes({ codes: marketCodes }))} disabled={!marketCodes.trim()}>
              Import Codes
            </Button>
            <Field label="Phase" value={marketPhase} onChange={setMarketPhase} />
            <Textarea value={marketNotes} onChange={(event) => setMarketNotes(event.target.value)} placeholder="Run notes" />
            <Button onClick={() => onRun('market-run', () => runMarketAnalysis({ phase: marketPhase, notes: marketNotes }))}>
              <Play className="mr-2 h-4 w-4" />
              Run Analysis
            </Button>
          </CardContent>
        </Card>
      </TabsContent>
    </Tabs>
  );
}

function replaceRecord<T extends Record<string, unknown>>(items: T[], next: T) {
  const id = textOf(next.id);
  return items.some((item) => textOf(item.id) === id) ? items.map((item) => (textOf(item.id) === id ? next : item)) : [...items, next];
}
