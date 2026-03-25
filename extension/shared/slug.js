const SLUG_PATTERN = /[^\p{L}\p{N}_/-]/gu;

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
