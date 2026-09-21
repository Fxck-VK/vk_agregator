import { defaultScenarios, parseScenarios, scenarioStorageKey, type LocalScenarios } from "./scenarios";

let snapshot = defaultScenarios;
const listeners = new Set<() => void>();
const notify = () => listeners.forEach(listener => listener());

export const scenarioStore = {
  getSnapshot: () => snapshot,
  getServerSnapshot: () => defaultScenarios,
  subscribe(listener: () => void) { listeners.add(listener); return () => { listeners.delete(listener); }; },
  initialize() {
    try { snapshot = parseScenarios(sessionStorage.getItem(scenarioStorageKey)); }
    catch { snapshot = { ...defaultScenarios }; }
    notify();
  },
  update(next: LocalScenarios) {
    snapshot = next;
    try { sessionStorage.setItem(scenarioStorageKey, JSON.stringify(next)); } catch { /* Memory-only preferences still work. */ }
    notify();
  },
};
