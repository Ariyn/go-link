const STORAGE_KEYS = {
  baseUrl: "baseUrl",
  recentSlugs: "recentSlugs",
  optionsOpened: "optionsOpened"
};

const MAX_RECENTS = 20;

export async function getBaseUrl() {
  const result = await chrome.storage.sync.get(STORAGE_KEYS.baseUrl);
  return result[STORAGE_KEYS.baseUrl] || "";
}

export async function setBaseUrl(value) {
  await chrome.storage.sync.set({ [STORAGE_KEYS.baseUrl]: value });
}

export async function getRecentSlugs() {
  const result = await chrome.storage.sync.get(STORAGE_KEYS.recentSlugs);
  const stored = result[STORAGE_KEYS.recentSlugs];
  return Array.isArray(stored) ? stored : [];
}

export async function updateRecentSlug(slug) {
  const recents = await getRecentSlugs();
  const filtered = recents.filter((item) => item.slug !== slug);
  filtered.unshift({ slug, ts: Date.now() });
  const next = filtered.slice(0, MAX_RECENTS);
  await chrome.storage.sync.set({ [STORAGE_KEYS.recentSlugs]: next });
}

export async function getOptionsAutoOpened() {
  const result = await chrome.storage.sync.get(STORAGE_KEYS.optionsOpened);
  return Boolean(result[STORAGE_KEYS.optionsOpened]);
}

export async function setOptionsAutoOpened(value) {
  await chrome.storage.sync.set({ [STORAGE_KEYS.optionsOpened]: value });
}
