import { mount } from 'svelte';
import './app.css';
import './doc.css';
import App from './App.svelte';
import { boot } from './lib/state.svelte';

boot();
mount(App, { target: document.getElementById('app')! });

if ('serviceWorker' in navigator && location.protocol === 'https:' || location.hostname === 'localhost') {
  navigator.serviceWorker?.register('/sw.js').catch(() => {});
}
