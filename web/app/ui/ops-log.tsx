import { Text } from "@mantine/core";
import { useEffect, useRef, useState } from "react";

export function OpsLog({ src, active }: { src: string; active?: boolean }) {
  const [text, setText] = useState("");
  const [error, setError] = useState<string | null>(null);
  const preRef = useRef<HTMLPreElement>(null);

  useEffect(() => {
    const ac = new AbortController();
    let cancelled = false;
    (async () => {
      const res = await fetch(src, { signal: ac.signal });
      if (!res.ok) {
        const body = await res.text();
        if (!cancelled) setError(body || `${res.status} ${res.statusText}`);
        return;
      }
      if (res.body == null) {
        if (!cancelled) setError("no log stream");
        return;
      }
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        const chunk = decoder.decode(value, { stream: true });
        if (cancelled) return;
        setText((prev) => prev + chunk);
      }
    })().catch((err: unknown) => {
      if (cancelled) return;
      if (err instanceof DOMException && err.name === "AbortError") return;
      setError(err instanceof Error ? err.message : String(err));
    });
    return () => {
      cancelled = true;
      ac.abort();
    };
  }, [src]);

  useEffect(() => {
    const el = preRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [text]);

  return (
    <>
      {error != null ? (
        <Text size="sm" c="red.4">
          {error}
        </Text>
      ) : null}
      <pre ref={preRef} className="db-logs">
        {text || (active ? "waiting for output..." : "")}
      </pre>
    </>
  );
}
