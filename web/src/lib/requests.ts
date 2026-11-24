import {useEffect, useRef, useState} from "react";
import {ApiError, describeError, operationTrace, withActionTrace} from "./http";

export type ResourceState<T> = {
  value?: T;
  error?: string;
  loading: boolean;
  stale: boolean;
};

export function useResource<T>(load: () => Promise<T>, key: string | number, enabled = true, revision = 0): ResourceState<T> | undefined {
  const token = `${key}::${revision}`;
  const [result, setResult] = useState<{token: string; value?: T; error?: string}>();
  useEffect(() => {
    if (!enabled) return;
    let active = true;
    load().then(
      value => { if (active) setResult({token, value}); },
      error => { if (active) setResult(previous => ({token, value: previous?.value, error: describeError(error)})); },
    );
    return () => { active = false; };
  }, [token, enabled]);
  if (!enabled) return undefined;
  const current = result?.token === token;
  return {
    value: result?.value,
    error: current ? result?.error : undefined,
    loading: !current,
    stale: !current && result?.value !== undefined,
  };
}

const emptyFieldErrors: Record<string, string> = {};

export function useAction() {
  const running = useRef(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>(emptyFieldErrors);
  const [code, setCode] = useState("");

  async function run(work: (trace: string) => Promise<void>) {
    if (running.current) return;
    running.current = true;
    setBusy(true);
    setError("");
    setCode("");
    setFieldErrors(emptyFieldErrors);
    const trace = operationTrace();
    try {
      await withActionTrace(trace, () => work(trace));
    } catch (failure) {
      if (failure instanceof ApiError) {
        setCode(failure.code);
        if (Object.keys(failure.fields).length > 0) {
          setFieldErrors(failure.fields);
        }
      }
      setError(describeError(failure));
    } finally {
      running.current = false;
      setBusy(false);
    }
  }

  return {
    busy,
    error,
    code,
    fieldErrors,
    run,
    clearError: () => {
      setError("");
      setCode("");
      setFieldErrors(emptyFieldErrors);
    },
  };
}

export function useDebouncedValue<T>(value: T, delay = 260): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const timer = window.setTimeout(() => setDebounced(value), delay);
    return () => window.clearTimeout(timer);
  }, [value, delay]);
  return debounced;
}
