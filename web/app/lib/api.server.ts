const apiBase = () => process.env.DEPLOYBOT_API_URL ?? "http://127.0.0.1:8080";

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const url = new URL(path, apiBase());
  const res = await fetch(url, {
    ...init,
    headers: {
      Accept: "application/json",
      ...(init?.body ? { "Content-Type": "application/json" } : {}),
      ...init?.headers,
    },
    signal: init?.signal ?? AbortSignal.timeout(20_000),
  });
  const text = await res.text();
  let body: unknown = null;
  if (text) {
    try {
      body = JSON.parse(text) as unknown;
    } catch {
      body = { error: text };
    }
  }
  if (!res.ok) {
    const err =
      typeof body === "object" && body !== null && "error" in body
        ? String((body as { error: unknown }).error)
        : `${res.status} ${res.statusText}`;
    throw new Error(err);
  }
  return body as T;
}

export type DeployableSummary = {
  name: string;
  namespace: string;
  image: string;
  stages: string[];
};

export type StageStatus = {
  name: string;
  hostname: string;
  image?: string;
  sync: string;
  health: string;
  revision?: string;
  message?: string;
};

export type DeployableStatus = {
  name: string;
  namespace: string;
  imageRepo: string;
  stages: StageStatus[];
  apply: boolean;
};

export type MutationResult = {
  dryRun: boolean;
  commit?: string;
  diff: string;
  files: string[];
  synced: boolean;
  error?: string;
};

export function listDeployables() {
  return apiFetch<{ deployables: DeployableSummary[] }>("/api/v1/deployables");
}

export function getDeployable(name: string) {
  return apiFetch<DeployableStatus>(`/api/v1/deployables/${encodeURIComponent(name)}`);
}

export function diffPin(name: string, stage: string, image: string) {
  const q = new URLSearchParams({ stage, image });
  return apiFetch<{ diff: string }>(
    `/api/v1/deployables/${encodeURIComponent(name)}/diff?${q}`,
  );
}

export function pinDeployable(name: string, stage: string, image: string) {
  return apiFetch<MutationResult>(`/api/v1/deployables/${encodeURIComponent(name)}/pin`, {
    method: "POST",
    body: JSON.stringify({ stage, image }),
  });
}

export function promoteDeployable(name: string, from: string, to: string) {
  return apiFetch<MutationResult>(
    `/api/v1/deployables/${encodeURIComponent(name)}/promote`,
    {
      method: "POST",
      body: JSON.stringify({ from, to }),
    },
  );
}

export function previewSync(name: string, stage: string) {
  const q = new URLSearchParams({ stage });
  return apiFetch<MutationResult>(
    `/api/v1/deployables/${encodeURIComponent(name)}/sync?${q}`,
  );
}

export function syncDeployable(name: string, stage: string) {
  return apiFetch<MutationResult>(
    `/api/v1/deployables/${encodeURIComponent(name)}/sync`,
    {
      method: "POST",
      body: JSON.stringify({ stage }),
    },
  );
}
