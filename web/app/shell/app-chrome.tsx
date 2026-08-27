import {
  ActionIcon,
  AppShell,
  Box,
  Burger,
  Group,
  NavLink,
  Text,
  Tooltip,
  useMantineColorScheme,
} from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import {
  IconApps,
  IconHistory,
  IconMoon,
  IconRefresh,
  IconSun,
  IconTerminal2,
} from "@tabler/icons-react";
import type { ReactNode } from "react";
import { Link, useLocation, useNavigation } from "react-router";

function ColorSchemeToggle() {
  const { toggleColorScheme } = useMantineColorScheme();

  return (
    <Tooltip label="Toggle light/dark mode" withArrow>
      <ActionIcon
        variant="subtle"
        color="gray"
        size="sm"
        aria-label="Toggle light/dark mode"
        onClick={() => toggleColorScheme()}
      >
        <Box component={IconSun} w={16} h={16} lightHidden />
        <Box component={IconMoon} w={16} h={16} darkHidden />
      </ActionIcon>
    </Tooltip>
  );
}

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
      <AppShell.Header className="db-shell-chrome" px="md">
        <Group h="100%" justify="space-between">
          <Group gap="sm">
            <Burger opened={opened} onClick={toggle} hiddenFrom="sm" size="sm" />
            <Text fw={600} size="sm" tt="uppercase" style={{ letterSpacing: "0.08em" }}>
              deploybot
            </Text>
          </Group>
          <ColorSchemeToggle />
        </Group>
      </AppShell.Header>
      <AppShell.Navbar p="xs" className="db-shell-chrome">
        <NavLink
          component={Link}
          to="/"
          label="Catalog"
          leftSection={<IconApps size={16} />}
          active={
            location.pathname === "/" || location.pathname.startsWith("/deployables")
          }
        />
        <NavLink
          component={Link}
          to="/history"
          label="History"
          leftSection={<IconHistory size={16} />}
          active={
            location.pathname === "/history" || location.pathname.startsWith("/history")
          }
        />
        <NavLink
          component={Link}
          to="/updates"
          label="Updates"
          leftSection={<IconRefresh size={16} />}
          active={
            location.pathname === "/updates" || location.pathname.startsWith("/updates")
          }
        />
        <NavLink
          component={Link}
          to="/ops"
          label="Ops"
          leftSection={<IconTerminal2 size={16} />}
          active={location.pathname === "/ops" || location.pathname.startsWith("/ops")}
        />
      </AppShell.Navbar>
      <AppShell.Main className="db-shell-main">
        <Box maw={1200}>{children}</Box>
      </AppShell.Main>
    </AppShell>
  );
}
