import { Checkbox, Text } from "@mantine/core";

import type { MutationResult } from "~/lib/api.server";

export function mutationCommitLabel(
  status: { apply: boolean; push: boolean },
  action: string,
  syncArgo: boolean,
): string {
  if (!status.apply) return `Preview ${action}`;
  if (syncArgo) return "Commit and sync Argo";
  if (status.push) return `Commit and push ${action}`;
  return `Commit ${action}`;
}

export function mutationNote(
  result?: MutationResult,
  opts?: { argoAvailable?: boolean },
): string {
  if (result?.dryRun) return " (dry-run)";
  let note = "";
  if (result?.pushed && !result?.synced) note += " and pushed";
  if (result?.synced) note += " and synced Argo";
  else if (opts?.argoAvailable) note += " (Argo not synced)";
  return note;
}

export function formFlag(form: FormData, key: string): boolean | undefined {
  const v = form.get(key);
  if (v == null || String(v) === "") return undefined;
  return String(v) !== "false" && String(v) !== "0";
}

export function MutationGitHint({
  apply,
  push,
  sync,
  syncStage,
}: {
  apply: boolean;
  push: boolean;
  sync: boolean;
  syncStage?: string;
}) {
  let text: string;
  if (!apply) {
    text = "Previews a git diff and does not commit.";
  } else if (!push) {
    text = "Commits locally and does not push.";
  } else {
    text = "Commits and pushes the current branch. Never force-pushes.";
  }
  if (apply) {
    if (sync) {
      const where = syncStage ? ` on ${syncStage}` : "";
      text += ` Then syncs Argo CD${where} and waits until healthy.`;
    } else {
      text += " Does not sync Argo CD.";
    }
  }
  return (
    <Text size="sm" c="dimmed">
      {text}
    </Text>
  );
}

export function ArgoSyncCheckbox({
  show,
  checked,
  onChange,
  stage,
}: {
  show: boolean;
  checked: boolean;
  onChange: (checked: boolean) => void;
  stage?: string;
}) {
  if (!show) return null;
  const where = stage ? ` on ${stage}` : "";
  return (
    <Checkbox
      checked={checked}
      onChange={(e) => onChange(e.currentTarget.checked)}
      label={`Sync Argo CD${where} after commit`}
      description="Waits until the app is healthy. Uncheck to review the Argo diff first."
    />
  );
}
