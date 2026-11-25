// What you've ticked off while cooking. Kept per recipe in localStorage so a
// reload or a screen lock never loses your place; forgotten after a day.

export interface CookState {
  ing: Record<number, boolean>;
  steps: Record<number, boolean>;
  scale: number;
  at: number;
}

const cache = new Map<number, CookState>();
const key = (id: number) => `fond:cook:${id}`;

export function cookState(id: number): CookState {
  let s = cache.get(id);
  if (s) return s;
  let saved: CookState | null = null;
  try {
    saved = JSON.parse(localStorage.getItem(key(id)) || 'null');
  } catch {
    saved = null;
  }
  const fresh = saved && Date.now() - saved.at < 24 * 3600 * 1000;
  const state = $state<CookState>(fresh ? saved! : { ing: {}, steps: {}, scale: 1, at: Date.now() });
  cache.set(id, state);
  return state;
}

export function saveCook(id: number) {
  const s = cache.get(id);
  if (!s) return;
  s.at = Date.now();
  try {
    localStorage.setItem(key(id), JSON.stringify(s));
  } catch {
    /* storage full or blocked */
  }
}

export function resetCook(id: number) {
  const s = cookState(id);
  s.ing = {};
  s.steps = {};
  s.scale = 1;
  saveCook(id);
}
