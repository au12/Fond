// Keeps the screen on while a recipe is open.
let lock: any = null;
let wanted = false;

async function acquire() {
  if (!wanted || lock || document.visibilityState !== 'visible') return;
  try {
    lock = await (navigator as any).wakeLock?.request('screen');
    lock?.addEventListener('release', () => (lock = null));
  } catch {
    lock = null;
  }
}

document.addEventListener('visibilitychange', acquire);

export const wakeSupported = 'wakeLock' in navigator;

export function keepAwake(on: boolean) {
  wanted = on;
  if (on) acquire();
  else {
    lock?.release().catch(() => {});
    lock = null;
  }
}
