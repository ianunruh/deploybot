import { AppShell, Box, Burger, Group, NavLink, Text } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { IconRocket } from "@tabler/icons-react";
import type { ReactNode } from "react";
import { Link, useLocation, useNavigation } from "react-router";

export function AppChrome({ children }: { children: ReactNode }) {
  const [opened, { toggle }] = useDisclosure();
  const location = useLocation();
  const navigation = useNavigation();
  const busy = navigation.state !== "idle";

  return (
    <AppShell
      header={{ height: 48 }}
      navbar={{ width: 220, breakpoint: "sm", collapsed: { mobile: !opened } }}
      padding="md"
    >
      <div className={busy ? "db-top-loading" : "db-top-loading db-top-loading--done"}>
        <div className="db-top-loading__bar" />
      </div>
      <AppShell.Header style={{ background: "#0b0d0f", borderColor: "#1e242c" }} px="md">
        <Group h="100%" justify="space-between">
          <Group gap="sm">
            <Burger opened={opened} onClick={toggle} hiddenFrom="sm" size="sm" />
            <Text fw={600} size="sm" tt="uppercase" style={{ letterSpacing: "0.08em" }}>
              deploybot
            </Text>
          </Group>
        </Group>
      </AppShell.Header>
      <AppShell.Navbar p="xs" style={{ background: "#0b0d0f", borderColor: "#1e242c" }}>
        <NavLink
          component={Link}
          to="/"
          label="Deployables"
          leftSection={<IconRocket size={16} />}
          active={
            location.pathname === "/" || location.pathname.startsWith("/deployables")
          }
        />
      </AppShell.Navbar>
      <AppShell.Main className="db-shell-main">
        <Box maw={1200}>{children}</Box>
      </AppShell.Main>
    </AppShell>
  );
}
