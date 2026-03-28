import type { DeviceView } from './types';

export const TOGGLE_OVERRIDE_TTL_MS = 30_000;

const TOGGLE_OVERRIDE_SEPARATOR = '\u0000';

export type ToggleControlOverride = {
  state: boolean;
  requestedAt: number;
};

export type ToggleControlOverrideMap = Record<string, ToggleControlOverride>;

export function getToggleOverrideKey(deviceId: string, controlId: string) {
  return `${deviceId}${TOGGLE_OVERRIDE_SEPARATOR}${controlId}`;
}

function parseToggleOverrideKey(key: string) {
  const separatorIndex = key.indexOf(TOGGLE_OVERRIDE_SEPARATOR);
  if (separatorIndex < 0) {
    return null;
  }
  return {
    deviceId: key.slice(0, separatorIndex),
    controlId: key.slice(separatorIndex + TOGGLE_OVERRIDE_SEPARATOR.length),
  };
}

function isFreshOverride(requestedAt: number, now: number) {
  return now - requestedAt <= TOGGLE_OVERRIDE_TTL_MS;
}

export function applyToggleOverrides(
  device: DeviceView | null,
  overrides: ToggleControlOverrideMap,
  now = Date.now(),
) {
  if (!device || !device.controls?.length) {
    return device;
  }

  let changed = false;
  const controls = device.controls.map((control) => {
    if (control.kind !== 'toggle') {
      return control;
    }

    const override = overrides[getToggleOverrideKey(device.device.id, control.id)];
    if (!override || !isFreshOverride(override.requestedAt, now) || control.state === override.state) {
      return control;
    }

    changed = true;
    return {
      ...control,
      state: override.state,
      value: override.state,
    };
  });

  if (!changed) {
    return device;
  }

  return {
    ...device,
    controls,
  };
}

export function isToggleControlPending(
  device: DeviceView | null,
  controlId: string,
  overrides: ToggleControlOverrideMap,
  now = Date.now(),
) {
  if (!device) {
    return false;
  }

  const control = device.controls?.find((item) => item.id === controlId && item.kind === 'toggle');
  if (!control) {
    return false;
  }

  const override = overrides[getToggleOverrideKey(device.device.id, controlId)];
  if (!override || !isFreshOverride(override.requestedAt, now)) {
    return false;
  }

  return control.state !== override.state;
}

export function pruneToggleOverrides(
  overrides: ToggleControlOverrideMap,
  devices: DeviceView[],
  now = Date.now(),
) {
  const deviceIndex = new Map(devices.map((device) => [device.device.id, device]));
  const next: ToggleControlOverrideMap = {};
  let changed = false;

  for (const [key, override] of Object.entries(overrides)) {
    if (!isFreshOverride(override.requestedAt, now)) {
      changed = true;
      continue;
    }

    const ref = parseToggleOverrideKey(key);
    if (!ref) {
      changed = true;
      continue;
    }

    const device = deviceIndex.get(ref.deviceId);
    if (!device) {
      next[key] = override;
      continue;
    }

    const control = device.controls?.find((item) => item.id === ref.controlId && item.kind === 'toggle');
    if (!control || control.state === override.state) {
      changed = true;
      continue;
    }

    next[key] = override;
  }

  if (!changed && Object.keys(next).length === Object.keys(overrides).length) {
    return overrides;
  }

  return next;
}
