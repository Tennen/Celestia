import { useEffect, useMemo, useState } from 'react';

type DraftUpdater<T> = T | ((current: T) => T);

type State<T> = {
  baseline: T | null;
  draft: T | null;
  revision: number;
  sourceKey: string;
};

type Options<T> = {
  clone: (value: T) => T;
  isEqual?: (left: T, right: T) => boolean;
  source: T | null;
  sourceKey: string;
};

function defaultEqual<T>(left: T, right: T) {
  return JSON.stringify(left) === JSON.stringify(right);
}

function adoptState<T>(source: T | null, sourceKey: string, revision: number, clone: (value: T) => T): State<T> {
  return {
    baseline: source ? clone(source) : null,
    draft: source ? clone(source) : null,
    revision,
    sourceKey,
  };
}

export function useRetainedDraft<T>({ clone, isEqual = defaultEqual, source, sourceKey }: Options<T>) {
  const [state, setState] = useState<State<T>>(() => adoptState(source, sourceKey, 0, clone));

  useEffect(() => {
    setState((current) => {
      const dirty =
        current.draft !== null && current.baseline !== null ? !isEqual(current.draft, current.baseline) : false;
      if (current.sourceKey !== sourceKey || current.draft === null || !dirty) {
        return adoptState(source, sourceKey, current.revision + 1, clone);
      }
      return current;
    });
  }, [clone, isEqual, source, sourceKey]);

  const dirty = useMemo(() => {
    if (state.draft === null || state.baseline === null) {
      return false;
    }
    return !isEqual(state.draft, state.baseline);
  }, [isEqual, state.baseline, state.draft]);

  const setDraft = (updater: DraftUpdater<T>) => {
    setState((current) => {
      if (current.draft === null) {
        return current;
      }
      const nextDraft =
        typeof updater === 'function' ? (updater as (value: T) => T)(clone(current.draft)) : updater;
      return {
        ...current,
        draft: nextDraft,
      };
    });
  };

  const replaceDraft = (nextSource: T | null, nextSourceKey = sourceKey) => {
    setState((current) => adoptState(nextSource, nextSourceKey, current.revision + 1, clone));
  };

  const resetDraft = () => {
    setState((current) => adoptState(current.baseline, current.sourceKey, current.revision + 1, clone));
  };

  return {
    baseline: state.baseline,
    dirty,
    draft: state.draft,
    replaceDraft,
    resetDraft,
    revision: state.revision,
    setDraft,
  };
}
