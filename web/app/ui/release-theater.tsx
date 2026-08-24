import { useEffect, useState } from "react";
import { Alert, Badge, Button, Group, Stack, Text } from "@mantine/core";
import {
  IconCheck,
  IconCircle,
  IconCloudUpload,
  IconGitCommit,
  IconHeart,
  IconLoader2,
  IconMinus,
  IconRefresh,
  IconServer,
  IconX,
} from "@tabler/icons-react";

import type { MutationResult, PodLive, StageStatus } from "~/lib/api.server";
import { CompactImage } from "~/ui/compact-image";
import { ConsolePaper } from "~/ui/console-paper";
import { DiffPanel } from "~/ui/diff-panel";
import { RelativeTime } from "~/ui/relative-time";
import { ResourceTable, Table } from "~/ui/resource-table";
import { ReplicaReady, StatusBadge } from "~/ui/status-badge";

export const THEATER_TIMEOUT_MS = 5 * 60 * 1000;
const NOOP_GRACE_MS = 3_000;

const FAIL_POD = /crash|error|imagepull|oomkilled|exitcode|signal/i;
const ROLLING_POD = /pending|creating|initializing|terminating|^init:/i;

export type TheaterKind = "pin" | "promote";

export type TheaterSession = {
  kind: TheaterKind;
  stage: string;
  image: string;
  startedAt: number;
  apply: boolean;
  push: boolean;
  sync: boolean;
  initialPodNames: string[];
  result?: MutationResult;
  error?: string;
};

export type StepState = "pending" | "running" | "done" | "failed" | "skipped";

export type TheaterStep = {
  id: "commit" | "push" | "sync" | "rollout" | "healthy";
  label: string;
  state: StepState;
  detail?: string;
};

export type TheaterPhase = "running" | "done" | "failed";

export function podFailed(status: string): boolean {
  return FAIL_POD.test(status);
}

export function podRolling(status: string): boolean {
  return ROLLING_POD.test(status);
}

export function formatElapsed(ms: number): string {
  const sec = Math.max(0, Math.floor(ms / 1000));
  if (sec < 60) return `${sec}s`;
  return `${Math.floor(sec / 60)}m ${String(sec % 60).padStart(2, "0")}s`;
}

export function shortCommit(sha?: string): string {
  if (!sha) return "";
  return sha.slice(0, 8);
}

export function theaterSteps(input: {
  session: TheaterSession;
  submitting: boolean;
  stage?: StageStatus;
  now: number;
}): { steps: TheaterStep[]; phase: TheaterPhase } {
  const { session, submitting, stage, now } = input;
  const result = session.result;
  const error = session.error;
  const elapsed = now - session.startedAt;
  const timedOut = elapsed >= THEATER_TIMEOUT_MS;

  const commit: TheaterStep = {
    id: "commit",
    label: "Commit",
    state: "pending",
  };
  if (error && !result?.commit) {
    commit.state = "failed";
    commit.detail = error;
  } else if (result?.commit) {
    commit.state = "done";
    commit.detail = shortCommit(result.commit);
  } else if (submitting) {
    commit.state = "running";
    commit.detail = "writing overlay";
  }

  const push: TheaterStep = { id: "push", label: "Push", state: "pending" };
  if (!session.push) {
    push.state = "skipped";
    push.detail = "local only";
  } else if (commit.state === "failed") {
    push.state = "pending";
  } else if (result?.pushed) {
    push.state = "done";
    push.detail = result.ref;
  } else if (result && !result.pushed) {
    push.state = "skipped";
  } else if (submitting && commit.state === "running") {
    push.state = "pending";
  } else if (submitting) {
    push.state = "running";
  }

  const sync: TheaterStep = { id: "sync", label: "Argo sync", state: "pending" };
  if (!session.sync) {
    sync.state = "skipped";
    sync.detail = "not requested";
  } else if (commit.state === "failed") {
    sync.state = "pending";
  } else if (result?.synced) {
    sync.state = "done";
    sync.detail = stage?.sync && stage.sync !== "unknown" ? stage.sync : "triggered";
  } else if (result && !result.synced) {
    sync.state = "skipped";
  } else if (submitting && commit.state === "done") {
    sync.state = "running";
  }

  const rollout = rolloutStep({
    session,
    submitting,
    stage,
    commitFailed: commit.state === "failed",
    timedOut,
    now,
  });
  const healthy = healthyStep({
    session,
    stage,
    rollout,
    commitFailed: commit.state === "failed",
    timedOut,
  });

  const steps = [commit, push, sync, rollout, healthy];
  const phase: TheaterPhase = steps.some((st) => st.state === "failed")
    ? "failed"
    : steps.every((st) => st.state === "done" || st.state === "skipped")
      ? "done"
      : "running";
  return { steps, phase };
}

function rolloutStep(input: {
  session: TheaterSession;
  submitting: boolean;
  stage?: StageStatus;
  commitFailed: boolean;
  timedOut: boolean;
  now: number;
}): TheaterStep {
  const step: TheaterStep = { id: "rollout", label: "Rollout", state: "pending" };
  if (!input.session.sync) {
    step.state = "skipped";
    return step;
  }
  if (input.commitFailed) return step;
  if (!input.session.result) {
    if (input.submitting) step.detail = "waiting for commit";
    return step;
  }
  const workload = input.stage?.workload;
  const pods = workload?.pods ?? [];
  const failed = pods.find((p) => podFailed(p.status));
  if (failed) {
    step.state = "failed";
    step.detail = `${failed.name} ${failed.status}`;
    return step;
  }
  const desired = workload?.desired ?? 0;
  const ready = workload?.ready ?? 0;
  const readyNow = desired > 0 && ready >= desired;
  const rolling = pods.some((p) => podRolling(p.status));
  const newPods = pods.filter((p) => !input.session.initialPodNames.includes(p.name));
  const oldLeft = pods.filter((p) => input.session.initialPodNames.includes(p.name));
  if (newPods.length > 0) {
    step.detail = replicaDetail(ready, desired, newPods.length);
    if (
      readyNow &&
      !rolling &&
      oldLeft.every((p) => podRolling(p.status) || p.status === "Terminating")
    ) {
      step.state = "done";
      return step;
    }
    if (readyNow && oldLeft.length === 0 && pods.every((p) => p.status === "Running")) {
      step.state = "done";
      return step;
    }
    step.state = "running";
    return step;
  }
  if (rolling) {
    step.state = "running";
    step.detail = replicaDetail(ready, desired, 0);
    return step;
  }
  const deployedAt = input.stage?.deployedAt ? Date.parse(input.stage.deployedAt) : NaN;
  const deployedAfter =
    Number.isFinite(deployedAt) && deployedAt + 2000 >= input.session.startedAt;
  const elapsed = input.now - input.session.startedAt;
  if (
    readyNow &&
    pods.every((p) => p.status === "Running" || p.status === "Succeeded") &&
    (deployedAfter || /^healthy$/i.test(input.stage?.health ?? "")) &&
    elapsed >= NOOP_GRACE_MS
  ) {
    step.state = "done";
    step.detail = replicaDetail(ready, desired, 0);
    return step;
  }
  if (input.timedOut) {
    step.state = "failed";
    step.detail = "timed out waiting for a new replica";
    return step;
  }
  step.state = "running";
  step.detail = readyNow ? "waiting for new replica" : replicaDetail(ready, desired, 0);
  return step;
}

function healthyStep(input: {
  session: TheaterSession;
  stage?: StageStatus;
  rollout: TheaterStep;
  commitFailed: boolean;
  timedOut: boolean;
}): TheaterStep {
  const step: TheaterStep = { id: "healthy", label: "Healthy", state: "pending" };
  if (!input.session.sync) {
    step.state = "skipped";
    return step;
  }
  if (input.commitFailed) return step;
  if (!input.session.result) return step;
  const health = input.stage?.health ?? "";
  if (input.rollout.state === "failed") {
    step.state = "failed";
    step.detail = input.stage?.message || health || "rollout failed";
    return step;
  }
  if (/^(degraded|missing)$/i.test(health)) {
    step.state = "failed";
    step.detail = input.stage?.message || health;
    return step;
  }
  if (/^healthy$/i.test(health) && input.rollout.state === "done") {
    step.state = "done";
    step.detail = input.stage?.name;
    return step;
  }
  if (input.timedOut) {
    step.state = "failed";
    step.detail = health ? `last ${health}` : "timed out";
    return step;
  }
  step.state = "running";
  step.detail = health && health !== "unknown" ? health : "waiting";
  return step;
}

function replicaDetail(ready: number, desired: number, incoming: number): string {
  const replicas = desired > 0 ? `${ready}/${desired}` : "pods";
  if (incoming > 0) return `${replicas} · ${incoming} new`;
  return replicas;
}

const STEP_ICON = {
  commit: IconGitCommit,
  push: IconCloudUpload,
  sync: IconRefresh,
  rollout: IconServer,
  healthy: IconHeart,
} as const;

export function ReleaseTheater({
  session,
  submitting,
  stage,
  onDismiss,
}: {
  session: TheaterSession;
  submitting: boolean;
  stage?: StageStatus;
  onDismiss: () => void;
}) {
  const [now, setNow] = useState(() => Date.now());
  const { steps, phase } = theaterSteps({ session, submitting, stage, now });

  useEffect(() => {
    if (phase !== "running") return;
    const id = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, [phase]);

  const title =
    session.kind === "promote" ? `Promote to ${session.stage}` : `Pin ${session.stage}`;
  const showPods = session.sync && (session.result != null || submitting);

  return (
    <ConsolePaper className={`db-theater db-theater--${phase}`}>
      <Stack gap="md">
        <Group justify="space-between" align="flex-start" wrap="wrap" gap="sm">
          <Stack gap={4}>
            <Text size="sm" tt="uppercase" c="dimmed" fw={600}>
              Release
            </Text>
            <Group gap="xs" wrap="wrap">
              <Text size="sm" fw={600}>
                {title}
              </Text>
              <CompactImage value={session.image} empty="" />
            </Group>
          </Stack>
          <Group gap="sm">
            <Text size="xs" c="dimmed">
              {formatElapsed(now - session.startedAt)}
            </Text>
            <Button variant="default" size="compact-sm" onClick={onDismiss}>
              Dismiss
            </Button>
          </Group>
        </Group>

        <div className="db-theater-steps">
          {steps.map((step, i) => (
            <TheaterStepView key={step.id} step={step} last={i === steps.length - 1} />
          ))}
        </div>

        {session.error ? (
          <Alert
            color="red"
            title={session.kind === "promote" ? "Promote failed" : "Pin failed"}
          >
            {session.error}
          </Alert>
        ) : null}

        {phase === "done" ? (
          <Text size="sm" c="teal.4">
            {session.sync
              ? `${session.stage} is healthy.`
              : session.push
                ? "Committed and pushed."
                : "Committed locally."}
          </Text>
        ) : null}

        {showPods ? (
          <TheaterPods
            stage={stage}
            initialPodNames={session.initialPodNames}
            startedAt={session.startedAt}
          />
        ) : null}

        {session.result?.diff ? (
          <DiffPanel diff={session.result.diff} title="Overlay diff" maxHeight={220} />
        ) : null}
      </Stack>
    </ConsolePaper>
  );
}

function TheaterStepView({ step, last }: { step: TheaterStep; last: boolean }) {
  const Glyph = STEP_ICON[step.id];
  return (
    <>
      <Stack gap={4} className={`db-theater-step db-theater-step--${step.state}`}>
        <Group gap={6} wrap="nowrap">
          <StepBullet state={step.state} />
          <Glyph size={14} stroke={1.5} />
          <Text size="sm" fw={600}>
            {step.label}
          </Text>
        </Group>
        {step.detail ? (
          <Text size="xs" c="dimmed" className="db-clip-text" title={step.detail}>
            {step.detail}
          </Text>
        ) : (
          <Text size="xs" c="dimmed">
            {step.state === "skipped" ? "skipped" : "\u00a0"}
          </Text>
        )}
      </Stack>
      {last ? null : (
        <div className="db-theater-edge">
          <div className="db-theater-edge-rule" />
        </div>
      )}
    </>
  );
}

function StepBullet({ state }: { state: StepState }) {
  if (state === "running") {
    return <IconLoader2 size={14} stroke={1.75} className="db-spin" />;
  }
  if (state === "done") {
    return <IconCheck size={14} stroke={2} color="var(--mantine-color-teal-5)" />;
  }
  if (state === "failed") {
    return <IconX size={14} stroke={2} color="var(--mantine-color-red-5)" />;
  }
  if (state === "skipped") {
    return <IconMinus size={14} stroke={1.5} color="var(--mantine-color-dimmed)" />;
  }
  return <IconCircle size={14} stroke={1.5} color="var(--mantine-color-dimmed)" />;
}

function TheaterPods({
  stage,
  initialPodNames,
  startedAt,
}: {
  stage?: StageStatus;
  initialPodNames: string[];
  startedAt: number;
}) {
  const workload = stage?.workload;
  const pods = workload?.pods ?? [];
  return (
    <Stack gap="sm">
      <Group justify="space-between" align="baseline">
        <Text size="sm" tt="uppercase" c="dimmed" fw={600}>
          Pods
        </Text>
        <Group gap="sm">
          {workload ? (
            <ReplicaReady ready={workload.ready} desired={workload.desired} />
          ) : null}
          {stage?.logsURL ? (
            <Button
              component="a"
              href={stage.logsURL}
              target="_blank"
              rel="noreferrer"
              variant="subtle"
              size="compact-xs"
            >
              Logs
            </Button>
          ) : null}
          {stage?.argoURL ? (
            <Button
              component="a"
              href={stage.argoURL}
              target="_blank"
              rel="noreferrer"
              variant="subtle"
              size="compact-xs"
            >
              Argo
            </Button>
          ) : null}
        </Group>
      </Group>
      {workload?.message ? (
        <Text size="xs" c="red.4">
          {workload.message}
        </Text>
      ) : null}
      <ResourceTable
        headers={["Name", "Ready", "Status", "Age", ""]}
        isEmpty={pods.length === 0}
        emptyMessage="Waiting for pods…"
        minWidth={560}
      >
        {pods.map((p) => (
          <PodRow
            key={p.name}
            pod={p}
            isNew={!initialPodNames.includes(p.name)}
            startedAt={startedAt}
          />
        ))}
      </ResourceTable>
    </Stack>
  );
}

function PodRow({
  pod,
  isNew,
  startedAt,
}: {
  pod: PodLive;
  isNew: boolean;
  startedAt: number;
}) {
  const created = pod.createdAt ? Date.parse(pod.createdAt) : NaN;
  const bornDuring = Number.isFinite(created) && created >= startedAt - 2000;
  return (
    <Table.Tr>
      <Table.Td className="db-cell-clip">
        <Text className="db-clip-text" size="sm" ff="monospace" title={pod.name}>
          {pod.name}
        </Text>
      </Table.Td>
      <Table.Td className="db-cell-fit">{pod.ready || "—"}</Table.Td>
      <Table.Td className="db-cell-fit">
        <StatusBadge status={pod.status} />
      </Table.Td>
      <Table.Td className="db-cell-fit">
        <RelativeTime value={pod.createdAt} size="xs" />
      </Table.Td>
      <Table.Td className="db-cell-fit">
        {isNew || bornDuring ? (
          <Badge size="xs" variant="light" color="accent" tt="none">
            new
          </Badge>
        ) : podRolling(pod.status) ? (
          <Badge size="xs" variant="light" color="orange" tt="none">
            outgoing
          </Badge>
        ) : null}
      </Table.Td>
    </Table.Tr>
  );
}
