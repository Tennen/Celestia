import { useCallback, useEffect, useState, type Dispatch, type SetStateAction } from 'react';
import { sendToggle } from '../lib/api';
import {
  getToggleControlStates,
  getToggleOverrideKey,
  pruneToggleOverrides,
  type ToggleControlOverrideMap,
  type ToggleControlPendingMap,
} from '../lib/control-state';
import type { DeviceView } from '../lib/types';

type Args = {
  actor: string;
  devices: DeviceView[];
  selectedDevice: DeviceView | null;
  refreshAll: () => Promise<unknown>;
  reportError: (message: string) => void;
};

function restoreToggleOverride(
  overrideKey: string,
  previousState: boolean | null,
  persistedState: boolean | null,
  setToggleOverrides: Dispatch<SetStateAction<ToggleControlOverrideMap>>,
) {
  setToggleOverrides((current) => {
    const next = { ...current };
    if (previousState === persistedState) {
      delete next[overrideKey];
      return next;
    }
    next[overrideKey] = {
      state: previousState ?? false,
      requestedAt: Date.now(),
    };
    return next;
  });
}

export function useToggleControlActions({ actor, devices, selectedDevice, refreshAll, reportError }: Args) {
  const [toggleOverrides, setToggleOverrides] = useState<ToggleControlOverrideMap>({});
  const [togglePending, setTogglePending] = useState<ToggleControlPendingMap>({});

  useEffect(() => {
    setToggleOverrides((current) => pruneToggleOverrides(current, devices));
  }, [devices]);

  const onToggleControl = useCallback(
    (controlId: string, on: boolean) => {
      if (!selectedDevice) {
        return;
      }

      const deviceId = selectedDevice.device.id;
      const compoundId = `${deviceId}.${controlId}`;
      const overrideKey = getToggleOverrideKey(deviceId, controlId);
      const toggleStates = getToggleControlStates(selectedDevice, controlId, toggleOverrides);
      if (!toggleStates || togglePending[overrideKey]) {
        return;
      }

      setToggleOverrides((current) => ({
        ...current,
        [overrideKey]: {
          state: on,
          requestedAt: Date.now(),
        },
      }));
      setTogglePending((current) => ({ ...current, [overrideKey]: true }));

      void (async () => {
        try {
          await sendToggle(compoundId, on, actor);
        } catch (error) {
          restoreToggleOverride(overrideKey, toggleStates.currentState, toggleStates.persistedState, setToggleOverrides);
          reportError(error instanceof Error ? error.message : 'Operation failed');
          setTogglePending((current) => {
            if (!(overrideKey in current)) {
              return current;
            }
            const next = { ...current };
            delete next[overrideKey];
            return next;
          });
          return;
        }

        try {
          await refreshAll();
        } catch (error) {
          reportError(error instanceof Error ? error.message : 'Operation failed');
        } finally {
          setTogglePending((current) => {
            if (!(overrideKey in current)) {
              return current;
            }
            const next = { ...current };
            delete next[overrideKey];
            return next;
          });
        }
      })();
    },
    [actor, refreshAll, reportError, selectedDevice, toggleOverrides, togglePending],
  );

  return {
    toggleOverrides,
    togglePending,
    onToggleControl,
  };
}
