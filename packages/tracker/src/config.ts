import type { TrackerConfig } from "./types";

export const defaultEndpoint = "http://localhost:8080/v1/analytics/events";

export function configFromScript(document: Document): TrackerConfig | null {
  const currentScript = document.currentScript as HTMLScriptElement | null;
  const script = currentScript && currentScript.tagName === "SCRIPT"
    ? currentScript
    : document.querySelector<HTMLScriptElement>("script[data-project]");
  if (!script) return null;

  const projectId = script.dataset.project?.trim();
  if (!projectId) return null;

  return {
    projectId,
    endpoint: script.dataset.endpoint?.trim() || defaultEndpoint,
  };
}
