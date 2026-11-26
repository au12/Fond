// Fond service worker: makes the app installable and keeps recipes you've
// opened readable when the kitchen Wi-Fi drops out.
const VERSION = 'fond-v2';
const SHELL = VERSION + '-shell';
const DATA = VERSION + '-data';
const IMAGES = VERSION + '-img';

self.addEventListener('install', (e) => {
  e.waitUntil(caches.open(SHELL).then((c) => c.add(new Request('/', { cache: 'reload' }))).then(() => self.skipWaiting()));
});

self.addEventListener('activate', (e) => {
  e.waitUntil(
    caches.keys().then((keys) => Promise.all(keys.filter((k) => !k.startsWith(VERSION)).map((k) => caches.delete(k)))).then(() => self.clients.claim()),
  );
});

async function trim(name, max) {
  const cache = await caches.open(name);
  const keys = await cache.keys();
  for (const k of keys.slice(0, Math.max(0, keys.length - max))) await cache.delete(k);
}

async function networkFirst(req, cacheName, fallbackUrl) {
  const cache = await caches.open(cacheName);
  try {
    const res = await fetch(req);
    if (res.ok) cache.put(fallbackUrl || req, res.clone());
    return res;
  } catch (err) {
    const hit = await cache.match(fallbackUrl || req);
    if (hit) return hit;
    throw err;
  }
}

async function cacheFirst(req, cacheName, max) {
  const cache = await caches.open(cacheName);
  const hit = await cache.match(req);
  if (hit) return hit;
  const res = await fetch(req);
  if (res.ok) {
    cache.put(req, res.clone());
    if (max) trim(cacheName, max);
  }
  return res;
}

self.addEventListener('fetch', (e) => {
  const req = e.request;
  if (req.method !== 'GET') return;
  const url = new URL(req.url);
  if (url.origin !== location.origin) return;
  if (req.mode === 'navigate') {
    // Shared pages carry their own preview tags, so always try the network.
    e.respondWith(networkFirst(req, SHELL, url.pathname.startsWith('/s/') || url.pathname.startsWith('/c/') ? null : '/'));
  } else if (url.pathname.startsWith('/assets/') || url.pathname.startsWith('/icons/')) {
    e.respondWith(cacheFirst(req, SHELL));
  } else if (url.pathname.startsWith('/img/') || url.pathname.startsWith('/leather/')) {
    e.respondWith(cacheFirst(req, IMAGES, 600));
  } else if (/^\/api\/(recipes|cookbooks|me$|shopping|public|food-rules$)/.test(url.pathname) && !url.pathname.startsWith('/api/export')) {
    e.respondWith(networkFirst(req, DATA));
  }
});

self.addEventListener('notificationclick', (e) => {
  e.notification.close();
  e.waitUntil(self.clients.matchAll({ type: 'window' }).then((cs) => (cs[0] ? cs[0].focus() : self.clients.openWindow('/'))));
});
