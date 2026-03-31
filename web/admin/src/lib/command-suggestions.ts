import type { DeviceView } from './types';

export type CommandSuggestion = {
  label: string;
  action: string;
  params: Record<string, unknown>;
};

type SuggestionSpec =
  | { capability: string; label: string; action: string; params: Record<string, unknown> }
  | { capability: string; label: string; action: string; params: (device: DeviceView) => Record<string, unknown> | null };

const STATIC_SUGGESTIONS: SuggestionSpec[] = [
  { capability: 'feed_once',        label: 'Feed once',        action: 'feed_once',        params: { portions: 1 } },
  { capability: 'manual_feed_dual', label: 'Feed dual',        action: 'manual_feed_dual', params: { amount1: 20, amount2: 20 } },
  { capability: 'cancel_manual_feed', label: 'Cancel feed',    action: 'cancel_manual_feed', params: {} },
  { capability: 'reset_desiccant',  label: 'Reset desiccant',  action: 'reset_desiccant',  params: {} },
  { capability: 'food_replenished', label: 'Food replenished', action: 'food_replenished', params: {} },
  { capability: 'call_pet',         label: 'Call pet',         action: 'call_pet',         params: {} },
  { capability: 'play_sound',       label: 'Play sound',       action: 'play_sound',
    params: (device) => {
      const sound = Number((device.state.state as Record<string, unknown> | undefined)?.selected_sound ?? 0);
      return sound > 0 ? { sound_id: sound } : null;
    },
  },
  { capability: 'clean_now',        label: 'Clean now',        action: 'clean_now',        params: {} },
  { capability: 'start',            label: 'Start cycle',      action: 'start',            params: {} },
  { capability: 'pause',            label: 'Pause',            action: 'pause',            params: {} },
  { capability: 'resume',           label: 'Resume',           action: 'resume',           params: {} },
  { capability: 'power',            label: 'Toggle power',     action: 'set_power',        params: { on: true } },
  { capability: 'pump_power',       label: 'Pump on',          action: 'set_pump_power',   params: { on: true } },
  { capability: 'light_power',      label: 'Aquarium light',   action: 'set_light_power',  params: { on: true } },
  { capability: 'light_brightness', label: 'Aquarium brightness', action: 'set_light_brightness', params: { value: 80 } },
  { capability: 'voice_push',       label: 'Voice push',       action: 'push_voice_message',
    params: { message: '检测到异常，请查看鱼缸状态', volume: 55 },
  },
  { capability: 'volume',           label: 'Set volume',       action: 'set_volume',       params: { value: 55 } },
  { capability: 'mute',             label: 'Mute speaker',     action: 'set_mute',         params: { on: true } },
];

export function buildCommandSuggestions(device: DeviceView): CommandSuggestion[] {
  const caps = new Set(device.device.capabilities);
  return STATIC_SUGGESTIONS.flatMap((spec) => {
    if (!caps.has(spec.capability)) return [];
    const resolvedParams =
      typeof spec.params === 'function' ? spec.params(device) : spec.params;
    if (resolvedParams === null) return [];
    return [{ label: spec.label, action: spec.action, params: resolvedParams }];
  });
}
