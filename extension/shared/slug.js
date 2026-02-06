const SLUG_PATTERN = /[^a-z0-9_/-]/g;

export function normalizeSlug(value) {
  if (!value) {
    return "";
  }
  const trimmed = value.trim().toLowerCase();
  if (!trimmed) {
    return "";
  }
  const withoutSlashes = trimmed.replace(/^\/+|\/+$/g, "");
  return withoutSlashes.replace(SLUG_PATTERN, "");
}
