import { Button, Group, Modal, Stack } from "@mantine/core";
import type { ReactNode } from "react";

export function ConfirmActionModal({
  opened,
  onClose,
  onConfirm,
  loading,
  title,
  confirmLabel = "Confirm",
  confirmColor = "accent",
  confirmDisabled,
  size = "lg",
  argoURL,
  message,
}: {
  opened: boolean;
  onClose: () => void;
  onConfirm: () => void;
  loading?: boolean;
  title: string;
  confirmLabel?: string;
  confirmColor?: string;
  confirmDisabled?: boolean;
  size?: string;
  argoURL?: string;
  message: ReactNode;
}) {
  return (
    <Modal opened={opened} onClose={onClose} title={title} centered size={size}>
      <Stack gap="md">
        {message}
        <Group>
          {argoURL ? (
            <Button
              component="a"
              href={argoURL}
              target="_blank"
              rel="noreferrer"
              variant="default"
            >
              Open Argo
            </Button>
          ) : null}
          <Group gap="sm" ml="auto">
            <Button variant="default" onClick={onClose} disabled={loading}>
              Cancel
            </Button>
            <Button
              color={confirmColor}
              loading={loading}
              disabled={confirmDisabled}
              onClick={onConfirm}
            >
              {confirmLabel}
            </Button>
          </Group>
        </Group>
      </Stack>
    </Modal>
  );
}
