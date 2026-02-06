import { normalizeSlug } from "../shared/slug.js";
import { normalizeBaseUrl, buildUrl } from "../shared/url.js";
import {
  getBaseUrl,
  getRecentSlugs,
  updateRecentSlug,
  getOptionsAutoOpened,
  setOptionsAutoOpened
} from "../shared/storage.js";

const MAX_SUGGESTIONS = 6;

async function ensureTabsPermission() {
  const hasPermission = await chrome.permissions.contains({ permissions: ["tabs"] });
  if (hasPermission) {
    return true;
  }
  return chrome.permissions.request({ permissions: ["tabs"] });
}

async function maybeOpenOptions() {
  const opened = await getOptionsAutoOpened();
  if (!opened) {
    await chrome.runtime.openOptionsPage();
    await setOptionsAutoOpened(true);
  }
}

async function navigateToUrl(url, disposition) {
  const canUseTabs = await ensureTabsPermission();
  if (!canUseTabs) {
    return;
  }

  if (disposition === "newForegroundTab" || disposition === "newBackgroundTab") {
    await chrome.tabs.create({
      url,
      active: disposition === "newForegroundTab"
    });
    return;
  }

  const [activeTab] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (activeTab && activeTab.id !== undefined) {
    await chrome.tabs.update(activeTab.id, { url });
    return;
  }

  await chrome.tabs.create({ url, active: true });
}

chrome.omnibox.onInputChanged.addListener(async (text, suggest) => {
  const normalized = normalizeSlug(text);
  const recents = await getRecentSlugs();
  const filtered = normalized
    ? recents.filter((item) => item.slug.startsWith(normalized))
    : recents;

  const suggestions = filtered.slice(0, MAX_SUGGESTIONS).map((item) => ({
    content: item.slug,
    description: `Recent: ${item.slug}`
  }));

  suggest(suggestions);
});

chrome.omnibox.onInputEntered.addListener(async (text, disposition) => {
  const slug = normalizeSlug(text);
  if (!slug) {
    return;
  }

  const baseUrl = normalizeBaseUrl(await getBaseUrl());
  if (!baseUrl) {
    await maybeOpenOptions();
    return;
  }

  const targetUrl = buildUrl(baseUrl, slug);
  try {
    await navigateToUrl(targetUrl, disposition);
    await updateRecentSlug(slug);
  } catch (error) {
    // Ignore navigation errors.
  }
});
