import { useMemo } from 'react';
import { getCatalogDefaultConfig, mergeCatalogDefaultConfig } from '../../lib/admin';
import { prettyJson } from '../../lib/utils';
import type { CatalogPlugin } from '../../lib/types';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { ScrollArea } from '../ui/scroll-area';
import { Textarea } from '../ui/textarea';

type Props = {
  plugin: CatalogPlugin;
  runtimeInstalled: boolean;
  pluginDraft: string;
  busy: string;
  onDraftChange: (value: string) => void;
  onInstall: () => void;
  onSaveConfig: () => void;
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export function PluginConfigPanel({
  plugin,
  runtimeInstalled,
  pluginDraft,
  busy,
  onDraftChange,
  onInstall,
  onSaveConfig,
}: Props) {
  const defaultConfig = useMemo(() => getCatalogDefaultConfig(plugin), [plugin]);
  const properties = useMemo(() => {
    const schema = plugin.manifest.config_schema;
    if (!isRecord(schema) || !isRecord(schema.properties)) {
      return [] as Array<{ key: string; description: string }>;
    }
    return Object.entries(schema.properties)
      .map(([key, value]) => ({
        key,
        description: isRecord(value) && typeof value.description === 'string' ? value.description : '',
      }))
      .sort((a, b) => a.key.localeCompare(b.key, 'en'));
  }, [plugin]);

  const defaultDraft = useMemo(() => JSON.stringify(defaultConfig, null, 2), [defaultConfig]);
  const mergedDefaultDraft = useMemo(() => {
    try {
      return JSON.stringify(mergeCatalogDefaultConfig(plugin, JSON.parse(pluginDraft) as Record<string, unknown>), null, 2);
    } catch {
      return defaultDraft;
    }
  }, [defaultDraft, plugin, pluginDraft]);

  return (
    <div className="detail">
      <Card>
        <CardHeader>
          <CardTitle>Plugin Config</CardTitle>
          <CardDescription>
            This draft is owned by Core. Defaults come from the catalog schema that the gateway exposes, not from a
            frontend-only preset.
          </CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <div className="button-row">
            <Button variant="secondary" onClick={() => onDraftChange(mergedDefaultDraft)}>
              Fill Missing Fields From Core Default
            </Button>
            {!runtimeInstalled ? (
              <Button variant="secondary" onClick={() => onDraftChange(defaultDraft)}>
                Reset To Pure Core Default
              </Button>
            ) : null}
            {runtimeInstalled ? (
              <Button onClick={onSaveConfig} disabled={busy === `refresh-config-${plugin.id}`}>
                Save Config
              </Button>
            ) : (
              <Button onClick={onInstall} disabled={busy === `install-${plugin.id}`}>
                Install With Draft
              </Button>
            )}
          </div>
          <div>
            <label>Editable Config JSON</label>
            <Textarea rows={20} value={pluginDraft} onChange={(event) => onDraftChange(event.target.value)} />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Core Default Baseline</CardTitle>
          <CardDescription>
            Use this as the starting point when a vendor changes app signatures, base URLs, or login headers.
          </CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <ScrollArea className="max-h-[420px]">
            <pre className="log-box">{prettyJson(defaultConfig)}</pre>
          </ScrollArea>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Schema Fields</CardTitle>
          <CardDescription>High-level fields that Core advertises for this plugin configuration.</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          {properties.length > 0 ? (
            <ScrollArea className="max-h-[420px] pr-4">
              <div className="config-field-list">
                {properties.map((property) => (
                  <div key={property.key} className="config-field-list__item">
                    <strong>{property.key}</strong>
                    <p className="muted">{property.description || 'No description from schema.'}</p>
                  </div>
                ))}
              </div>
            </ScrollArea>
          ) : (
            <p className="muted">No schema metadata available for this plugin.</p>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
