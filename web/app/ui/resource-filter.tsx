import { CloseButton, Group, Select, TextInput } from "@mantine/core";
import { IconSearch } from "@tabler/icons-react";
import { useSearchParams } from "react-router";

export type ResourceFilters = {
  name: string;
  namespace: string;
  project: string;
};

const PARAMS = ["name", "namespace", "project"] as const;

export function useResourceFilters(): [ResourceFilters, (next: ResourceFilters) => void] {
  const [searchParams, setSearchParams] = useSearchParams();
  const filters: ResourceFilters = {
    name: searchParams.get("name") ?? "",
    namespace: searchParams.get("namespace") ?? "",
    project: searchParams.get("project") ?? "",
  };

  function setFilters(next: ResourceFilters) {
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev);
        for (const key of PARAMS) {
          const value = next[key].trim();
          if (value) params.set(key, value);
          else params.delete(key);
        }
        return params;
      },
      { replace: true },
    );
  }

  return [filters, setFilters];
}

export function matchesResourceFilters(
  item: { name: string; namespace?: string; project?: string },
  filters: ResourceFilters,
): boolean {
  const q = filters.name.trim().toLowerCase();
  if (q && !item.name.toLowerCase().includes(q)) return false;
  if (filters.namespace && item.namespace !== filters.namespace) return false;
  if (filters.project && item.project !== filters.project) return false;
  return true;
}

export function uniqueSorted(values: Array<string | undefined | null>): string[] {
  const set = new Set<string>();
  for (const v of values) {
    const s = v?.trim();
    if (s) set.add(s);
  }
  return [...set].sort((a, b) => a.localeCompare(b));
}

export function updatesNameHref(name: string): string {
  return `/updates?${new URLSearchParams({ name })}`;
}

export function ResourceFilterBar({
  value,
  onChange,
  namespaces,
  projects,
}: {
  value: ResourceFilters;
  onChange: (next: ResourceFilters) => void;
  namespaces: string[];
  projects: string[];
}) {
  return (
    <Group gap="sm" align="flex-end" wrap="wrap">
      <TextInput
        aria-label="Filter by name"
        placeholder="Filter by name"
        value={value.name}
        onChange={(event) => onChange({ ...value, name: event.currentTarget.value })}
        leftSection={<IconSearch size={16} />}
        rightSection={
          value.name ? (
            <CloseButton
              size="sm"
              aria-label="Clear name filter"
              onClick={() => onChange({ ...value, name: "" })}
            />
          ) : undefined
        }
        style={{ flex: "1 1 16rem" }}
        size="sm"
      />
      <Select
        aria-label="Namespace"
        placeholder="Namespace"
        data={namespaces}
        value={value.namespace || null}
        onChange={(next) => onChange({ ...value, namespace: next ?? "" })}
        clearable
        searchable
        nothingFoundMessage="No namespaces"
        size="sm"
        w={180}
      />
      <Select
        aria-label="Project"
        placeholder="Project"
        data={projects}
        value={value.project || null}
        onChange={(next) => onChange({ ...value, project: next ?? "" })}
        clearable
        searchable
        nothingFoundMessage="No projects"
        size="sm"
        w={160}
      />
    </Group>
  );
}
