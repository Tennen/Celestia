import { useEffect, useState } from 'react';
import { Plus, Save, Send, Trash2 } from 'lucide-react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../ui/tabs';
import { Textarea } from '../ui/textarea';
import {
  publishAgentWeComMenu,
  saveAgentPush,
  saveAgentSettings,
  saveAgentWeComMenu,
  sendAgentWeComMessage,
  type AgentSnapshot,
  type AgentWeComButton,
} from '../../lib/agent';
import { Field, FieldGrid, SelectField, ToggleField, numberValue, parseOptionalNumber } from './AgentFormFields';
import type { AgentRunner } from './AgentWorkspace';

type Props = {
  snapshot: AgentSnapshot;
  busy: string;
  onRun: AgentRunner;
};

type WeComSettings = AgentSnapshot['settings']['wecom'];
type PushUser = Record<string, unknown>;
type PushTask = Record<string, unknown>;

const textOf = (value: unknown) => (typeof value === 'string' ? value : '');
const valueText = (value: unknown) => (typeof value === 'number' && Number.isFinite(value) ? String(value) : textOf(value));

export function AgentWeComPanel({ snapshot, busy, onRun }: Props) {
  const [settings, setSettings] = useState<WeComSettings>(snapshot.settings.wecom);
  const [textMaxBytes, setTextMaxBytes] = useState(numberValue(snapshot.settings.wecom.text_max_bytes));
  const [buttons, setButtons] = useState<AgentWeComButton[]>(snapshot.wecom_menu.config.buttons);
  const [pushUser, setPushUser] = useState<PushUser>(snapshot.push.users[0] ?? emptyPushUser());
  const [pushTask, setPushTask] = useState<PushTask>(snapshot.push.tasks[0] ?? emptyPushTask(textOf(snapshot.push.users[0]?.id)));
  const [toUser, setToUser] = useState('');
  const [message, setMessage] = useState('');

  useEffect(() => {
    setSettings(snapshot.settings.wecom);
    setTextMaxBytes(numberValue(snapshot.settings.wecom.text_max_bytes));
    setButtons(snapshot.wecom_menu.config.buttons);
    setPushUser(snapshot.push.users[0] ?? emptyPushUser());
    setPushTask(snapshot.push.tasks[0] ?? emptyPushTask(textOf(snapshot.push.users[0]?.id)));
  }, [snapshot]);

  const saveSettings = () => {
    onRun(
      'settings-save',
      () => saveAgentSettings({ ...snapshot.settings, wecom: { ...settings, text_max_bytes: parseOptionalNumber(textMaxBytes) } }),
      false,
    );
  };

  const saveMenu = () => {
    onRun('wecom-save', () => saveAgentWeComMenu({ ...snapshot.wecom_menu.config, buttons: normalizeMenuButtons(buttons) }), false);
  };

  const savePushUser = () => {
    const id = textOf(pushUser.id) || slugId(textOf(pushUser.name) || textOf(pushUser.wecom_user), 'user');
    onRun('push-save', () => saveAgentPush({ ...snapshot.push, users: replaceRecordById(snapshot.push.users, { ...pushUser, id }) }), false);
  };

  const savePushTask = () => {
    const id = textOf(pushTask.id) || slugId(textOf(pushTask.name), 'task');
    const interval = parseOptionalNumber(valueText(pushTask.interval_minutes));
    onRun(
      'push-save',
      () => saveAgentPush({ ...snapshot.push, tasks: replaceRecordById(snapshot.push.tasks, { ...pushTask, id, interval_minutes: interval }) }),
      false,
    );
  };

  const userOptions = [
    { value: '', label: 'Select recipient' },
    ...snapshot.push.users.map((user) => ({ value: textOf(user.id), label: textOf(user.name) || textOf(user.wecom_user) || textOf(user.id) })),
  ];

  return (
    <Tabs defaultValue="settings">
      <TabsList className="flex-wrap justify-start">
        <TabsTrigger value="settings">Settings</TabsTrigger>
        <TabsTrigger value="menu">Menu</TabsTrigger>
        <TabsTrigger value="push">Push</TabsTrigger>
        <TabsTrigger value="message">Message</TabsTrigger>
      </TabsList>

      <TabsContent value="settings" className="grid grid--two">
        <Card className="panel">
          <CardHeader>
            <CardTitle>WeCom Settings</CardTitle>
            <CardDescription>WeCom app, bridge, and message settings</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <ToggleField label="WeCom enabled" checked={settings.enabled} onChange={(enabled) => setSettings({ ...settings, enabled })} />
            <ToggleField label="Bridge stream enabled" checked={settings.bridge_stream_enabled === true} onChange={(bridge_stream_enabled) => setSettings({ ...settings, bridge_stream_enabled })} />
            <FieldGrid>
              <Field label="Corp ID" value={settings.corp_id ?? ''} onChange={(corp_id) => setSettings({ ...settings, corp_id })} />
              <Field label="Corp Secret" value={settings.corp_secret ?? ''} onChange={(corp_secret) => setSettings({ ...settings, corp_secret })} />
              <Field label="Agent ID" value={settings.agent_id ?? ''} onChange={(agent_id) => setSettings({ ...settings, agent_id })} />
              <Field label="Base URL" value={settings.base_url ?? ''} onChange={(base_url) => setSettings({ ...settings, base_url })} />
              <Field label="Bridge URL" value={settings.bridge_url ?? ''} onChange={(bridge_url) => setSettings({ ...settings, bridge_url })} />
              <Field label="Bridge Token" value={settings.bridge_token ?? ''} onChange={(bridge_token) => setSettings({ ...settings, bridge_token })} />
              <Field label="Audio directory" value={settings.audio_dir ?? ''} onChange={(audio_dir) => setSettings({ ...settings, audio_dir })} />
              <Field label="Text max bytes" value={textMaxBytes} onChange={setTextMaxBytes} />
            </FieldGrid>
            <Button onClick={saveSettings} disabled={busy === 'settings-save'}>
              <Save className="mr-2 h-4 w-4" />
              Save Settings
            </Button>
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="menu" className="grid grid--two">
        <Card className="panel grid__full">
          <CardHeader>
            <CardTitle>WeCom Click Menu</CardTitle>
            <CardDescription>Top-level groups can contain up to five sub-buttons; grouped parents do not dispatch clicks</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <div className="button-row">
              <Button variant="secondary" disabled={buttons.length >= 3} onClick={() => setButtons([...buttons, buildButton('root')])}>
                <Plus className="mr-2 h-4 w-4" />
                Add Top Level
              </Button>
              <Button onClick={saveMenu} disabled={busy === 'wecom-save'}>
                <Save className="mr-2 h-4 w-4" />
                Save Menu
              </Button>
              <Button variant="secondary" onClick={() => onRun('wecom-publish', () => publishAgentWeComMenu())}>
                Publish
              </Button>
              <Badge tone={snapshot.wecom_menu.validation_errors?.length ? 'bad' : 'good'}>
                {snapshot.wecom_menu.validation_errors?.length ? `${snapshot.wecom_menu.validation_errors.length} issues` : 'publishable'}
              </Badge>
            </div>
            {buttons.map((button, index) => (
              <MenuButtonEditor
                key={button.id}
                button={button}
                index={index}
                onChange={(next) => setButtons(buttons.map((item, itemIndex) => (itemIndex === index ? next : item)))}
                onDelete={() => setButtons(buttons.filter((_, itemIndex) => itemIndex !== index))}
              />
            ))}
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="push" className="grid grid--two">
        <Card className="panel">
          <CardHeader>
            <CardTitle>Push Users</CardTitle>
            <CardDescription>WeCom recipients used by scheduled push tasks</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <div className="button-row">
              {snapshot.push.users.map((user) => (
                <Button key={textOf(user.id)} variant={textOf(user.id) === textOf(pushUser.id) ? 'default' : 'secondary'} onClick={() => setPushUser(user)}>
                  {textOf(user.name) || textOf(user.wecom_user)}
                </Button>
              ))}
              <Button variant="secondary" onClick={() => setPushUser(emptyPushUser())}>
                <Plus className="mr-2 h-4 w-4" />
                New
              </Button>
            </div>
            <ToggleField label="Enabled" checked={pushUser.enabled !== false} onChange={(enabled) => setPushUser({ ...pushUser, enabled })} />
            <FieldGrid>
              <Field label="Name" value={textOf(pushUser.name)} onChange={(name) => setPushUser({ ...pushUser, name })} />
              <Field label="WeCom User" value={textOf(pushUser.wecom_user)} onChange={(wecom_user) => setPushUser({ ...pushUser, wecom_user })} />
            </FieldGrid>
            <div className="button-row">
              <Button onClick={savePushUser}>
                <Save className="mr-2 h-4 w-4" />
                Save User
              </Button>
              <Button variant="danger" disabled={!pushUser.id} onClick={() => onRun('push-save', () => saveAgentPush({ ...snapshot.push, users: snapshot.push.users.filter((user) => textOf(user.id) !== textOf(pushUser.id)) }), false)}>
                <Trash2 className="mr-2 h-4 w-4" />
                Delete
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card className="panel">
          <CardHeader>
            <CardTitle>Push Tasks</CardTitle>
            <CardDescription>Scheduled WeCom messages</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <div className="button-row">
              {snapshot.push.tasks.map((task) => (
                <Button key={textOf(task.id)} variant={textOf(task.id) === textOf(pushTask.id) ? 'default' : 'secondary'} onClick={() => setPushTask(task)}>
                  {textOf(task.name)}
                </Button>
              ))}
              <Button variant="secondary" onClick={() => setPushTask(emptyPushTask(textOf(snapshot.push.users[0]?.id)))}>
                <Plus className="mr-2 h-4 w-4" />
                New
              </Button>
            </div>
            <ToggleField label="Enabled" checked={pushTask.enabled !== false} onChange={(enabled) => setPushTask({ ...pushTask, enabled })} />
            <FieldGrid>
              <Field label="Name" value={textOf(pushTask.name)} onChange={(name) => setPushTask({ ...pushTask, name })} />
              <SelectField label="Recipient" value={textOf(pushTask.user_id)} options={userOptions} onChange={(user_id) => setPushTask({ ...pushTask, user_id })} />
              <Field label="Interval minutes" value={valueText(pushTask.interval_minutes)} onChange={(interval_minutes) => setPushTask({ ...pushTask, interval_minutes })} />
            </FieldGrid>
            <Textarea value={textOf(pushTask.text)} onChange={(event) => setPushTask({ ...pushTask, text: event.target.value })} placeholder="Push text" />
            <div className="button-row">
              <Button onClick={savePushTask} disabled={!pushTask.name || !pushTask.user_id}>
                <Save className="mr-2 h-4 w-4" />
                Save Task
              </Button>
              <Button variant="danger" disabled={!pushTask.id} onClick={() => onRun('push-save', () => saveAgentPush({ ...snapshot.push, tasks: snapshot.push.tasks.filter((task) => textOf(task.id) !== textOf(pushTask.id)) }), false)}>
                <Trash2 className="mr-2 h-4 w-4" />
                Delete
              </Button>
            </div>
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="message" className="grid grid--two">
        <Card className="panel">
          <CardHeader>
            <CardTitle>Manual Message</CardTitle>
            <CardDescription>One-off WeCom text delivery for operator checks</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <Field label="To user" value={toUser} onChange={setToUser} />
            <Textarea value={message} onChange={(event) => setMessage(event.target.value)} placeholder="Text message" />
            <Button onClick={() => onRun('wecom-send', () => sendAgentWeComMessage({ to_user: toUser, text: message }))} disabled={!toUser || !message}>
              <Send className="mr-2 h-4 w-4" />
              Send Text
            </Button>
          </CardContent>
        </Card>
      </TabsContent>
    </Tabs>
  );
}

function MenuButtonEditor(props: {
  button: AgentWeComButton;
  index: number;
  onChange: (next: AgentWeComButton) => void;
  onDelete: () => void;
}) {
  const isGroup = (props.button.sub_buttons ?? []).length > 0;
  return (
    <Card className="panel">
      <CardHeader>
        <div className="section-title section-title--inline">
          <div>
            <CardTitle>Top Level {props.index + 1}</CardTitle>
            <CardDescription>{isGroup ? 'Group menu; parent does not dispatch click events' : 'Clickable top-level menu'}</CardDescription>
          </div>
          <div className="button-row">
            <Button variant="secondary" disabled={(props.button.sub_buttons ?? []).length >= 5} onClick={() => props.onChange({ ...props.button, sub_buttons: [...(props.button.sub_buttons ?? []), buildButton('sub')] })}>
              Add Sub Menu
            </Button>
            <Button variant="danger" onClick={props.onDelete}>
              <Trash2 className="mr-2 h-4 w-4" />
              Delete
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent className="stack">
        <ToggleField label="Enabled" checked={props.button.enabled} onChange={(enabled) => props.onChange({ ...props.button, enabled })} />
        <Field label="Menu name" value={props.button.name} onChange={(name) => props.onChange({ ...props.button, name })} />
        {!isGroup ? <LeafFields button={props.button} onChange={props.onChange} /> : null}
        {(props.button.sub_buttons ?? []).map((subButton, subIndex) => (
          <div key={subButton.id} className="stack rounded-md border border-border-light p-3">
            <div className="section-title section-title--inline">
              <strong>Sub Menu {props.index + 1}.{subIndex + 1}</strong>
              <Button variant="danger" onClick={() => props.onChange({ ...props.button, sub_buttons: (props.button.sub_buttons ?? []).filter((_, index) => index !== subIndex) })}>
                <Trash2 className="mr-2 h-4 w-4" />
                Delete
              </Button>
            </div>
            <ToggleField label="Enabled" checked={subButton.enabled} onChange={(enabled) => updateSubButton(props, subIndex, { enabled })} />
            <Field label="Menu name" value={subButton.name} onChange={(name) => updateSubButton(props, subIndex, { name })} />
            <LeafFields button={subButton} onChange={(next) => updateSubButton(props, subIndex, next)} />
          </div>
        ))}
      </CardContent>
    </Card>
  );
}

function LeafFields(props: { button: AgentWeComButton; onChange: (next: AgentWeComButton) => void }) {
  return (
    <>
      <Field label="EventKey" value={props.button.key} onChange={(key) => props.onChange({ ...props.button, key })} />
      <Textarea value={props.button.dispatch_text} onChange={(event) => props.onChange({ ...props.button, dispatch_text: event.target.value })} placeholder="Text sent into the Agent, e.g. /market close" />
    </>
  );
}

function updateSubButton(props: { button: AgentWeComButton; onChange: (next: AgentWeComButton) => void }, subIndex: number, patch: Partial<AgentWeComButton>) {
  props.onChange({
    ...props.button,
    sub_buttons: (props.button.sub_buttons ?? []).map((item, index) => (index === subIndex ? { ...item, ...patch } : item)),
  });
}

function normalizeMenuButtons(buttons: AgentWeComButton[]) {
  return buttons.map((button) => ({
    ...button,
    id: button.id || slugId(button.name, 'menu'),
    key: (button.sub_buttons ?? []).length > 0 ? '' : button.key,
    dispatch_text: (button.sub_buttons ?? []).length > 0 ? '' : button.dispatch_text,
    sub_buttons: (button.sub_buttons ?? []).map((subButton) => ({ ...subButton, id: subButton.id || slugId(subButton.name, 'submenu') })),
  }));
}

function buildButton(prefix: string): AgentWeComButton {
  return { id: `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`, name: '', key: '', enabled: true, dispatch_text: '' };
}

function emptyPushUser(): PushUser {
  return { id: '', name: '', wecom_user: '', enabled: true };
}

function emptyPushTask(userID: string): PushTask {
  return { id: '', name: '', user_id: userID, text: '', interval_minutes: 60, enabled: true };
}

function replaceRecordById(items: Array<Record<string, unknown>>, next: Record<string, unknown>) {
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
