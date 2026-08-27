import { Alert, Button, Group, Select, Stack, Text, TextInput } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { useState } from "react";
import { useFetcher, useRevalidator } from "react-router";

import { notifyActionError, notifyActionSuccess } from "~/lib/action-feedback";
import type { PauseActionData, PauseFile } from "~/lib/api.server";
import { pauseSelector, pauseTitle, visiblePauses, type PauseHit } from "~/lib/pause";
import { useFetcherResult } from "~/lib/use-fetcher-result";
import { ConfirmActionModal } from "~/ui/confirm-action-modal";
import { RelativeTime } from "~/ui/relative-time";

export function PauseBanners({
  pause,
  app,
  stages,
  action,
}: {
  pause?: PauseFile;
  app?: string;
  stages?: string[];
  action: string;
}) {
  const fetcher = useFetcher<PauseActionData>();
  const revalidator = useRevalidator();
  const hits = visiblePauses(pause, app, app ? stages : undefined);

  useFetcherResult(fetcher, (data) => {
    if (!data.ok) {
      notifyActionError("Unpause failed", data.error);
      return;
    }
    notifyActionSuccess(
      "Unpaused",
      data.result?.dryRun ? "Previewed (dry-run)" : "Wrote pause file",
    );
    void revalidator.revalidate();
  });

  if (hits.length === 0) return null;

  return (
    <Stack gap="sm">
      {hits.map((hit) => (
        <PauseBanner
          key={`${hit.scope}:${hit.app ?? ""}:${hit.stage ?? ""}`}
          hit={hit}
          loading={fetcher.state !== "idle"}
          onUnpause={() => {
            const sel = pauseSelector(hit);
            void fetcher.submit(
              {
                intent: "unpause",
                ...(sel.name ? { name: sel.name } : {}),
                ...(sel.stage ? { stage: sel.stage } : {}),
              },
              { method: "post", action },
            );
          }}
        />
      ))}
    </Stack>
  );
}

function PauseBanner({
  hit,
  loading,
  onUnpause,
}: {
  hit: PauseHit;
  loading: boolean;
  onUnpause: () => void;
}) {
  return (
    <Alert color="orange" title={pauseTitle(hit)}>
      <Group justify="space-between" align="center" gap="sm" wrap="wrap">
        <Text size="sm">
          {hit.reason ? `${hit.reason}. ` : null}
          {hit.by ? `Paused by ${hit.by}` : "Paused"}
          {hit.at ? (
            <>
              {" "}
              <RelativeTime value={hit.at} />
            </>
          ) : null}
          .
        </Text>
        <Button size="compact-sm" variant="default" loading={loading} onClick={onUnpause}>
          Unpause
        </Button>
      </Group>
    </Alert>
  );
}

export function PauseButton({
  app,
  stages,
  action,
}: {
  app?: string;
  stages: string[];
  action: string;
}) {
  const fetcher = useFetcher<PauseActionData>();
  const revalidator = useRevalidator();
  const [open, handlers] = useDisclosure(false);
  const [reason, setReason] = useState("");
  const defaultScope = app ? "app" : "all";
  const [scope, setScope] = useState(defaultScope);

  useFetcherResult(fetcher, (data) => {
    if (!data.ok) {
      notifyActionError("Pause failed", data.error);
      return;
    }
    const note = data.result?.dryRun ? " (dry-run)" : "";
    notifyActionSuccess("Paused", `Wrote pause file${note}`);
    handlers.close();
    setReason("");
    setScope(defaultScope);
    void revalidator.revalidate();
  });

  return (
    <>
      <Button variant="default" onClick={handlers.open}>
        Pause
      </Button>
      <ConfirmActionModal
        opened={open}
        onClose={() => {
          handlers.close();
          setReason("");
          setScope(defaultScope);
        }}
        loading={fetcher.state !== "idle"}
        title={app ? `Pause ${app}` : "Pause deployments"}
        confirmLabel="Pause"
        confirmColor="orange"
        message={
          <Stack gap="sm">
            <Select
              label="Scope"
              data={pauseScopeOptions(app, stages)}
              value={scope}
              allowDeselect={false}
              onChange={(v) => setScope(v ?? defaultScope)}
            />
            <TextInput
              label="Reason"
              placeholder="optional"
              value={reason}
              onChange={(e) => setReason(e.currentTarget.value)}
              maxLength={160}
            />
            <Text size="sm" c="dimmed">
              Blocks auto-pin and GitHub Actions pin-homelab (HTTP 409). Console pin and
              promote still work. Rollback still works.
            </Text>
          </Stack>
        }
        onConfirm={() => {
          const sel = parseScope(scope, app);
          void fetcher.submit(
            {
              intent: "pause",
              ...(sel.name ? { name: sel.name } : {}),
              ...(sel.stage ? { stage: sel.stage } : {}),
              ...(reason.trim() ? { reason: reason.trim() } : {}),
            },
            { method: "post", action },
          );
        }}
      />
    </>
  );
}

function pauseScopeOptions(
  app: string | undefined,
  stages: string[],
): { value: string; label: string }[] {
  if (app) {
    return [
      { value: "app", label: `All stages of ${app}` },
      ...stages.map((st) => ({ value: `app-stage:${st}`, label: `${app}/${st}` })),
    ];
  }
  return [
    { value: "all", label: "All apps, all stages" },
    ...stages.map((st) => ({ value: `stage:${st}`, label: `${st} (every app)` })),
  ];
}

function parseScope(value: string, app?: string): { name?: string; stage?: string } {
  if (value === "all") return {};
  if (value === "app") return { name: app };
  if (value.startsWith("stage:")) return { stage: value.slice("stage:".length) };
  if (value.startsWith("app-stage:")) {
    return { name: app, stage: value.slice("app-stage:".length) };
  }
  return {};
}
