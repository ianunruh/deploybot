import {
  Accordion,
  Autocomplete,
  MultiSelect,
  Stack,
  Switch,
  TextInput,
} from "@mantine/core";

import type { OpsField } from "~/lib/api.server";

export type OpsParamValues = Record<string, unknown>;

export function OpsKindFields({
  fields,
  values,
  onChange,
}: {
  fields: OpsField[];
  values: OpsParamValues;
  onChange: (values: OpsParamValues) => void;
}) {
  const set = (name: string, value: unknown) => {
    onChange({ ...values, [name]: value });
  };

  return (
    <Stack gap="sm">
      {fields.map((field) => (
        <OpsFieldControl
          key={field.name}
          field={field}
          value={values[field.name]}
          onChange={(value) => set(field.name, value)}
        />
      ))}
    </Stack>
  );
}

function OpsFieldControl({
  field,
  value,
  onChange,
}: {
  field: OpsField;
  value: unknown;
  onChange: (value: unknown) => void;
}) {
  const label = field.title || field.name;
  if (field.type === "multi") {
    const selected = Array.isArray(value)
      ? value.filter((item): item is string => typeof item === "string")
      : [];
    return (
      <MultiSelect
        label={label}
        description={field.description}
        data={field.options ?? []}
        value={selected}
        onChange={onChange}
        required={field.required}
        searchable
      />
    );
  }
  if (field.type === "bool") {
    return (
      <Switch
        label={label}
        description={field.description}
        checked={value === true}
        onChange={(e) => onChange(e.currentTarget.checked)}
      />
    );
  }
  if (field.type === "map") {
    const current =
      value != null && typeof value === "object" && !Array.isArray(value)
        ? (value as Record<string, string>)
        : {};
    const filled = Object.keys(current).length;
    return (
      <Accordion variant="separated" radius="sm">
        <Accordion.Item value={field.name}>
          <Accordion.Control>
            {label}
            {filled > 0 ? ` (${filled})` : ""}
          </Accordion.Control>
          <Accordion.Panel>
            <Stack gap={6}>
              {(field.keys ?? []).map((key) => (
                <TextInput
                  key={key}
                  label={key}
                  description={field.description}
                  value={current[key] ?? ""}
                  onChange={(e) => {
                    const next = { ...current };
                    const v = e.currentTarget.value;
                    if (v === "") delete next[key];
                    else next[key] = v;
                    onChange(next);
                  }}
                />
              ))}
            </Stack>
          </Accordion.Panel>
        </Accordion.Item>
      </Accordion>
    );
  }
  const str = typeof value === "string" ? value : "";
  if (field.suggestions != null && field.suggestions.length > 0) {
    return (
      <Autocomplete
        label={label}
        description={field.description}
        data={field.suggestions}
        value={str}
        onChange={onChange}
        required={field.required}
      />
    );
  }
  return (
    <TextInput
      label={label}
      description={field.description}
      value={str}
      onChange={(e) => onChange(e.currentTarget.value)}
      required={field.required}
    />
  );
}

export function requiredParamsFilled(fields: OpsField[], values: OpsParamValues): boolean {
  for (const field of fields) {
    if (!field.required) continue;
    const v = values[field.name];
    if (v == null || v === "") return false;
    if (Array.isArray(v) && v.length === 0) return false;
  }
  return true;
}

export function cleanParams(values: OpsParamValues): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(values)) {
    if (value == null || value === "") continue;
    if (Array.isArray(value) && value.length === 0) continue;
    if (
      typeof value === "object" &&
      !Array.isArray(value) &&
      Object.keys(value).length === 0
    ) {
      continue;
    }
    out[key] = value;
  }
  return out;
}
