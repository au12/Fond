<script lang="ts">
  import { fly } from 'svelte/transition';
  import { flip } from 'svelte/animate';
  import Logo from './components/Logo.svelte';
  import AddSheet from './components/AddSheet.svelte';
  import TimerDock from './components/TimerDock.svelte';
  import Login from './routes/Login.svelte';
  import Library from './routes/Library.svelte';
  import RecipePage from './routes/RecipePage.svelte';
  import EditPage from './routes/EditPage.svelte';
  import Cookbooks from './routes/Cookbooks.svelte';
  import CookbookPage from './routes/CookbookPage.svelte';
  import Shopping from './routes/Shopping.svelte';
  import SettingsPage from './routes/SettingsPage.svelte';
  import PrintPage from './routes/PrintPage.svelte';
  import PublicPage from './routes/PublicPage.svelte';
  import { app, dismiss, isPublicPath, openAdd, toasts, go } from './lib/state.svelte';
  import { BookOpen, Plus, Settings, ShoppingBasket, Utensils } from './lib/icons';

  let scrolled = $state(false);
  const seg = $derived(app.path.split('/').filter(Boolean));
  const idOf = (s: string | undefined) => parseInt(s ?? '', 10) || 0;
  const section = $derived(seg[0] === 'books' ? 'books' : seg[0] === 'list' ? 'list' : seg[0] === 'settings' ? 'settings' : 'recipes');
  const printing = $derived(seg[2] === 'print');

  // Android "Share to Fond" lands on /share?url=…; some apps put the link in text.
  $effect(() => {
    if (app.user && app.path === '/share') {
      const q = new URLSearchParams(app.query);
      const raw = [q.get('url'), q.get('text'), q.get('title')].filter(Boolean).join(' ');
      const link = raw.match(/https?:\/\/\S+/)?.[0] ?? '';
      go('/', { replace: true });
      openAdd(link || raw, q.get('clip') ?? '');
    }
  });
</script>

<svelte:window onscroll={() => (scrolled = scrollY > 4)} />

{#if !app.ready}
  <div style="display:grid;place-items:center;height:100dvh;color:var(--bay)"><Logo size={44} /></div>
{:else if isPublicPath(app.path)}
  <PublicPage kind={seg[0] === 's' ? 'recipe' : 'cookbook'} token={seg[1] ?? ''} />
{:else if !app.user}
  <Login />
{:else if printing}
  <PrintPage kind={seg[0] === 'books' ? 'cookbook' : 'recipe'} id={idOf(seg[1])} />
{:else}
  <div class="shell">
    <header class="topbar" class:scrolled>
      <a href="/" class="wordmark" aria-label="Fond, all recipes"><Logo />Fond</a>
      <nav class="topnav" aria-label="Main">
        <a href="/" class:on={section === 'recipes'}>Recipes</a>
        <a href="/books" class:on={section === 'books'}>Cookbooks</a>
        <a href="/list" class:on={section === 'list'}>Shopping list</a>
      </nav>
      <div class="grow"></div>
      {#if !app.online}<span class="chip" style="color:var(--tomato)">Offline</span>{/if}
      <button class="btn primary desk" onclick={() => openAdd()}><Plus />Add recipe</button>
      <a href="/settings" class="icon-btn desk" class:on={section === 'settings'} aria-label="Settings"><Settings /></a>
    </header>

    <main>
      {#if seg[0] === 'r' && seg[2] === 'edit'}
        <EditPage id={idOf(seg[1])} />
      {:else if seg[0] === 'r'}
        {#key idOf(seg[1])}<RecipePage id={idOf(seg[1])} />{/key}
      {:else if seg[0] === 'new'}
        <EditPage id={0} />
      {:else if seg[0] === 'books' && seg[1]}
        {#key idOf(seg[1])}<CookbookPage id={idOf(seg[1])} />{/key}
      {:else if seg[0] === 'books'}
        <Cookbooks />
      {:else if seg[0] === 'list'}
        <Shopping />
      {:else if seg[0] === 'settings'}
        <SettingsPage tab={seg[1] ?? ''} />
      {:else}
        <Library />
      {/if}
    </main>

    <nav class="tabbar" aria-label="Main">
      <a href="/" class:on={section === 'recipes'}><Utensils />Recipes</a>
      <a href="/books" class:on={section === 'books'}><BookOpen />Cookbooks</a>
      <button class="add" onclick={() => openAdd()} aria-label="Add recipe"><Plus /></button>
      <a href="/list" class:on={section === 'list'}><ShoppingBasket />List</a>
      <a href="/settings" class:on={section === 'settings'}><Settings />Settings</a>
    </nav>
    <TimerDock />
    <AddSheet />
  </div>
{/if}

<div class="toasts" aria-live="polite">
  {#each toasts.list as t (t.id)}
    <div class="toast" class:bad={t.bad} animate:flip={{ duration: 200 }} in:fly={{ y: 16, duration: 220 }} out:fly={{ y: 8, duration: 160 }}>
      <span>{t.msg}</span>
      {#if t.action}<button onclick={() => { t.action!.run(); dismiss(t.id); }}>{t.action.label}</button>{/if}
    </div>
  {/each}
</div>
