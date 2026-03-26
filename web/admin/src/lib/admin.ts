import type {
  AuditRecord,
  CatalogPlugin,
  DashboardSummary,
  DeviceView,
  EventRecord,
  PluginRuntimeView,
} from './types';

export const DEFAULT_INSTALL_CONFIGS: Record<string, string> = {
  xiaomi: JSON.stringify(
    {
      accounts: [
        {
          name: 'primary',
          region: 'cn',
          username: '<xiaomi-username>',
          password: '<xiaomi-password>',
          device_id: 'CELESTIAXIAOMI01',
          verify_url: '<optional-xiaomi-verify-url>',
          verify_ticket: '<optional-sms-or-email-code>',
          service_token: '<optional-service-token>',
          ssecurity: '<optional-ssecurity>',
          user_id: '<optional-user-id>',
          cuser_id: '<optional-cuser-id>',
          locale: 'zh_CN',
          timezone: 'GMT+08:00',
          home_ids: ['<optional-home-id>'],
        },
      ],
      poll_interval_seconds: 30,
    },
    null,
    2,
  ),
  petkit: JSON.stringify(
    {
      accounts: [
        {
          name: 'primary',
          username: '<petkit-username>',
          password: '<petkit-password>',
          region: 'US',
          timezone: 'Asia/Shanghai',
        },
      ],
      poll_interval_seconds: 30,
    },
    null,
    2,
  ),
  haier: JSON.stringify(
    {
      accounts: [
        {
          name: 'primary',
          email: '<hon-email>',
          password: '<hon-password>',
          mobile_id: 'celestia-primary',
          timezone: 'Asia/Shanghai',
        },
      ],
      poll_interval_seconds: 20,
    },
    null,
    2,
  ),
};

export const POLL_MS = 10000;

export type StatusBanner = {
  tone: 'good' | 'warn' | 'bad';
  text: string;
};

export type AppSection = 'overview' | 'plugins' | 'devices' | 'activity';

export type LoadState = {
  dashboard: DashboardSummary | null;
  catalog: CatalogPlugin[];
  plugins: PluginRuntimeView[];
  devices: DeviceView[];
  events: EventRecord[];
  audits: AuditRecord[];
  loading: boolean;
  error: string | null;
};

export const emptyLoadState = (): LoadState => ({
  dashboard: null,
  catalog: [],
  plugins: [],
  devices: [],
  events: [],
  audits: [],
  loading: true,
  error: null,
});

export function asArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : [];
}

export function compareText(a: string, b: string) {
  return a.localeCompare(b, 'en');
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
