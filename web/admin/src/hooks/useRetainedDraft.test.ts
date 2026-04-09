import { act, renderHook } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { useRetainedDraft } from './useRetainedDraft';

type Item = {
  id: string;
  name: string;
};

function cloneItem<T>(value: T) {
  return JSON.parse(JSON.stringify(value)) as T;
}

describe('useRetainedDraft', () => {
  it('keeps dirty local edits when the same source refreshes', () => {
    const { result, rerender } = renderHook(
      ({ source, sourceKey }: { source: Item | null; sourceKey: string }) =>
        useRetainedDraft<Item>({ source, sourceKey, clone: cloneItem }),
      {
        initialProps: {
          source: { id: 'vision', name: 'Recognition' },
          sourceKey: 'vision',
        },
      },
    );

    act(() => {
      result.current.setDraft((current) => ({ ...current, name: 'Unsaved edit' }));
    });

    rerender({
      source: { id: 'vision', name: 'Remote refresh' },
      sourceKey: 'vision',
    });

    expect(result.current.draft).toEqual({ id: 'vision', name: 'Unsaved edit' });
    expect(result.current.dirty).toBe(true);
  });

  it('adopts the incoming source when the source key changes', () => {
    const { result, rerender } = renderHook(
      ({ source, sourceKey }: { source: Item | null; sourceKey: string }) =>
        useRetainedDraft<Item>({ source, sourceKey, clone: cloneItem }),
      {
        initialProps: {
          source: { id: 'a', name: 'First' },
          sourceKey: 'a',
        },
      },
    );

    act(() => {
      result.current.setDraft((current) => ({ ...current, name: 'Unsaved edit' }));
    });

    rerender({
      source: { id: 'b', name: 'Second' },
      sourceKey: 'b',
    });

    expect(result.current.draft).toEqual({ id: 'b', name: 'Second' });
    expect(result.current.dirty).toBe(false);
  });
});
