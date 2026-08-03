// Local query history (M1.6: "historique de requêtes (local + serveur)").
// The "serveur" half would read engine/querybroker's audit log (M1.4), but
// that's written to a local JSON-lines file with no HTTP route exposing
// it yet — see tasks-herminas.md M1.6 for the honest status. This covers
// the local half for real: every run is remembered across page reloads.

const HISTORY_KEY = "herminas-query-history";
const MAX_HISTORY = 20;

export function loadHistory(): string[] {
  try {
    const raw = localStorage.getItem(HISTORY_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed.filter((v): v is string => typeof v === "string") : [];
  } catch {
    return [];
  }
}

export function pushHistory(history: string[], sql: string): string[] {
  const next = [sql, ...history.filter((q) => q !== sql)].slice(0, MAX_HISTORY);
  try {
    localStorage.setItem(HISTORY_KEY, JSON.stringify(next));
  } catch {
    // Storage full or unavailable (private browsing): history just won't
    // persist across reloads this session, not worth surfacing an error
    // for a convenience feature.
  }
  return next;
}
