import { notifications } from "@mantine/notifications";

export function notifyActionError(title: string, error: string): void {
  console.error(`[deploybot:error] ${title}`, error);
  notifications.show({
    color: "red",
    title,
    message: error,
    autoClose: 12_000,
    withCloseButton: true,
  });
}

export function notifyActionSuccess(title: string, message: string): void {
  notifications.show({
    color: "teal",
    title,
    message,
    autoClose: 4_000,
  });
}
