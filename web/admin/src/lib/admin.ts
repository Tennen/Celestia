import type {
  AdminStreamFrame,
  Automation,
  AuditRecord,
  CapabilitySummary,
  CatalogPlugin,
  DashboardSummary,
  DeviceView,
  EventRecord,
  PluginRuntimeView,
} from './types';

const RECENT_ACTIVITY_LIMIT = 80;

export type StatusBanner = {
  tone: 'good' | 'warn' | 'bad';
  text: string;
};

export type AppSection = 'overview' | 'plugins' | 'workflow' | 'agent' | 'touchpoints' | 'capabilities' | 'devices' | 'activity';

export type LoadState = {
  dashboard: DashboardSummary | null;
  catalog: CatalogPlugin[];
  plugins: PluginRuntimeView[];
  capabilities: CapabilitySummary[];
  automations: Automation[];
  devices: DeviceView[];
  events: EventRecord[];
  audits: AuditRecord[];
  loading: boolean;
  refreshing: boolean;
  hasLoaded: boolean;
  error: string | null;
};

export type RemoteLoadState = Pick<
  LoadState,
  'dashboard' | 'catalog' | 'plugins' | 'capabilities' | 'automations' | 'devices' | 'events' | 'audits'
>;

export const emptyLoadState = (): LoadState => ({
  dashboard: null,
  catalog: [],
  plugins: [],
  capabilities: [],
  automations: [],
  devices: [],
  events: [],
  audits: [],
  loading: true,
  refreshing: false,
  hasLoaded: false,
  error: null,
});

export function asArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : [];
}

export function compareText(a: string, b: string) {
  return a.localeCompare(b, 'en');
}

export function sortPlugins(items: PluginRuntimeView[]) {
  return asArray(items).sort((a, b) => compareText(a.record.plugin_id, b.record.plugin_id));
}

export function sortCapabilities(items: CapabilitySummary[]) {
  return asArray(items).sort((a, b) => compareText(a.name || a.id, b.name || b.id));
}

export function sortAutomations(items: Automation[]) {
  return asArray(items).sort((a, b) => compareText(a.name || a.id, b.name || b.id));
}

export function sortDevices(items: DeviceView[]) {
  return asArray(items).sort((a, b) => compareText(a.device.name || a.device.id, b.device.name || b.device.id));
}

function isJsonEqual(left: unknown, right: unknown) {
  return JSON.stringify(left) === JSON.stringify(right);
}

function reuseEqualValue<T>(current: T, next: T) {
  return isJsonEqual(current, next) ? current : next;
}

function mergeStableList<T>(current: T[], next: T[], getKey: (item: T) => string) {
  if (!current.length) {
    return next;
  }

  const indexed = new Map(current.map((item) => [getKey(item), item]));
  let changed = current.length !== next.length;
  const merged = next.map((item) => {
    const previous = indexed.get(getKey(item));
    if (previous && isJsonEqual(previous, item)) {
      return previous;
    }
    changed = true;
    return item;
  });

  if (!changed) {
    for (let index = 0; index < current.length; index += 1) {
      if (current[index] !== merged[index]) {
        changed = true;
        break;
      }
    }
  }

  return changed ? merged : current;
}

export function mergeLoadStateData(current: LoadState, next: RemoteLoadState): RemoteLoadState {
  return {
    dashboard: reuseEqualValue(current.dashboard, next.dashboard),
    catalog: mergeStableList(current.catalog, next.catalog, (item) => item.id),
    plugins: mergeStableList(current.plugins, next.plugins, (item) => item.record.plugin_id),
    capabilities: mergeStableList(current.capabilities, next.capabilities, (item) => item.id),
    automations: mergeStableList(current.automations, next.automations, (item) => item.id),
    devices: mergeStableList(current.devices, next.devices, (item) => item.device.id),
    events: mergeStableList(current.events, next.events, (item) => item.id),
    audits: mergeStableList(current.audits, next.audits, (item) => item.id),
  };
}

function prependStableItem<T>(current: T[], item: T, getKey: (value: T) => string, limit: number) {
  const itemKey = getKey(item);
  const next = [item, ...current.filter((value) => getKey(value) !== itemKey)];
  return next.slice(0, limit);
}

export function applyAdminStreamFrame(current: LoadState, frame: AdminStreamFrame): RemoteLoadState {
  return {
    dashboard: frame.dashboard ?? current.dashboard,
    catalog: current.catalog,
    plugins: frame.plugins ? sortPlugins(frame.plugins) : current.plugins,
    capabilities: frame.capabilities ? sortCapabilities(frame.capabilities) : current.capabilities,
    automations: frame.automations ? sortAutomations(frame.automations) : current.automations,
    devices: frame.devices ? sortDevices(frame.devices) : current.devices,
    events: frame.events
      ? asArray(frame.events)
      : frame.event
        ? prependStableItem(current.events, frame.event, (item) => item.id, RECENT_ACTIVITY_LIMIT)
        : current.events,
    audits: frame.audits
      ? asArray(frame.audits)
      : frame.audit
        ? prependStableItem(current.audits, frame.audit, (item) => item.id, RECENT_ACTIVITY_LIMIT)
        : current.audits,
  };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function stringValue(value: unknown) {
  return typeof value === 'string' ? value.trim() : '';
}

function parseJsonObject(raw: string, errorMessage: string) {
  const parsed = JSON.parse(raw) as unknown;
  if (!isRecord(parsed)) {
    throw new Error(errorMessage);
  }
  return parsed;
}

export function getCatalogDefaultConfig(plugin: CatalogPlugin): Record<string, unknown> {
  const schema = plugin.manifest.config_schema;
  if (!isRecord(schema)) {
    return {};
  }
  const config = schema.default;
  if (!isRecord(config)) {
    return {};
  }
  return config;
}

function mergeDefaultObjects(defaults: Record<string, unknown>, current: Record<string, unknown>) {
  const merged: Record<string, unknown> = { ...defaults };
  for (const [key, value] of Object.entries(current)) {
    const defaultValue = merged[key];
    if (isRecord(defaultValue) && isRecord(value)) {
      merged[key] = mergeDefaultObjects(defaultValue, value);
      continue;
    }
    merged[key] = value;
  }
  return merged;
}

export function buildInstallDrafts(catalog: CatalogPlugin[]) {
  const drafts: Record<string, string> = {};
  for (const plugin of catalog) {
    drafts[plugin.id] = JSON.stringify(getCatalogDefaultConfig(plugin), null, 2);
  }
  return drafts;
}

export function mergeCatalogDefaultConfig(plugin: CatalogPlugin, currentConfig: Record<string, unknown>) {
  return mergeDefaultObjects(getCatalogDefaultConfig(plugin), currentConfig);
}

export function getXiaomiDraftSeed(raw: string) {
  const draft = parseJsonObject(raw, 'Xiaomi config must be a JSON object');
  const accounts = Array.isArray(draft.accounts) ? draft.accounts : [];
  if (accounts.length === 0) {
    throw new Error('Xiaomi config must include at least one account');
  }
  const firstAccount = accounts[0];
  if (!isRecord(firstAccount)) {
    throw new Error('Xiaomi first account must be a JSON object');
  }
  const region = stringValue(firstAccount.region);
  if (!region) {
    throw new Error('Xiaomi first account requires region');
  }
  const clientId = stringValue(firstAccount.client_id);
  if (!clientId) {
    throw new Error('Xiaomi first account requires client_id');
  }
  return {
    draft,
    accounts,
    accountName: stringValue(firstAccount.name) || 'primary',
    region,
    clientId,
  };
}

export function canStartXiaomiOAuth(raw: string) {
  try {
    getXiaomiDraftSeed(raw);
    return true;
  } catch {
    return false;
  }
}

export function mergeXiaomiAccountConfig(rawDraft: string, accountConfig: Record<string, unknown>) {
  const draft = parseJsonObject(rawDraft, 'Xiaomi config must be a JSON object');
  const nextDraft: Record<string, unknown> = { ...draft };
  const accounts = Array.isArray(draft.accounts) ? [...draft.accounts] : [];
  if (accounts.length === 0) {
    throw new Error('Xiaomi config must include at least one account');
  }

  const targetName = stringValue(accountConfig.name);
  let targetIndex = -1;
  if (targetName) {
    targetIndex = accounts.findIndex((account) => isRecord(account) && stringValue(account.name) === targetName);
  }
  if (targetIndex < 0) {
    targetIndex = 0;
  }

  const currentAccount = isRecord(accounts[targetIndex]) ? accounts[targetIndex] : {};
  accounts[targetIndex] = {
    ...currentAccount,
    ...accountConfig,
  };
  nextDraft.accounts = accounts;

  return {
    draft: nextDraft,
    json: JSON.stringify(nextDraft, null, 2),
    accountName:
      targetName ||
      stringValue((accounts[targetIndex] as Record<string, unknown>).name) ||
      stringValue(accounts[0] && isRecord(accounts[0]) ? accounts[0].name : undefined) ||
      'primary',
  };
}

export function getPluginDraftText(
  pluginId: string,
  runtimeInstalled: boolean,
  installDrafts: Record<string, string>,
  configDrafts: Record<string, string>,
) {
  if (runtimeInstalled) {
    return configDrafts[pluginId] ?? installDrafts[pluginId] ?? '{}';
  }
  return installDrafts[pluginId] ?? configDrafts[pluginId] ?? '{}';
}
