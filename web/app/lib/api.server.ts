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

export type PauseEntry = {
  at?: string;
  by?: string;
  reason?: string;
};

export type AppPause = PauseEntry & {
  stages?: Record<string, PauseEntry>;
};

export type PauseFile = {
  all?: PauseEntry;
  stages?: Record<string, PauseEntry>;
  apps?: Record<string, AppPause>;
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
  pause?: PauseFile;
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
  return apiFetch<{ deployables: DeployableSummary[]; pause?: PauseFile }>(
    "/api/v1/deployables",
  );
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

export function pauseDeployments(opts: {
  name?: string;
  stage?: string;
  reason?: string;
  headers?: HeadersInit;
}) {
  return apiFetch<MutationResult>("/api/v1/pause", {
    method: "POST",
    headers: opts.headers,
    body: JSON.stringify({
      ...(opts.name ? { name: opts.name } : {}),
      ...(opts.stage ? { stage: opts.stage } : {}),
      ...(opts.reason ? { reason: opts.reason } : {}),
    }),
  });
}

export function unpauseDeployments(opts: {
  name?: string;
  stage?: string;
  headers?: HeadersInit;
}) {
  return apiFetch<MutationResult>("/api/v1/unpause", {
    method: "POST",
    headers: opts.headers,
    body: JSON.stringify({
      ...(opts.name ? { name: opts.name } : {}),
      ...(opts.stage ? { stage: opts.stage } : {}),
    }),
  });
}

export type PauseActionData =
  { ok: true; intent: string; result?: MutationResult } | { ok: false; error: string };

export async function submitPauseForm(
  form: FormData,
  headers: HeadersInit,
): Promise<PauseActionData | null> {
  const intent = String(form.get("intent") ?? "");
  if (intent !== "pause" && intent !== "unpause") return null;
  const name = String(form.get("name") ?? "").trim();
  const stage = String(form.get("stage") ?? "").trim();
  const reason = String(form.get("reason") ?? "").trim();
  try {
    const result =
      intent === "pause"
        ? await pauseDeployments({
            name: name || undefined,
            stage: stage || undefined,
            reason: reason || undefined,
            headers,
          })
        : await unpauseDeployments({
            name: name || undefined,
            stage: stage || undefined,
            headers,
          });
    return { ok: true, intent, result };
  } catch (err) {
    return { ok: false, error: err instanceof Error ? err.message : String(err) };
  }
}

export type OpsField = {
  name: string;
  type: string;
  title?: string;
  description?: string;
  required?: boolean;
  options?: string[];
  suggestions?: string[];
  keys?: string[];
};

export type OpsKind = {
  name: string;
  title: string;
  workDir?: string;
  fields: OpsField[];
};

export type OpsCatalog = {
  kinds: OpsKind[];
  clusters: string[];
  defaultRef?: string;
  imageSet: boolean;
};

export type OpsExecution = {
  id: string;
  kind: string;
  cluster: string;
  phase: string;
  dryRun: boolean;
  ref?: string;
  summary?: string;
  command?: string[];
  params?: unknown;
  actor?: Actor;
  podName?: string;
  message?: string;
  createdAt?: string;
};

export function getOpsCatalog() {
  return apiFetch<OpsCatalog>("/api/v1/ops/catalog");
}

export function listOpsExecutions(opts?: { kind?: string; cluster?: string }) {
  const q = new URLSearchParams();
  if (opts?.kind) q.set("kind", opts.kind);
  if (opts?.cluster) q.set("cluster", opts.cluster);
  const suffix = q.size > 0 ? `?${q}` : "";
  return apiFetch<{ executions: OpsExecution[] }>(`/api/v1/ops/executions${suffix}`);
}

export function getOpsExecution(cluster: string, id: string) {
  const q = new URLSearchParams({ cluster });
  return apiFetch<OpsExecution>(`/api/v1/ops/executions/${encodeURIComponent(id)}?${q}`);
}

export function startOpsExecution(
  body: {
    kind: string;
    cluster: string;
    dryRun?: boolean;
    ref?: string;
    params?: unknown;
  },
  opts?: { headers?: HeadersInit },
) {
  return apiFetch<OpsExecution>("/api/v1/ops/executions", {
    method: "POST",
    headers: opts?.headers,
    body: JSON.stringify(body),
  });
}

export function opsLogsPath(cluster: string, id: string, follow = true): string {
  const q = new URLSearchParams({
    cluster,
    follow: follow ? "1" : "0",
  });
  return `/api/v1/ops/executions/${encodeURIComponent(id)}/logs?${q}`;
}

export function apiURL(path: string): string {
  return new URL(path, apiBase()).toString();
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
