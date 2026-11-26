import { flushSync } from 'svelte';
import { get, post, put, setUnauthorizedHandler } from './api';
import { DEFAULT_PREFS, type Card, type Cookbook, type Prefs, type User } from './types';
import { ensureFoods, loadRules } from './weigh.svelte';

export const app = $state({
  ready: false,
  setup: false,
  user: null as User | null,
  features: { ai: false },
  version: '',
  origin: '',
  path: location.pathname,
  query: location.search,
  recipes: [] as Card[],
  recipesLoaded: false,
  cookbooks: [] as Cookbook[],
  adding: false,
  addUrl: '',
  addClip: '',
  online: navigator.onLine,
});

// What the library was showing, so coming back from a recipe finds the same list at the same spot.
const LIBRARY_VIEW = { q: '', fav: false, quick: false, tag: '', limit: 60 };
export const libraryView = { ...LIBRARY_VIEW };

export const toasts = $state({ list: [] as { id: number; msg: string; bad?: boolean; action?: { label: string; run: () => void } }[] });
let toastId = 0;
export function toast(msg: string, opts: { bad?: boolean; action?: { label: string; run: () => void }; ms?: number } = {}) {
  const id = ++toastId;
  toasts.list.push({ id, msg, bad: opts.bad, action: opts.action });
  setTimeout(() => dismiss(id), opts.ms ?? (opts.action ? 7000 : 3200));
}
export function dismiss(id: number) {
  toasts.list = toasts.list.filter((t) => t.id !== id);
}
export function oops(e: unknown) {
  toast(e instanceof Error ? e.message : 'Something went wrong', { bad: true, ms: 5000 });
}

export function prefs(): Prefs {
  return { ...DEFAULT_PREFS, ...(app.user?.prefs ?? {}) };
}

export async function savePrefs(patch: Partial<Prefs>) {
  if (!app.user) return;
  const next = { ...prefs(), ...patch };
  app.user.prefs = next;
  applyTheme();
  try {
    await put('/me/prefs', next);
  } catch (e) {
    oops(e);
  }
}

export function applyTheme() {
  const t = app.user ? prefs().theme : ((localStorage.getItem('fond:theme') as Prefs['theme']) ?? 'auto');
  if (app.user) localStorage.setItem('fond:theme', t);
  const dark = t === 'dark' || (t === 'auto' && matchMedia('(prefers-color-scheme: dark)').matches);
  document.documentElement.dataset.theme = dark ? 'dark' : 'light';
  document.querySelector('meta[name="theme-color"]')?.setAttribute('content', dark ? '#111714' : '#f7f8f3');
}
matchMedia('(prefers-color-scheme: dark)').addEventListener('change', applyTheme);

// ---------------------------------------------------------------- routing

const scrolls = new Map<string, number>();
let navKey = 0;
let stopHolding = () => {};

export function go(to: string, opts: { replace?: boolean } = {}) {
  const url = new URL(to, location.origin);
  if (url.pathname === app.path && url.search === app.query) return;
  stopHolding();
  scrolls.set(String(history.state?.k ?? 0), scrollY);
  const apply = () => {
    if (opts.replace) history.replaceState({ k: history.state?.k ?? 0 }, '', url);
    else history.pushState({ k: ++navKey }, '', url);
    app.path = url.pathname;
    app.query = url.search;
    scrollTo(0, 0);
  };
  const vt = (document as any).startViewTransition;
  // Leaving from an open sheet, its own exit is the animation; a cross-fade as well would show the sheet twice.
  const sheet = document.querySelector('[aria-modal="true"]');
  if (vt && !sheet && !matchMedia('(prefers-reduced-motion: reduce)').matches) vt.call(document, apply);
  else apply();
}

addEventListener('popstate', () => {
  stopHolding();
  app.path = location.pathname;
  app.query = location.search;
  // Draw the page now and scroll it before the next paint, so going back never shows the wrong spot first.
  flushSync();
  restoreScroll(scrolls.get(String(history.state?.k ?? 0)) ?? 0);
});
history.scrollRestoration = 'manual';

// The place may not exist yet: a recipe loads after it appears, and the library grows as it is scrolled.
// Keep returning to it as the page grows, until it is reached or the reader starts scrolling themselves.
const TAKEOVER = ['pointerdown', 'touchstart', 'wheel', 'keydown'] as const;
function restoreScroll(y: number) {
  scrollTo(0, y);
  if (Math.abs(scrollY - y) < 2) return;
  const ro = new ResizeObserver(() => {
    scrollTo(0, y);
    if (Math.abs(scrollY - y) < 2) stop();
  });
  const timer = setTimeout(() => stop(), 2000);
  const stop = () => {
    ro.disconnect();
    clearTimeout(timer);
    for (const t of TAKEOVER) removeEventListener(t, stop, true);
    stopHolding = () => {};
  };
  for (const t of TAKEOVER) addEventListener(t, stop, { capture: true, passive: true });
  ro.observe(document.body);
  stopHolding = stop;
}

addEventListener('click', (e) => {
  if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
  const a = (e.target as Element).closest?.('a');
  if (!a || a.target || a.hasAttribute('download') || a.origin !== location.origin) return;
  if (a.pathname.startsWith('/api/') || a.pathname.startsWith('/img/')) return;
  e.preventDefault();
  go(a.pathname + a.search);
});
addEventListener('online', () => (app.online = true));
addEventListener('offline', () => (app.online = false));

// ---------------------------------------------------------------- session & data

export const isPublicPath = (p: string) => p.startsWith('/s/') || p.startsWith('/c/');

export async function boot() {
  applyTheme();
  setUnauthorizedHandler(() => {
    app.user = null;
    if (!isPublicPath(app.path) && app.path !== '/login') go('/login?next=' + encodeURIComponent(app.path + app.query), { replace: true });
  });
  try {
    const me = await get('/me');
    setSession(me);
  } catch {
    try {
      const st = await get('/state');
      app.setup = st.setup;
      app.version = st.version;
    } catch {
      /* offline: the service worker may still serve cached data */
    }
  }
  app.ready = true;
}

export function setSession(me: any) {
  app.user = me.user;
  app.features = me.features;
  app.version = me.version;
  app.origin = me.origin;
  app.setup = false;
  applyTheme();
  loadRecipes();
  loadCookbooks();
  loadRules();
  // The ingredient dictionary is a chunk of its own; fetch it once the page has settled.
  (window.requestIdleCallback ?? ((f: () => void) => setTimeout(f, 800)))(() => ensureFoods());
}

export async function loadRecipes() {
  try {
    app.recipes = await get<Card[]>('/recipes');
    app.recipesLoaded = true;
  } catch (e) {
    if (!app.recipesLoaded) oops(e);
  }
}

export async function loadCookbooks() {
  try {
    app.cookbooks = await get<Cookbook[]>('/cookbooks');
  } catch {
    /* keep what we have */
  }
}

export async function logout() {
  await post('/auth/logout').catch(() => {});
  // Don't leave this person's recipes readable offline on a shared device.
  if ('caches' in window) caches.keys().then((ks) => ks.filter((k) => k.endsWith('-data')).forEach((k) => caches.delete(k))).catch(() => {});
  app.user = null;
  app.recipes = [];
  app.recipesLoaded = false;
  Object.assign(libraryView, LIBRARY_VIEW);
  go('/login', { replace: true });
}

export function recipePath(r: { id: number; slug: string }) {
  return `/r/${r.id}-${r.slug}`;
}

export function openAdd(url = '', clip = '') {
  app.addUrl = url;
  app.addClip = clip;
  app.adding = true;
}

export function timeAgo(unix: number): string {
  if (!unix) return '';
  const d = new Date(unix * 1000);
  const days = Math.floor((Date.now() - d.getTime()) / 86400000);
  if (days <= 0) return 'today';
  if (days === 1) return 'yesterday';
  if (days < 7) return `${days} days ago`;
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: d.getFullYear() === new Date().getFullYear() ? undefined : 'numeric' });
}
