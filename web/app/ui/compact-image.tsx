import { Text } from "@mantine/core";

export function CompactImage({ value, empty = "—" }: { value?: string; empty?: string }) {
  if (!value) {
    return (
      <Text size="xs" c="dimmed" span>
        {empty}
      </Text>
    );
  }
  const at = value.indexOf("@");
  let tag = at >= 0 ? value.slice(0, at) : value;
  const colon = tag.lastIndexOf(":");
  const slash = tag.lastIndexOf("/");
  if (colon > slash && colon >= 0) {
    tag = tag.slice(colon + 1);
  }
  return (
    <Text className="db-clip-text" size="xs" ff="monospace" title={value} span>
      {tag}
    </Text>
  );
}

export function shortDigest(digest?: string): string {
  if (!digest) return "";
  const hex = digest.replace(/^sha256:/, "");
  return hex ? `sha256:${hex.slice(0, 12)}` : digest;
}

export function releaseImageRef(
  repository: string,
  rel: { tag?: string; digest?: string },
): string {
  let s = repository;
  if (rel.tag) s += `:${rel.tag}`;
  if (rel.digest) s += `@${rel.digest}`;
  return s;
}

export function releaseMatchesImage(
  rel: { image?: string; digest?: string },
  stageImage?: string,
): boolean {
  if (!stageImage) return false;
  if (rel.image && stageImage === rel.image) return true;
  if (!rel.digest) return false;
  const hex = rel.digest.replace(/^sha256:/, "").slice(0, 12);
  return hex.length > 0 && stageImage.includes(hex);
}
