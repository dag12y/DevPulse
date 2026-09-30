const visitorCookie = "dp_visitor";
const sessionKey = "dp_session";
const sessionActivityKey = "dp_session_activity";
const visitorMaxAge = 60 * 60 * 24 * 365;
export const sessionTimeout = 30 * 60 * 1000;

const memoryVisitorIDs = new WeakMap<object, string>();
let memorySession: { id: string; activity: number } | undefined;

export function getVisitorID(document: Document): string {
  const existing = readCookieSafely(document, visitorCookie);
  if (existing) return existing;

  const id = memoryVisitorIDs.get(document) ?? createUUID();
  memoryVisitorIDs.set(document, id);
  try {
    const secure = document.location?.protocol === "https:" ? "; Secure" : "";
    document.cookie = `${visitorCookie}=${encodeURIComponent(id)}; Path=/; Max-Age=${visitorMaxAge}; SameSite=Lax${secure}`;
    return readCookieSafely(document, visitorCookie) || id;
  } catch {
    return id;
  }
}

export function getSessionID(storage: Storage | undefined, now = Date.now()): string {
  if (!storage) return getMemorySessionID(now);
  try {
    const current = storage.getItem(sessionKey);
    const lastActivity = Number(storage.getItem(sessionActivityKey));
    if (current && Number.isFinite(lastActivity) && now-lastActivity <= sessionTimeout) {
      storage.setItem(sessionActivityKey, String(now));
      return current;
    }
    const id = createUUID();
    storage.setItem(sessionKey, id);
    storage.setItem(sessionActivityKey, String(now));
    return id;
  } catch {
    return getMemorySessionID(now);
  }
}

function getMemorySessionID(now: number): string {
  if (memorySession && now-memorySession.activity <= sessionTimeout) {
    memorySession.activity = now;
    return memorySession.id;
  }
  memorySession = { id: createUUID(), activity: now };
  return memorySession.id;
}

export function createUUID(): string {
  if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID();
  const bytes = new Uint8Array(16);
  if (globalThis.crypto?.getRandomValues) {
    globalThis.crypto.getRandomValues(bytes);
  } else {
    for (let index = 0; index < bytes.length; index += 1) bytes[index] = Math.floor(Math.random() * 256);
  }
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  return Array.from(bytes, (value, index) => {
    const hex = value.toString(16).padStart(2, "0");
    return [4, 6, 8, 10].includes(index) ? `-${hex}` : hex;
  }).join("");
}

function readCookieSafely(document: Document, name: string): string | undefined {
  try {
    return readCookie(document.cookie, name);
  } catch {
    return undefined;
  }
}

function readCookie(cookies: string, name: string): string | undefined {
  const prefix = `${name}=`;
  for (const cookie of cookies.split(";")) {
    const value = cookie.trim();
    if (value.startsWith(prefix)) {
      try { return decodeURIComponent(value.slice(prefix.length)); } catch { return undefined; }
    }
  }
  return undefined;
}
