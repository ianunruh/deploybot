import type { PauseEntry, PauseFile } from "~/lib/api.server";

export type PauseScope = "all" | "stage" | "app" | "app-stage";

export type PauseHit = PauseEntry & {
  scope: PauseScope;
  app?: string;
  stage?: string;
};

export function pauseHit(
  pause: PauseFile | undefined,
  app: string,
  stage: string,
): PauseHit | null {
  if (pause == null) return null;
  const appPause = pause.apps?.[app];
  if (appPause?.stages?.[stage] != null) {
    return { ...appPause.stages[stage], scope: "app-stage", app, stage };
  }
  if (appPause?.at) {
    return {
      at: appPause.at,
      by: appPause.by,
      reason: appPause.reason,
      scope: "app",
      app,
    };
  }
  if (pause.stages?.[stage] != null) {
    return { ...pause.stages[stage], scope: "stage", stage };
  }
  if (pause.all != null) {
    return { ...pause.all, scope: "all" };
  }
  return null;
}

export function visiblePauses(
  pause: PauseFile | undefined,
  app?: string,
  stages?: string[],
): PauseHit[] {
  if (pause == null) return [];
  const out: PauseHit[] = [];
  if (pause.all != null) out.push({ ...pause.all, scope: "all" });
  for (const [stage, entry] of Object.entries(pause.stages ?? {})) {
    if (stages != null && !stages.includes(stage)) continue;
    out.push({ ...entry, scope: "stage", stage });
  }
  if (app) {
    const a = pause.apps?.[app];
    if (a?.at) {
      out.push({ at: a.at, by: a.by, reason: a.reason, scope: "app", app });
    }
    for (const [stage, entry] of Object.entries(a?.stages ?? {})) {
      out.push({ ...entry, scope: "app-stage", app, stage });
    }
  }
  return out;
}

export function pauseTitle(hit: PauseHit): string {
  switch (hit.scope) {
    case "all":
      return "All deployments paused";
    case "stage":
      return `${hit.stage} paused for every app`;
    case "app":
      return `${hit.app} paused`;
    case "app-stage":
      return `${hit.app}/${hit.stage} paused`;
  }
}

export function pauseSelector(hit: PauseHit): { name?: string; stage?: string } {
  switch (hit.scope) {
    case "all":
      return {};
    case "stage":
      return { stage: hit.stage };
    case "app":
      return { name: hit.app };
    case "app-stage":
      return { name: hit.app, stage: hit.stage };
  }
}
