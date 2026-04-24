import { useEffect, useState } from 'react';
import { Plus } from 'lucide-react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Textarea } from '../ui/textarea';
import { addWritingMaterial, createWritingTopic, summarizeWritingTopic, type AgentSnapshot } from '../../lib/agent';
import { Field } from './AgentFormFields';
import type { AgentRunner } from './AgentWorkspace';
import { SelectableListItem } from './shared/SelectableListItem';

type Props = {
  snapshot: AgentSnapshot;
  busy: string;
  onRun: AgentRunner;
};

export function AgentWritingPanel({ snapshot, busy, onRun }: Props) {
  const [writingTopicId, setWritingTopicId] = useState(snapshot.writing.topics[0]?.id ?? '');
  const [writingTitle, setWritingTitle] = useState('');
  const [materialTitle, setMaterialTitle] = useState('');
  const [materialContent, setMaterialContent] = useState('');

  const activeWritingTopic = snapshot.writing.topics.find((item) => item.id === writingTopicId) ?? snapshot.writing.topics[0];

  useEffect(() => {
    setWritingTopicId(snapshot.writing.topics[0]?.id ?? '');
  }, [snapshot.writing.topics]);

  return (
    <div className="agent-tab-content grid grid--two">
      <Card className="panel">
        <CardHeader>
          <CardTitle>Writing Organizer</CardTitle>
          <CardDescription>{snapshot.writing.topics.length} topics</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <div className="list-stack">
            {snapshot.writing.topics.map((topic) => (
              <SelectableListItem
                key={topic.id}
                title={topic.title}
                description={`${topic.materials.length} materials`}
                selected={topic.id === writingTopicId}
                badges={
                  <Badge tone={topic.status === 'done' ? 'good' : 'neutral'} size="xxs">
                    {topic.status}
                  </Badge>
                }
                onClick={() => setWritingTopicId(topic.id)}
              />
            ))}
            {snapshot.writing.topics.length === 0 ? <div className="detail">No writing topics configured.</div> : null}
          </div>
          <Field label="New topic title" value={writingTitle} onChange={setWritingTitle} />
          <Button onClick={() => onRun('writing', () => createWritingTopic({ title: writingTitle }))} disabled={!writingTitle.trim() || busy === 'writing'}>
            <Plus className="mr-2 h-4 w-4" />
            Create Topic
          </Button>
          <Field label="Material title" value={materialTitle} onChange={setMaterialTitle} />
          <Textarea value={materialContent} onChange={(event) => setMaterialContent(event.target.value)} placeholder="Material content" />
          <div className="button-row">
            <Button
              disabled={!activeWritingTopic || !materialContent.trim() || busy === 'writing'}
              onClick={() => onRun('writing', () => addWritingMaterial(activeWritingTopic!.id, { title: materialTitle, content: materialContent }))}
            >
              Add Material
            </Button>
            <Button
              variant="secondary"
              disabled={!activeWritingTopic || busy === 'writing'}
              onClick={() => onRun('writing', () => summarizeWritingTopic(activeWritingTopic!.id))}
            >
              Summarize
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card className="panel">
        <CardHeader>
          <CardTitle>{activeWritingTopic?.title ?? 'No topic'}</CardTitle>
          <CardDescription>
            {activeWritingTopic ? `${activeWritingTopic.materials.length} materials, ${activeWritingTopic.status}` : 'Create a topic to begin'}
          </CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <Badge tone={activeWritingTopic?.last_summarized_at ? 'good' : 'neutral'}>
            {activeWritingTopic?.last_summarized_at ? `Summarized ${activeWritingTopic.last_summarized_at}` : 'Not summarized'}
          </Badge>
          <Textarea readOnly value={activeWritingTopic?.state?.summary ?? ''} placeholder="Summary" />
          <Textarea readOnly value={activeWritingTopic?.state?.outline ?? ''} placeholder="Outline" />
        </CardContent>
      </Card>
    </div>
  );
}
