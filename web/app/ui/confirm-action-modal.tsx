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
  size = "md",
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
  message: ReactNode;
}) {
  return (
    <Modal opened={opened} onClose={onClose} title={title} centered size={size}>
      <Stack gap="md">
        {message}
        <Group justify="flex-end">
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
      </Stack>
    </Modal>
  );
}
