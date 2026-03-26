import { useEffect, useMemo, useRef, useState } from 'react';
import {
  fetchAudits,
  fetchCatalogPlugins,
  fetchDashboard,
  fetchDevices,
  fetchEvents,
  fetchPluginLogs,
  fetchPlugins,
  getApiBase,
} from '../lib/api';
import { asArray, compareText, emptyLoadState, POLL_MS } from '../lib/admin';
import type { LoadState } from '../lib/admin';

export function useAdminConsole() {
  const [state, setState] = useState<LoadState>(emptyLoadState);
  const [selectedPluginId, setSelectedPluginId] = useState('');
  const [selectedDeviceId, setSelectedDeviceId] = useState('');
  const [deviceSearch, setDeviceSearch] = useState('');
  const [pluginLogs, setPluginLogs] = useState<string[]>([]);

  const selectedPluginIdRef = useRef('');
  const selectedDeviceIdRef = useRef('');
  const deviceSearchRef = useRef('');
  const refreshAllRef = useRef<() => Promise<void>>(async () => {});

  useEffect(() => {
    selectedPluginIdRef.current = selectedPluginId;
  }, [selectedPluginId]);

  useEffect(() => {
    selectedDeviceIdRef.current = selectedDeviceId;
  }, [selectedDeviceId]);

  useEffect(() => {
    deviceSearchRef.current = deviceSearch;
  }, [deviceSearch]);

  const refreshAll = async () => {
    setState((current) => ({ ...current, loading: true, error: null }));
    try {
      const [dashboard, rawCatalog, rawPlugins, rawDevices, rawEvents, rawAudits] = await Promise.all([
        fetchDashboard(),
        fetchCatalogPlugins(),
        fetchPlugins(),
        fetchDevices(deviceSearchRef.current),
        fetchEvents(80),
        fetchAudits(80),
      ]);

      const catalog = asArray(rawCatalog).sort((a, b) => compareText(a.id, b.id));
      const plugins = asArray(rawPlugins).sort((a, b) => compareText(a.record.plugin_id, b.record.plugin_id));
      const devices = asArray(rawDevices).sort((a, b) =>
        compareText(a.device.name || a.device.id, b.device.name || b.device.id),
      );
      const events = asArray(rawEvents);
      const audits = asArray(rawAudits);

      setState({
        dashboard,
        catalog,
        plugins,
        devices,
        events,
        audits,
        loading: false,
        error: null,
      });

      if (!selectedPluginIdRef.current && catalog.length > 0) {
        setSelectedPluginId(catalog[0].id);
      } else if (selectedPluginIdRef.current && !catalog.some((plugin) => plugin.id === selectedPluginIdRef.current)) {
        setSelectedPluginId(catalog[0]?.id ?? '');
      }

      if (!selectedDeviceIdRef.current && devices.length > 0) {
        setSelectedDeviceId(devices[0].device.id);
      } else if (selectedDeviceIdRef.current && !devices.some((device) => device.device.id === selectedDeviceIdRef.current)) {
        setSelectedDeviceId(devices[0]?.device.id ?? '');
      }
    } catch (error) {
      setState((current) => ({
        ...current,
        loading: false,
        error: error instanceof Error ? error.message : 'Failed to load admin data',
      }));
    }
  };

  refreshAllRef.current = refreshAll;

  useEffect(() => {
    void refreshAll();
    const interval = window.setInterval(() => {
      void refreshAllRef.current();
    }, POLL_MS);
    return () => window.clearInterval(interval);
  }, [deviceSearch]);

  useEffect(() => {
    if (!selectedPluginId) {
      setPluginLogs([]);
      return;
    }

    let canceled = false;
    void fetchPluginLogs(selectedPluginId)
      .then((data) => {
        if (!canceled) {
          setPluginLogs(asArray(data.logs));
        }
      })
      .catch(() => {
        if (!canceled) {
          setPluginLogs(['Unable to load logs.']);
        }
      });

    return () => {
      canceled = true;
    };
  }, [selectedPluginId, state.plugins.length]);

  useEffect(() => {
    const source = new EventSource(`${getApiBase()}/events/stream`);
    source.onmessage = () => void refreshAllRef.current();
    source.addEventListener('device.state.changed', () => void refreshAllRef.current());
    source.addEventListener('device.event.occurred', () => void refreshAllRef.current());
    source.addEventListener('plugin.lifecycle.changed', () => void refreshAllRef.current());
    source.onerror = () => {
      source.close();
    };

    return () => {
      source.close();
    };
  }, []);

  const selectedCatalogPlugin = useMemo(
    () => state.catalog.find((plugin) => plugin.id === selectedPluginId) ?? null,
    [selectedPluginId, state.catalog],
  );

  const selectedPlugin = useMemo(
    () => state.plugins.find((plugin) => plugin.record.plugin_id === selectedPluginId) ?? null,
    [selectedPluginId, state.plugins],
  );

  const selectedDevice = useMemo(
    () => state.devices.find((item) => item.device.id === selectedDeviceId) ?? null,
    [selectedDeviceId, state.devices],
  );

  const reloadPluginLogs = async (pluginId = selectedPluginId) => {
    if (!pluginId) {
      setPluginLogs([]);
      return;
    }

    try {
      const data = await fetchPluginLogs(pluginId);
      setPluginLogs(asArray(data.logs));
    } catch {
      setPluginLogs(['Unable to load logs.']);
    }
  };

  const reportError = (message: string) => {
    setState((current) => ({ ...current, error: message }));
  };

  return {
    state,
    refreshAll,
    selectedPluginId,
    setSelectedPluginId,
    selectedDeviceId,
    setSelectedDeviceId,
    deviceSearch,
    setDeviceSearch,
    pluginLogs,
    reloadPluginLogs,
    selectedCatalogPlugin,
    selectedPlugin,
    selectedDevice,
    reportError,
  };
}
