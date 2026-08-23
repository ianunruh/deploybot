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

export type StageStatus = {
  name: string;
  hostname: string;
  image?: string;
  sync: string;
  health: string;
  revision?: string;
  message?: string;
  deployedAt?: string;
  pinnedAt?: string;
  argoURL?: string;
  headlampURL?: string;
  grafanaURL?: string;
  logsURL?: string;
};

export type FlowHop = {
  from: string;
  to: string;
  state: string;
  gate?: string;
  remaining?: string;
  bakeUntil?: string;
  sourceImage?: string;
  destImage?: string;
};

export type SourceCommit = {
  sha?: string;
  message?: string;
  author?: string;
  url?: string;
};

export type Flow = {
  image?: string;
  digest?: string;
  tag?: string;
  source?: SourceCommit;
  hops: FlowHop[];
};

export type DeployableSummary = {
  name: string;
  namespace: string;
  repoURL?: string;
  projectURL?: string;
  deployedAt?: string;
  stages: StageStatus[];
  flow?: Flow;
};

export type DeployableStatus = {
  name: string;
  namespace: string;
  imageRepo: string;
  repoURL?: string;
  projectURL?: string;
  stages: StageStatus[];
  flow?: Flow;
  apply: boolean;
  push: boolean;
  sync: boolean;
};

export type ReleaseStageHit = {
  at: string;
  kind: string;
  commit?: string;
  commitURL?: string;
};

export type Release = {
  image: string;
  digest?: string;
  tag?: string;
  current?: boolean;
  source?: SourceCommit;
  stages: Record<string, ReleaseStageHit>;
};

export type DeployableHistory = {
  events: Array<{
    at: string;
    kind: string;
    stage: string;
    image: string;
    digest?: string;
    tag?: string;
    commit: string;
    commitURL?: string;
    author?: string;
  }>;
  releases: Release[];
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

export function getDeployableHistory(name: string) {
  return apiFetch<DeployableHistory>(
    `/api/v1/deployables/${encodeURIComponent(name)}/history`,
  );
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
  opts?: { sync?: boolean; image?: string },
) {
  return apiFetch<MutationResult>(
    `/api/v1/deployables/${encodeURIComponent(name)}/promote`,
    {
      method: "POST",
      body: JSON.stringify({
        from,
        to,
        ...(opts?.image ? { image: opts.image } : {}),
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
