const apiBase = () => process.env.DEPLOYBOT_API_URL ?? "http://127.0.0.1:8080";

// Dex Authorization, or the IdToken cookie when Envoy only forwarded the cookie.
export function actorHeaders(request: Request): Record<string, string> {
  const auth = request.headers.get("authorization");
  if (auth) {
    return { Authorization: auth };
  }
  const token = cookieValue(request.headers.get("cookie") ?? "", "IdToken");
  if (token) {
    return { Authorization: `Bearer ${token}` };
  }
  return {};
}

function cookieValue(header: string, name: string): string {
  const prefix = `${name}=`;
  for (const part of header.split(";")) {
    const trimmed = part.trim();
    if (trimmed.startsWith(prefix)) {
      try {
        return decodeURIComponent(trimmed.slice(prefix.length));
      } catch {
        return trimmed.slice(prefix.length);
      }
    }
  }
  return "";
}

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

export type PodLive = {
  name: string;
  ready: string;
  status: string;
  restarts: number;
  ip?: string;
  node?: string;
  createdAt?: string;
  restartedAt?: string;
};

export type WorkloadLive = {
  kind: string;
  name: string;
  desired: number;
  ready: number;
  updated?: number;
  available?: number;
  message?: string;
  pods?: PodLive[];
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
  pinnedAt?: string;
  previousImage?: string;
  previousRef?: string;
  argoURL?: string;
  headlampURL?: string;
  grafanaURL?: string;
  logsURL?: string;
  workload?: WorkloadLive;
  connected?: boolean;
  updatedAt?: string;
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

export type UpdateSummary = {
  stale: boolean;
  auto?: string;
};

export type UpdatePin = {
  tag?: string;
  digest?: string;
  compact?: string;
  ref?: string;
};

export type UpdateNewest = {
  tag?: string;
  digest?: string;
  ref?: string;
  createdAt?: string;
};

export type UpdateStatus = {
  name: string;
  namespace: string;
  project: string;
  repository: string;
  stage: string;
  current: UpdatePin;
  newest?: UpdateNewest;
  stale: boolean;
  auto?: string;
  checkedAt?: string;
  error?: string;
};

export type UpdateList = {
  updates: UpdateStatus[];
  apply: boolean;
  push: boolean;
  sync: boolean;
};

export type DeployableSummary = {
  name: string;
  namespace: string;
  project: string;
  summary?: string;
  icon?: string;
  docsURL?: string;
  repoURL?: string;
  projectURL?: string;
  deployedAt?: string;
  stages: StageStatus[];
  flow?: Flow;
  update?: UpdateSummary;
};

export type WorkflowRun = {
  id: number;
  name: string;
  title?: string;
  number: number;
  event?: string;
  status: string;
  branch?: string;
  sha?: string;
  actor?: string;
  url?: string;
  commitURL?: string;
  startedAt?: string;
};

export type Workflows = {
  url?: string;
  runs: WorkflowRun[];
  error?: string;
};

export type DeployableStatus = {
  name: string;
  namespace: string;
  project: string;
  summary?: string;
  icon?: string;
  docsURL?: string;
  imageRepo: string;
  repoURL?: string;
  projectURL?: string;
  source?: boolean;
  stages: StageStatus[];
  flow?: Flow;
  apply: boolean;
  push: boolean;
  sync: boolean;
  update?: UpdateStatus;
};

export type StageWorkload = {
  name: string;
  workload?: WorkloadLive;
};

export type LiveWorkloads = {
  stages: StageWorkload[];
};

export type Actor = {
  kind?: string;
  id?: string;
  repo?: string;
  name?: string;
  email?: string;
};

export type ReleaseStageHit = {
  at: string;
  kind: string;
  commit?: string;
  commitURL?: string;
  author?: string;
  actor?: Actor;
};

export type Release = {
  image: string;
  digest?: string;
  tag?: string;
  current?: boolean;
  source?: SourceCommit;
  stages: Record<string, ReleaseStageHit>;
};

export type HistoryEvent = {
  at: string;
  kind: string;
  deployable?: string;
  namespace?: string;
  project?: string;
  stage: string;
  image: string;
  digest?: string;
  tag?: string;
  commit: string;
  commitURL?: string;
  author?: string;
  actor?: Actor;
};

export type DeployableHistory = {
  events: HistoryEvent[];
  releases: Release[];
};

export type GlobalHistory = {
  events: HistoryEvent[];
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

export function listUpdates() {
  return apiFetch<UpdateList>("/api/v1/updates");
}

export function listHistory() {
  return apiFetch<GlobalHistory>("/api/v1/history");
}

export function getDeployable(name: string) {
  return apiFetch<DeployableStatus>(`/api/v1/deployables/${encodeURIComponent(name)}`);
}

export function getDeployableWorkloads(name: string) {
  return apiFetch<LiveWorkloads>(
    `/api/v1/deployables/${encodeURIComponent(name)}/workloads`,
  );
}

export function getDeployableWorkflows(name: string) {
  return apiFetch<Workflows>(`/api/v1/deployables/${encodeURIComponent(name)}/workflows`);
}

export function getDeployableHistory(name: string) {
  return apiFetch<DeployableHistory>(
    `/api/v1/deployables/${encodeURIComponent(name)}/history`,
  );
}

export type Changelog = {
  from: string;
  to: string;
  base?: SourceCommit;
  head?: SourceCommit;
  url?: string;
  status?: string;
  aheadBy?: number;
  behindBy?: number;
  truncated?: boolean;
  commits: SourceCommit[];
  error?: string;
};

export function getDeployableChangelog(name: string, from: string, to: string) {
  const q = new URLSearchParams({ from, to });
  return apiFetch<Changelog>(
    `/api/v1/deployables/${encodeURIComponent(name)}/changelog?${q}`,
    { signal: AbortSignal.timeout(10_000) },
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
  opts?: { sync?: boolean; wait?: boolean; headers?: HeadersInit },
) {
  return apiFetch<MutationResult>(`/api/v1/deployables/${encodeURIComponent(name)}/pin`, {
    method: "POST",
    headers: opts?.headers,
    body: JSON.stringify({
      stage,
      image,
      ...(opts?.sync != null ? { sync: opts.sync } : {}),
      ...(opts?.wait != null ? { wait: opts.wait } : {}),
    }),
  });
}

export function rollbackDeployable(
  name: string,
  stage: string,
  image: string,
  opts?: { sync?: boolean; wait?: boolean; headers?: HeadersInit },
) {
  return apiFetch<MutationResult>(
    `/api/v1/deployables/${encodeURIComponent(name)}/rollback`,
    {
      method: "POST",
      headers: opts?.headers,
      body: JSON.stringify({
        stage,
        image,
        ...(opts?.sync != null ? { sync: opts.sync } : {}),
        ...(opts?.wait != null ? { wait: opts.wait } : {}),
      }),
    },
  );
}

export function promoteDeployable(
  name: string,
  from: string,
  to: string,
  opts?: { sync?: boolean; wait?: boolean; image?: string; headers?: HeadersInit },
) {
  return apiFetch<MutationResult>(
    `/api/v1/deployables/${encodeURIComponent(name)}/promote`,
    {
      method: "POST",
      headers: opts?.headers,
      body: JSON.stringify({
        from,
        to,
        ...(opts?.image ? { image: opts.image } : {}),
        ...(opts?.sync != null ? { sync: opts.sync } : {}),
        ...(opts?.wait != null ? { wait: opts.wait } : {}),
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
  opts?: { sync?: boolean; wait?: boolean; headers?: HeadersInit },
) {
  return apiFetch<MutationResult>(
    `/api/v1/deployables/${encodeURIComponent(name)}/reconcile`,
    {
      method: "POST",
      headers: opts?.headers,
      body: JSON.stringify({
        stage,
        ...(opts?.sync != null ? { sync: opts.sync } : {}),
        ...(opts?.wait != null ? { wait: opts.wait } : {}),
      }),
    },
  );
}
