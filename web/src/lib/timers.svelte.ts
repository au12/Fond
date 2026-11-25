// Kitchen timers. They live outside any page so they keep running (and stay
// visible in the dock) while you browse, and they survive a reload.
import { cookState, saveCook } from './cook.svelte';

export interface Timer {
  id: string;
  label: string;
  recipeId: number;
  recipeTitle: string;
  recipePath: string;
  step: number;
  total: number;
  endsAt: number;
  pausedLeft: number | null;
  done: boolean;
}

export const timers = $state({ list: [] as Timer[], now: Date.now() });

try {
  const saved: Timer[] = JSON.parse(localStorage.getItem('fond:timers') || '[]');
  timers.list = saved.filter((t) => !t.done || Date.now() - t.endsAt < 10 * 60000);
} catch {
  /* ignore */
}

function persist() {
  try {
    localStorage.setItem('fond:timers', JSON.stringify(timers.list));
  } catch {
    /* ignore */
  }
}

export const left = (t: Timer) => (t.pausedLeft != null ? t.pausedLeft : Math.max(0, (t.endsAt - timers.now) / 1000));

let ctx: AudioContext | null = null;
let ringing: number | null = null;

function chime() {
  try {
    ctx ??= new (window.AudioContext || (window as any).webkitAudioContext)();
    const t0 = ctx.currentTime;
    [880, 1108.73, 1318.51].forEach((f, i) => {
      const o = ctx!.createOscillator();
      const g = ctx!.createGain();
      o.type = 'sine';
      o.frequency.value = f;
      g.gain.setValueAtTime(0.0001, t0 + i * 0.18);
      g.gain.exponentialRampToValueAtTime(0.35, t0 + i * 0.18 + 0.02);
      g.gain.exponentialRampToValueAtTime(0.0001, t0 + i * 0.18 + 0.7);
      o.connect(g).connect(ctx!.destination);
      o.start(t0 + i * 0.18);
      o.stop(t0 + i * 0.18 + 0.75);
    });
  } catch {
    /* no audio */
  }
}

function ring(t: Timer) {
  chime();
  navigator.vibrate?.([200, 100, 200, 100, 400]);
  let n = 0;
  if (ringing) clearInterval(ringing);
  ringing = window.setInterval(() => {
    if (++n >= 8 || !timers.list.some((x) => x.done)) return silence();
    chime();
  }, 2500);
  if ('Notification' in window && Notification.permission === 'granted' && document.visibilityState !== 'visible') {
    navigator.serviceWorker?.ready
      .then((reg) => reg.showNotification(`${t.label} — time’s up`, { body: t.recipeTitle, tag: t.id, icon: '/icons/icon-192.png' }))
      .catch(() => {});
  }
}

export function silence() {
  if (ringing) clearInterval(ringing);
  ringing = null;
}

function tick() {
  timers.now = Date.now();
  for (const t of timers.list) {
    if (!t.done && t.pausedLeft == null && t.endsAt <= timers.now) {
      t.done = true;
      // A finished timer completes its step.
      if (t.step >= 0) {
        cookState(t.recipeId).steps[t.step] = true;
        saveCook(t.recipeId);
      }
      ring(t);
      persist();
    }
  }
}
setInterval(tick, 250);
document.addEventListener('visibilitychange', tick);

export function startTimer(t: Omit<Timer, 'id' | 'endsAt' | 'pausedLeft' | 'done'>) {
  // Unlock audio on this user gesture so the chime can play later.
  try {
    ctx ??= new (window.AudioContext || (window as any).webkitAudioContext)();
    ctx.resume();
  } catch {
    /* ignore */
  }
  if ('Notification' in window && Notification.permission === 'default') Notification.requestPermission().catch(() => {});
  const timer: Timer = { ...t, id: Math.random().toString(36).slice(2), endsAt: Date.now() + t.total * 1000, pausedLeft: null, done: false };
  timers.list.push(timer);
  persist();
  return timer;
}

export function toggleTimer(id: string) {
  const t = timers.list.find((x) => x.id === id);
  if (!t || t.done) return;
  if (t.pausedLeft != null) {
    t.endsAt = Date.now() + t.pausedLeft * 1000;
    t.pausedLeft = null;
  } else t.pausedLeft = left(t);
  persist();
}

export function addMinute(id: string) {
  const t = timers.list.find((x) => x.id === id);
  if (!t) return;
  if (t.done) {
    t.done = false;
    t.endsAt = Date.now() + 60000;
    silence();
  } else if (t.pausedLeft != null) t.pausedLeft += 60;
  else t.endsAt += 60000;
  t.total += 60;
  persist();
}

export function removeTimer(id: string) {
  timers.list = timers.list.filter((x) => x.id !== id);
  if (!timers.list.some((x) => x.done)) silence();
  persist();
}

export const timerFor = (recipeId: number, step: number, label: string) => timers.list.find((t) => t.recipeId === recipeId && t.step === step && t.label === label);
