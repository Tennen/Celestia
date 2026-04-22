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
  runAgentTopic,
  saveAgentTopic,
  summarizeWritingTopic,
  type AgentSnapshot,
} from '../../lib/agent';
import { Field, FieldGrid, ToggleField, requiredNumber } from './AgentFormFields';
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

const textOf = (value: unknown) => (typeof value === 'string' ? value : '');
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
  }, [snapshot]);

  const saveProfile = () => {
    const id = textOf(profileDraft.id) || slugId(textOf(profileDraft.name), 'profile');
    const next = { ...profileDraft, id, sources: profileDraft.sources ?? [] };
    const profiles = replaceRecord(snapshot.topic_summary.profiles, next);
    onRun('topic-save', () => saveAgentTopic({ ...snapshot.topic_summary, active_profile_id: snapshot.topic_summary.active_profile_id || id, profiles }), false);
  };

  const saveSource = () => {
    const selected = activeProfile ?? profileDraft;
    const sourceID = textOf(sourceDraft.id) || slugId(textOf(sourceDraft.name), 'source');
    const nextSource = { ...sourceDraft, id: sourceID, weight: numOf(sourceDraft.weight) ?? 1, enabled: sourceDraft.enabled !== false };
    const profiles = snapshot.topic_summary.profiles.map((item) => {
      if (textOf(item.id) !== textOf(selected.id)) return item;
      const profile = item as TopicProfile;
      return { ...profile, sources: replaceRecord(profile.sources ?? [], nextSource) };
    });
    onRun('topic-save', () => saveAgentTopic({ ...snapshot.topic_summary, profiles }), false);
  };

  return (
    <Tabs defaultValue="topic">
      <TabsList className="flex-wrap justify-start">
        <TabsTrigger value="topic">Topic Summary</TabsTrigger>
        <TabsTrigger value="writing">Writing</TabsTrigger>
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
              <Button variant="secondary" onClick={() => setProfileDraft({ id: '', name: '', sources: [] })}>
                <Plus className="mr-2 h-4 w-4" />
                New
              </Button>
            </div>
            <Field label="Name" value={textOf(profileDraft.name)} onChange={(name) => setProfileDraft({ ...profileDraft, name })} />
            <div className="button-row">
              <Button onClick={saveProfile} disabled={busy === 'topic-save' || !profileDraft.name}>
                <Save className="mr-2 h-4 w-4" />
                Save Profile
              </Button>
              <Button variant="secondary" disabled={!profileId} onClick={() => onRun('topic-run', () => runAgentTopic(profileId))}>
                <Play className="mr-2 h-4 w-4" />
                Run Profile
              </Button>
              <Button variant="danger" disabled={!profileDraft.id} onClick={() => onRun('topic-save', () => saveAgentTopic({ ...snapshot.topic_summary, profiles: snapshot.topic_summary.profiles.filter((item) => textOf(item.id) !== textOf(profileDraft.id)) }), false)}>
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
              <Button variant="secondary" onClick={() => setSourceDraft({ id: '', name: '', category: '', feed_url: '', weight: 1, enabled: true })}>
                <Plus className="mr-2 h-4 w-4" />
                New
              </Button>
            </div>
            <ToggleField label="Source enabled" checked={sourceDraft.enabled !== false} onChange={(enabled) => setSourceDraft({ ...sourceDraft, enabled })} />
            <FieldGrid>
              <Field label="Name" value={textOf(sourceDraft.name)} onChange={(name) => setSourceDraft({ ...sourceDraft, name })} />
              <Field label="Category" value={textOf(sourceDraft.category)} onChange={(category) => setSourceDraft({ ...sourceDraft, category })} />
              <Field label="Feed URL" value={textOf(sourceDraft.feed_url)} onChange={(feed_url) => setSourceDraft({ ...sourceDraft, feed_url })} />
              <Field label="Weight" value={String(sourceDraft.weight ?? '')} onChange={(weight) => setSourceDraft({ ...sourceDraft, weight: requiredNumber(weight, 1) })} />
            </FieldGrid>
            <div className="button-row">
              <Button onClick={saveSource} disabled={!activeProfile || !sourceDraft.name || !sourceDraft.feed_url}>
                <Save className="mr-2 h-4 w-4" />
                Save Source
              </Button>
              <Button variant="danger" disabled={!sourceDraft.id} onClick={() => deleteSource(snapshot, activeProfile, sourceDraft, onRun)}>
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
    </Tabs>
  );
}

function deleteSource(snapshot: AgentSnapshot, activeProfile: TopicProfile | undefined, sourceDraft: TopicSource, onRun: AgentRunner) {
  const profiles = snapshot.topic_summary.profiles.map((item) => {
    if (textOf(item.id) !== textOf(activeProfile?.id)) return item;
    const profile = item as TopicProfile;
    return { ...profile, sources: (profile.sources ?? []).filter((source) => textOf(source.id) !== textOf(sourceDraft.id)) };
  });
  onRun('topic-save', () => saveAgentTopic({ ...snapshot.topic_summary, profiles }), false);
}

function replaceRecord<T extends Record<string, unknown>>(items: T[], next: T) {
  const id = textOf(next.id);
  return items.some((item) => textOf(item.id) === id) ? items.map((item) => (textOf(item.id) === id ? next : item)) : [...items, next];
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
