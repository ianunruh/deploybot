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

export type StageLinks = {
  name: string;
  headlampURL?: string;
  grafanaURL?: string;
  logsURL?: string;
};

export type DeployableSummary = {
  name: string;
  namespace: string;
  image: string;
  stages: string[];
  repoURL?: string;
  projectURL?: string;
  deployedAt?: string;
  stageLinks?: StageLinks[];
};

export type StageStatus = {
  name: string;
  hostname: string;
  image?: string;
  sync: string;
  health: string;
  revision?: string;
  message?: string;
  deployedAt?: string;
  argoURL?: string;
  headlampURL?: string;
  grafanaURL?: string;
  logsURL?: string;
};

export type DeployableStatus = {
  name: string;
  namespace: string;
  imageRepo: string;
  repoURL?: string;
  projectURL?: string;
  stages: StageStatus[];
  apply: boolean;
  push: boolean;
  sync: boolean;
};

export type MutationResult = {
  dryRun: boolean;
  commit?: string;
  pushed: boolean;
  ref?: string;
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

export type ImageVersion = {
  repository: string;
  ref: string;
  tag?: string;
  digest?: string;
  tags: string[];
  createdAt: string;
};

export type ImageList = {
  repository: string;
  source: string;
  images: ImageVersion[];
};

export function listImages(name: string) {
  return apiFetch<ImageList>(`/api/v1/deployables/${encodeURIComponent(name)}/images`, {
    signal: AbortSignal.timeout(30_000),
  });
}

export function diffPin(name: string, stage: string, image: string) {
  const q = new URLSearchParams({ stage, image });
  return apiFetch<{ diff: string }>(
    `/api/v1/deployables/${encodeURIComponent(name)}/diff?${q}`,
  );
}

export function pinDeployable(
  name: string,
  stage: string,
  image: string,
  opts?: { sync?: boolean },
) {
  return apiFetch<MutationResult>(`/api/v1/deployables/${encodeURIComponent(name)}/pin`, {
    method: "POST",
    body: JSON.stringify({
      stage,
      image,
      ...(opts?.sync != null ? { sync: opts.sync } : {}),
    }),
  });
}

export function promoteDeployable(
  name: string,
  from: string,
  to: string,
  opts?: { sync?: boolean },
) {
  return apiFetch<MutationResult>(
    `/api/v1/deployables/${encodeURIComponent(name)}/promote`,
    {
      method: "POST",
      body: JSON.stringify({
        from,
        to,
        ...(opts?.sync != null ? { sync: opts.sync } : {}),
      }),
    },
  );
}

export function previewReconcile(name: string, stage: string) {
  const q = new URLSearchParams({ stage });
  return apiFetch<MutationResult>(
    `/api/v1/deployables/${encodeURIComponent(name)}/reconcile?${q}`,
  );
}

export function reconcileDeployable(
  name: string,
  stage: string,
  opts?: { sync?: boolean },
) {
  return apiFetch<MutationResult>(
    `/api/v1/deployables/${encodeURIComponent(name)}/reconcile`,
    {
      method: "POST",
      body: JSON.stringify({
        stage,
        ...(opts?.sync != null ? { sync: opts.sync } : {}),
      }),
    },
  );
}
