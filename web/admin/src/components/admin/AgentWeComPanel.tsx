import { useEffect, useState } from 'react';
import { Image as ImageIcon, Plus, Save, Send, Trash2 } from 'lucide-react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../ui/tabs';
import { Textarea } from '../ui/textarea';
import {
  publishAgentWeComMenu,
  saveAgentSettings,
  saveAgentWeComMenu,
  sendAgentWeComImage,
  sendAgentWeComMessage,
  type AgentSnapshot,
  type AgentWeComButton,
} from '../../lib/agent';
import { Field, FieldGrid, ToggleField, numberValue, parseOptionalNumber } from './AgentFormFields';
import type { AgentRunner } from './AgentWorkspace';

type Props = {
  snapshot: AgentSnapshot;
  busy: string;
  onRun: AgentRunner;
};

type WeComSettings = AgentSnapshot['settings']['wecom'];

const newId = (prefix: string) => `${prefix}-${Date.now()}`;
const emptyButton: AgentWeComButton = { id: '', name: '', key: '', dispatch_text: '', enabled: true };

export function AgentWeComPanel({ snapshot, busy, onRun }: Props) {
  const [settings, setSettings] = useState<WeComSettings>(snapshot.settings.wecom);
  const [textMaxBytes, setTextMaxBytes] = useState(numberValue(snapshot.settings.wecom.text_max_bytes));
  const [button, setButton] = useState<AgentWeComButton>(snapshot.wecom_menu.config.buttons[0] ?? emptyButton);
  const [toUser, setToUser] = useState('');
  const [message, setMessage] = useState('');
  const [imageBase64, setImageBase64] = useState('');
  const [imageFilename, setImageFilename] = useState('');
  const [contentType, setContentType] = useState('image/png');

  useEffect(() => {
    setSettings(snapshot.settings.wecom);
    setTextMaxBytes(numberValue(snapshot.settings.wecom.text_max_bytes));
    setButton(snapshot.wecom_menu.config.buttons[0] ?? emptyButton);
  }, [snapshot]);

  const saveSettings = () => {
    onRun(
      'settings-save',
      () =>
        saveAgentSettings({
          ...snapshot.settings,
          wecom: {
            ...settings,
            text_max_bytes: parseOptionalNumber(textMaxBytes),
          },
        }),
      false,
    );
  };

  const saveMenuButton = () => {
    const id = button.id.trim() || newId('menu');
    const next = { ...button, id };
    const buttons = snapshot.wecom_menu.config.buttons.some((item) => item.id === id)
      ? snapshot.wecom_menu.config.buttons.map((item) => (item.id === id ? next : item))
      : [...snapshot.wecom_menu.config.buttons, next];
    onRun('wecom-save', () => saveAgentWeComMenu({ ...snapshot.wecom_menu.config, buttons }), false);
  };

  const deleteMenuButton = () => {
    onRun(
      'wecom-save',
      () => saveAgentWeComMenu({ ...snapshot.wecom_menu.config, buttons: snapshot.wecom_menu.config.buttons.filter((item) => item.id !== button.id) }),
      false,
    );
  };

  const validationErrors = snapshot.wecom_menu.validation_errors ?? [];
  const validationText = validationErrors.length
    ? validationErrors.join('; ')
    : 'Menu payload ready';

  return (
    <Tabs defaultValue="settings">
      <TabsList className="flex-wrap justify-start">
        <TabsTrigger value="settings">Settings</TabsTrigger>
        <TabsTrigger value="menu">Menu</TabsTrigger>
        <TabsTrigger value="message">Message</TabsTrigger>
      </TabsList>

      <TabsContent value="settings" className="grid grid--two">
        <Card className="panel">
          <CardHeader>
            <CardTitle>WeCom Settings</CardTitle>
            <CardDescription>Credentials, bridge endpoint, and message limits</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <ToggleField label="WeCom enabled" checked={settings.enabled} onChange={(enabled) => setSettings({ ...settings, enabled })} />
            <ToggleField
              label="Bridge stream enabled"
              checked={settings.bridge_stream_enabled === true}
              onChange={(bridge_stream_enabled) => setSettings({ ...settings, bridge_stream_enabled })}
            />
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
        <Card className="panel">
          <CardHeader>
            <CardTitle>Menu Buttons</CardTitle>
            <CardDescription>{snapshot.wecom_menu.config.buttons.length} top-level buttons</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <div className="button-row">
              {snapshot.wecom_menu.config.buttons.map((item) => (
                <Button key={item.id} variant={item.id === button.id ? 'default' : 'secondary'} onClick={() => setButton(item)}>
                  {item.name || item.id}
                </Button>
              ))}
              <Button variant="secondary" onClick={() => setButton({ ...emptyButton, id: newId('menu') })}>
                <Plus className="mr-2 h-4 w-4" />
                New
              </Button>
            </div>
            <ToggleField label="Button enabled" checked={button.enabled} onChange={(enabled) => setButton({ ...button, enabled })} />
            <FieldGrid>
              <Field label="ID" value={button.id} onChange={(id) => setButton({ ...button, id })} />
              <Field label="Name" value={button.name} onChange={(name) => setButton({ ...button, name })} />
              <Field label="Key" value={button.key} onChange={(key) => setButton({ ...button, key })} />
            </FieldGrid>
            <Textarea value={button.dispatch_text} onChange={(event) => setButton({ ...button, dispatch_text: event.target.value })} placeholder="Dispatch text sent to the agent" />
            <div className="button-row">
              <Button onClick={saveMenuButton} disabled={busy === 'wecom-save'}>
                <Save className="mr-2 h-4 w-4" />
                Save Button
              </Button>
              <Button variant="danger" onClick={deleteMenuButton} disabled={!button.id}>
                <Trash2 className="mr-2 h-4 w-4" />
                Delete
              </Button>
              <Button variant="secondary" onClick={() => onRun('wecom-publish', () => publishAgentWeComMenu())}>
                Publish Menu
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card className="panel">
          <CardHeader>
            <CardTitle>Menu Status</CardTitle>
            <CardDescription>{validationText}</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <div className="button-row">
              <Badge tone={snapshot.wecom_menu.config.last_published_at ? 'good' : 'neutral'}>
                {snapshot.wecom_menu.config.last_published_at ? `Published ${snapshot.wecom_menu.config.last_published_at}` : 'Not published'}
              </Badge>
              <Badge tone="neutral">{snapshot.wecom_menu.recent_events.length} events</Badge>
            </div>
            {snapshot.wecom_menu.recent_events.slice(0, 5).map((event, index) => (
              <div key={index} className="rounded-md border border-border-light p-3 text-xs text-muted-foreground">
                {Object.entries(event)
                  .map(([key, value]) => `${key}: ${String(value)}`)
                  .join(' | ')}
              </div>
            ))}
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="message" className="grid grid--two">
        <Card className="panel">
          <CardHeader>
            <CardTitle>Send Message</CardTitle>
            <CardDescription>Manual WeCom text or image delivery</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <Field label="To user" value={toUser} onChange={setToUser} />
            <Textarea value={message} onChange={(event) => setMessage(event.target.value)} placeholder="Text message" />
            <div className="button-row">
              <Button onClick={() => onRun('wecom-send', () => sendAgentWeComMessage({ to_user: toUser, text: message }))} disabled={!toUser || !message}>
                <Send className="mr-2 h-4 w-4" />
                Send Text
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card className="panel">
          <CardHeader>
            <CardTitle>Send Image</CardTitle>
            <CardDescription>Base64 image payload delivery</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <Field label="Filename" value={imageFilename} onChange={setImageFilename} />
            <Field label="Content type" value={contentType} onChange={setContentType} />
            <Textarea value={imageBase64} onChange={(event) => setImageBase64(event.target.value)} placeholder="Image base64 or data URL" />
            <Button
              variant="secondary"
              onClick={() => onRun('wecom-send', () => sendAgentWeComImage({ to_user: toUser, base64: imageBase64, filename: imageFilename || undefined, content_type: contentType || undefined }))}
              disabled={!toUser || !imageBase64}
            >
              <ImageIcon className="mr-2 h-4 w-4" />
              Send Image
            </Button>
          </CardContent>
        </Card>
      </TabsContent>
    </Tabs>
  );
}
