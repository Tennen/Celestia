import type { PluginRuntimeView } from './types';

export type PluginStatusBadge = {
  label: string;
  tone: 'neutral' | 'good' | 'warn' | 'bad' | 'accent';
};

export function getPluginStatusBadge(runtime?: PluginRuntimeView | null): PluginStatusBadge {
  if (!runtime) {
    return { label: 'uninstalled', tone: 'neutral' };
  }

  if (runtime.record.status === 'disabled') {
    return { label: 'disabled', tone: 'neutral' };
  }

  if (runtime.health.status === 'degraded') {
    return { label: 'degraded', tone: 'warn' };
  }

  if (runtime.health.status === 'unhealthy' || runtime.health.status === 'stopped') {
    return { label: 'degraded', tone: 'bad' };
  }

  if (runtime.record.status === 'enabled') {
    return { label: 'enabled', tone: 'good' };
  }

  return { label: 'installed', tone: 'accent' };
}
