import {useEffect, useRef, useState} from "react";
import {describeError} from "./api";

export function useResource<T>(load: () => Promise<T>, key: string | number, enabled = true, revision = 0) {
  const [result, setResult] = useState<{key: string | number; value?: T; error?: string}>();
  useEffect(() => {
    if (!enabled) return;
    let active = true;
    load().then(
      value => { if (active) setResult({key, value}); },
      error => { if (active) setResult({key, error: describeError(error)}); },
    );
    return () => { active = false; };
  }, [key, enabled, revision]);
  return enabled && result?.key === key ? result : undefined;
}

export function useAction() {
  const running = useRef(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  async function run(work: () => Promise<void>) {
    if (running.current) return;
    running.current = true;
    setBusy(true);
    setError("");
    try { await work(); }
    catch (failure) { setError(describeError(failure)); }
    finally { running.current = false; setBusy(false); }
  }
  return {busy, error, run, clearError: () => setError("")};
}
