import { Anchor, Breadcrumbs, Group, Stack, Text, Title } from "@mantine/core";
import type { ReactNode } from "react";
import { Link } from "react-router";

export type PageCrumb = {
  label: string;
  to?: string;
};

export function PageHeader({
  title,
  description,
  actions,
  crumbs,
}: {
  title: string;
  description?: ReactNode;
  actions?: ReactNode;
  crumbs?: PageCrumb[];
}) {
  return (
    <Stack gap={8}>
      {crumbs != null && crumbs.length > 0 && (
        <Breadcrumbs>
          {crumbs.map((crumb) =>
            crumb.to != null ? (
              <Anchor
                key={`${crumb.label}:${crumb.to}`}
                component={Link}
                to={crumb.to}
                size="sm"
                c="var(--db-link)"
                underline="hover"
              >
                {crumb.label}
              </Anchor>
            ) : (
              <Text key={crumb.label} size="sm" c="dimmed">
                {crumb.label}
              </Text>
            ),
          )}
        </Breadcrumbs>
      )}
      <Group justify="space-between" align="flex-end" wrap="wrap" gap="md">
        <Stack gap={4}>
          <Title order={2} size="h3">
            {title}
          </Title>
          {typeof description === "string" ? (
            <Text size="sm" c="dimmed">
              {description}
            </Text>
          ) : (
            description
          )}
        </Stack>
        {actions != null && <Group gap="sm">{actions}</Group>}
      </Group>
    </Stack>
  );
}
