const SLUG_PATTERN = /[^a-z0-9-_]/g;

export function normalizeSlug(value) {
  if (!value) {
    return "";
  }
  const trimmed = value.trim().toLowerCase();
  if (!trimmed) {
    return "";
  }
  return trimmed.replace(SLUG_PATTERN, "");
}
