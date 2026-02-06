import { normalizeBaseUrl } from "../shared/url.js";
import { getBaseUrl, setBaseUrl } from "../shared/storage.js";

const DEFAULT_PLACEHOLDER = "https://go.example.com/";

function setStatus(message) {
  const status = document.getElementById("status");
  status.textContent = message;
}

document.addEventListener("DOMContentLoaded", async () => {
  const input = document.getElementById("baseUrl");
  const saveButton = document.getElementById("save");

  const stored = normalizeBaseUrl(await getBaseUrl());
  input.placeholder = stored || DEFAULT_PLACEHOLDER;

  saveButton.addEventListener("click", async () => {
    const raw = input.value.trim();
    if (!raw) {
      setStatus("Base URL required.");
      return;
    }

    const normalized = normalizeBaseUrl(raw);
    if (!normalized) {
      setStatus("Base URL required.");
      return;
    }

    await setBaseUrl(normalized);
    input.value = "";
    input.placeholder = normalized;
    setStatus("Saved.");
    setTimeout(() => setStatus(""), 1500);
  });
});
