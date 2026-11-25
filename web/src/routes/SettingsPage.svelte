<script lang="ts">
  import { api, del, get, post, put } from '../lib/api';
  import { app, go, loadCookbooks, loadRecipes, logout, oops, prefs, savePrefs, setSession, timeAgo, toast } from '../lib/state.svelte';
  import { wakeSupported } from '../lib/wake';
  import { Bot, Copy, Download, KeyRound, LogOut, Settings, Smartphone, Trash, Upload, Users } from '../lib/icons';
  import type { User } from '../lib/types';

  let { tab }: { tab: string } = $props();
  const admin = $derived(app.user?.role === 'admin');
  const pf = $derived(prefs());
  const origin = $derived(app.origin || location.origin);
  const current = $derived(tab || 'prefs');
  const tabs = $derived([
    ['prefs', 'Preferences', Settings], ['share', 'Save from your phone', Smartphone],
    ...(admin ? ([['import', 'Import', Upload], ['people', 'Household', Users], ['ai', 'AI assist', Bot]] as const) : []),
    ['account', 'Account', KeyRound],
  ] as const);

  // ---- account
  let name = $state(app.user?.name ?? '');
  let curPw = $state('');
  let newPw = $state('');
  let sessions = $state<any[]>([]);
  let tokens = $state<any[]>([]);
  let freshToken = $state('');
  // ---- people
  let users = $state<User[]>([]);
  let nu = $state({ username: '', name: '', password: '', role: 'member' });
  // ---- import
  let mealieUrl = $state('');
  let mealieToken = $state('');
  let localFiles = $state<{ name: string; size: number }[]>([]);
  let job = $state<any>(null);
  let uploading = $state(0);
  // ---- ai
  let ai = $state({ baseUrl: '', model: '', apiKey: '', hasKey: false });

  $effect(() => {
    if (current === 'account' || current === 'share') {
      get('/me/sessions').then((x) => (sessions = x)).catch(() => {});
      get('/me/tokens').then((x) => (tokens = x)).catch(() => {});
    }
    if (current === 'people' && admin) get('/users').then((x) => (users = x)).catch(oops);
    if (current === 'import' && admin) get('/import/local').then((x) => (localFiles = x)).catch(() => {});
    if (current === 'ai' && admin) get('/settings/ai').then((x) => (ai = { ...x, apiKey: '' })).catch(() => {});
  });

  async function saveName(e: SubmitEvent) {
    e.preventDefault();
    try {
      setSession(await put('/me', { name }));
      toast('Name updated');
    } catch (err) {
      oops(err);
    }
  }
  async function changePw(e: SubmitEvent) {
    e.preventDefault();
    try {
      await put('/me/password', { current: curPw, password: newPw });
      curPw = newPw = '';
      toast('Password changed. Other devices were signed out.');
    } catch (err) {
      oops(err);
    }
  }
  async function makeToken(label: string) {
    try {
      const t = await post('/me/tokens', { name: label });
      freshToken = t.token;
      tokens = await get('/me/tokens');
    } catch (err) {
      oops(err);
    }
  }
  async function copy(text: string, what = 'Copied') {
    await navigator.clipboard.writeText(text);
    toast(what);
  }
  async function addUser(e: SubmitEvent) {
    e.preventDefault();
    try {
      await post('/users', nu);
      nu = { username: '', name: '', password: '', role: 'member' };
      users = await get('/users');
      toast('Account created');
    } catch (err) {
      oops(err);
    }
  }
  async function removeUser(u: User) {
    if (!confirm(`Remove ${u.name}’s account? Their recipes stay in the library.`)) return;
    await del(`/users/${u.id}`).catch(oops);
    users = await get('/users');
  }
  async function resetPw(u: User) {
    const pw = prompt(`New password for ${u.name} (at least 8 characters)`);
    if (!pw) return;
    try {
      await put(`/users/${u.id}`, { password: pw });
      toast('Password reset');
    } catch (err) {
      oops(err);
    }
  }

  function watch(id: string) {
    job = { state: 'running', done: 0, total: 0, current: '', problems: [] };
    const poll = async () => {
      try {
        job = await get(`/jobs/${id}`);
      } catch {
        return;
      }
      if (job.state === 'running') setTimeout(poll, 500);
      else {
        loadRecipes();
        loadCookbooks();
      }
    };
    poll();
  }
  function uploadZip(e: Event) {
    const f = (e.target as HTMLInputElement).files?.[0];
    if (!f) return;
    const fd = new FormData();
    fd.append('file', f);
    const xhr = new XMLHttpRequest();
    xhr.open('POST', '/api/import/mealie');
    xhr.upload.onprogress = (ev) => (uploading = ev.lengthComputable ? Math.max(0.01, ev.loaded / ev.total) : 0.01);
    xhr.onload = () => {
      uploading = 0;
      try {
        const res = JSON.parse(xhr.responseText);
        if (xhr.status < 300) watch(res.job);
        else toast(res.error ?? 'Upload failed', { bad: true });
      } catch {
        toast(xhr.status === 413 ? 'Your reverse proxy rejected the upload as too large. Put the zip in Fond’s import folder instead.' : 'Upload failed', { bad: true, ms: 8000 });
      }
    };
    xhr.onerror = () => { uploading = 0; toast('Upload failed', { bad: true }); };
    uploading = 0.01;
    xhr.send(fd);
  }
  async function fromApi(e: SubmitEvent) {
    e.preventDefault();
    try {
      watch((await post('/import/mealie-api', { url: mealieUrl, token: mealieToken })).job);
    } catch (err) {
      oops(err);
    }
  }
  async function fromLocal(file: string) {
    try {
      watch((await post('/import/local', { file })).job);
    } catch (err) {
      oops(err);
    }
  }
  async function saveAI(e: SubmitEvent) {
    e.preventDefault();
    try {
      const res = await put('/settings/ai', { baseUrl: ai.baseUrl, model: ai.model, apiKey: ai.apiKey, keepKey: !ai.apiKey && ai.hasKey });
      ai = { ...res, apiKey: '' };
      setSession(await get('/me'));
      toast(res.baseUrl ? 'AI assist is on' : 'AI assist is off');
    } catch (err) {
      oops(err);
    }
  }
  const shareToken = $derived(freshToken || '<your token>');
  const bookmarklet = $derived(
    `javascript:(()=>{const f=document.createElement('form');f.method='POST';f.action='${origin}/share';f.target='_blank';for(const[k,v]of[['url',location.href],['html',document.documentElement.outerHTML.slice(0,3500000)]]){const i=document.createElement('input');i.type='hidden';i.name=k;i.value=v;f.appendChild(i)}document.body.appendChild(f);f.submit();f.remove()})()`,
  );
  const device = (ua: string) => (/iPhone|iPad/.test(ua) ? 'iPhone or iPad' : /Android/.test(ua) ? 'Android' : /Mac/.test(ua) ? 'Mac' : /Windows/.test(ua) ? 'Windows PC' : 'Browser');
</script>

<div class="page settings">
  <h1 class="page-title">Settings</h1>
  <div class="layout">
    <nav aria-label="Settings sections">
      {#each tabs as [key, label, Icon]}
        <a href={`/settings/${key}`} class:on={current === key}><Icon />{label}</a>
      {/each}
      <button onclick={logout}><LogOut />Sign out</button>
    </nav>

    <div class="pane">
      {#if current === 'prefs'}
        <section>
          <h2>How recipes are shown</h2>
          <p class="muted">These are your defaults. You can still switch units on any recipe while you cook.</p>
          <div class="opt">
            <div><b>Measurements</b><span>Convert every recipe, or leave them as the author wrote them.</span></div>
            <div class="seg">
              {#each [['original', 'As written'], ['metric', 'Metric'], ['us', 'US']] as [v, l]}<button class:on={pf.system === v} onclick={() => savePrefs({ system: v as any })}>{l}</button>{/each}
            </div>
          </div>
          <div class="opt">
            <div><b>Weigh instead of scooping</b><span>Show flour, sugar and other staples by weight when a recipe gives cups. Uses standard densities, so treat the numbers as close estimates.</span></div>
            <button class="switch" role="switch" aria-checked={pf.weigh} aria-label="Prefer weights" onclick={() => savePrefs({ weigh: !pf.weigh })}></button>
          </div>
          <div class="opt">
            <div><b>Oven temperatures</b><span>Convert temperatures written inside the steps.</span></div>
            <div class="seg">
              {#each [['original', 'Follow units'], ['C', '°C'], ['F', '°F']] as [v, l]}<button class:on={pf.temp === v} onclick={() => savePrefs({ temp: v as any })}>{l}</button>{/each}
            </div>
          </div>
          <div class="opt">
            <div><b>Fractions</b><span>Write 1½ cups instead of 1.5 cups.</span></div>
            <button class="switch" role="switch" aria-checked={pf.fractions} aria-label="Use fractions" onclick={() => savePrefs({ fractions: !pf.fractions })}></button>
          </div>
          <div class="opt">
            <div><b>Baker’s percentages</b><span>Remember when you turn them on for a bread or pastry recipe.</span></div>
            <button class="switch" role="switch" aria-checked={pf.baker} aria-label="Baker's percentages" onclick={() => savePrefs({ baker: !pf.baker })}></button>
          </div>
        </section>
        <section>
          <h2>While you cook</h2>
          <div class="opt">
            <div><b>Keep the screen awake</b><span>{wakeSupported ? 'The screen stays on while a recipe is open.' : 'This browser doesn’t support keeping the screen awake.'}</span></div>
            <button class="switch" role="switch" aria-checked={pf.awake} aria-label="Keep screen awake" onclick={() => savePrefs({ awake: !pf.awake })}></button>
          </div>
          <div class="opt">
            <div><b>Appearance</b></div>
            <div class="seg">
              {#each [['auto', 'Match device'], ['light', 'Light'], ['dark', 'Dark']] as [v, l]}<button class:on={pf.theme === v} onclick={() => savePrefs({ theme: v as any })}>{l}</button>{/each}
            </div>
          </div>
        </section>
      {:else if current === 'share'}
        <section>
          <h2>Android</h2>
          <ol class="steps">
            <li>Open Fond in Chrome, tap the <b>⋮</b> menu and choose <b>Add to Home screen</b>, then <b>Install</b>.</li>
            <li>On any recipe page, tap <b>Share</b> and pick <b>Fond</b>. The recipe is saved and opens straight away.</li>
          </ol>
        </section>
        <section>
          <h2>iPhone and iPad</h2>
          <p class="muted">Safari doesn’t let web apps appear in the share sheet, so Fond uses a two-step Shortcut instead. Set it up once and “Save to Fond” shows up when you tap Share in Safari.</p>
          <ol class="steps">
            <li>
              Create an access token for the shortcut.
              <div class="row wrap" style:margin-top="8px">
                <button class="btn sm" onclick={() => makeToken('iPhone shortcut')}><KeyRound />Create token</button>
                {#if freshToken}<code class="code grow">{freshToken}</code><button class="btn sm quiet" onclick={() => copy(freshToken, 'Token copied')}><Copy />Copy</button>{/if}
              </div>
              {#if freshToken}<p class="hint" style:margin-top="6px">Copy it now. Fond only shows a token once.</p>{/if}
            </li>
            <li>Open the <b>Shortcuts</b> app, tap <b>+</b>, and name the shortcut <b>Save to Fond</b>.</li>
            <li>Tap the <b>ⓘ</b> button, turn on <b>Show in Share Sheet</b>, and set it to receive <b>URLs</b> and <b>Safari web pages</b>.</li>
            <li>
              Add the action <b>Get Contents of URL</b> and fill it in:
              <dl class="kv">
                <dt>URL</dt><dd><code class="code">{origin}/api/import/url</code><button class="btn sm quiet" onclick={() => copy(`${origin}/api/import/url`)}><Copy /></button></dd>
                <dt>Method</dt><dd><code class="code">POST</code></dd>
                <dt>Headers</dt><dd><code class="code">Authorization</code> = <code class="code">Bearer {shareToken}</code><button class="btn sm quiet" onclick={() => copy(`Bearer ${shareToken}`)}><Copy /></button></dd>
                <dt>Request Body</dt><dd>JSON, with one text field: <code class="code">url</code> = <b>Shortcut Input</b></dd>
              </dl>
            </li>
            <li>Optional: add <b>Show Notification</b> with the text “Saved to Fond”.</li>
            <li>Optional, for sites that turn away automated requests: put a second <b>Get Contents of URL</b> action first, with its URL set to <b>Shortcut Input</b>, and add a second JSON text field <code class="code">html</code> = <b>Contents of URL</b>. Your phone then fetches the page itself and hands it to Fond.</li>
          </ol>
          <p class="hint">Made it once? Share the shortcut from the Shortcuts app to anyone else in the house, then swap in their own token.</p>
        </section>
        <section>
          <h2>Any desktop browser</h2>
          <p class="muted">Drag this button to your bookmarks bar. Click it on a recipe page to save it.</p>
          <!-- svelte-ignore a11y_invalid_attribute -->
          <a class="btn" href={bookmarklet} onclick={(e) => e.preventDefault()}>Save to Fond</a>
          <p class="hint" style:margin-top="10px">The bookmark sends Fond the page your browser has already loaded, so it also works on sites that turn away automated requests and on pages behind a login. On the Recipes page you can also just paste a link with Ctrl/⌘ V.</p>
        </section>
      {:else if current === 'import' && admin}
        <section>
          <h2>Move from Mealie</h2>
          <p class="muted">Recipes, photos, tags, categories, favorites, ratings and cookbooks come across. Structured Mealie ingredients are turned back into plain lines. Running an import twice is safe: recipes already imported are skipped.</p>
          {#if job}
            <div class="job">
              {#if job.state === 'running'}
                <b>Importing{job.total ? ` ${job.done} of ${job.total}` : ''}</b>
                <div class="meter"><i style:width={`${job.total ? (100 * job.done) / job.total : 4}%`}></i></div>
                <span class="hint">{job.current}</span>
              {:else if job.state === 'done'}
                <b>Import finished</b>
                <span>{job.imported} recipes added{job.skipped ? `, ${job.skipped} already here` : ''}{job.cookbooks ? `, ${job.cookbooks} cookbooks` : ''}{job.failed ? `, ${job.failed} couldn’t be read` : ''}.</span>
                {#each job.problems as p}<span class="hint">{p}</span>{/each}
                <div><button class="btn primary sm" onclick={() => go('/')}>See your recipes</button></div>
              {:else}
                <b class="error-text">Import stopped</b><span>{job.error}</span>
              {/if}
            </div>
          {/if}
          <h3>From a backup or export file</h3>
          <p class="hint">In Mealie: <b>Settings → Backups → Create</b>, then download the zip. The recipe export from <b>Data Management</b> works too.</p>
          <label class="btn" class:primary={!job} style:width="fit-content">
            <Upload />{uploading ? `Uploading ${Math.round(uploading * 100)}%` : 'Choose a Mealie zip'}
            <input type="file" accept=".zip,application/zip" hidden onchange={uploadZip} disabled={!!uploading} />
          </label>
          {#if localFiles.length}
            <p class="hint">Or use a file from Fond’s <code class="code">import</code> folder:</p>
            {#each localFiles as f}<button class="btn sm" style:width="fit-content" onclick={() => fromLocal(f.name)}>{f.name} <span class="faint">{(f.size / 1048576).toFixed(1)} MB</span></button>{/each}
          {:else}
            <p class="hint">Very large backup? Drop the zip into the <code class="code">import</code> folder inside Fond’s data folder and reload this page.</p>
          {/if}
          <h3>Straight from a running Mealie</h3>
          <form class="stack" onsubmit={fromApi}>
            <label class="field"><span>Mealie address</span><input class="input" bind:value={mealieUrl} placeholder="http://10.0.10.20:9925" required /></label>
            <label class="field"><span>API token</span><input class="input" bind:value={mealieToken} type="password" autocomplete="off" required /><span class="hint">In Mealie: your profile → <b>Manage Your API Tokens</b>.</span></label>
            <button class="btn" style:width="fit-content">Import from Mealie</button>
          </form>
        </section>
        <section>
          <h2>Take your recipes with you</h2>
          <p class="muted">Download every recipe, cookbook and photo as one zip. For a full backup, copy Fond’s data folder.</p>
          <a class="btn" href="/api/export" download style:width="fit-content"><Download />Download everything</a>
        </section>
      {:else if current === 'people' && admin}
        <section>
          <h2>Household</h2>
          <p class="muted">Everyone shares one recipe library. Favorites and preferences are personal.</p>
          <ul class="list">
            {#each users as u (u.id)}
              <li>
                <div class="grow"><b>{u.name}</b><span class="hint">{u.username}{u.role === 'admin' ? ', admin' : ''}</span></div>
                <button class="btn sm quiet" onclick={() => resetPw(u)}>Reset password</button>
                {#if u.id !== app.user?.id}<button class="icon-btn" onclick={() => removeUser(u)} aria-label={`Remove ${u.name}`}><Trash /></button>{/if}
              </li>
            {/each}
          </ul>
          <h3>Add someone</h3>
          <form class="stack" onsubmit={addUser}>
            <div class="two">
              <label class="field"><span>Name</span><input class="input" bind:value={nu.name} required /></label>
              <label class="field"><span>Username</span><input class="input" bind:value={nu.username} autocapitalize="none" required /></label>
            </div>
            <div class="two">
              <label class="field"><span>Password</span><input class="input" type="text" bind:value={nu.password} minlength="8" required autocomplete="off" /></label>
              <label class="field"><span>Role</span><select class="input" bind:value={nu.role}><option value="member">Member</option><option value="admin">Admin</option></select></label>
            </div>
            <button class="btn primary" style:width="fit-content">Create account</button>
          </form>
        </section>
      {:else if current === 'ai' && admin}
        <section>
          <h2>AI assist</h2>
          <p class="muted">Optional. Fond reads most recipe pages on its own. With a model connected it can also rescue pages that have no recipe markup, tidy pasted text, and read recipes from photos (with a vision model). Works with any OpenAI-compatible API, including Ollama on your own server.</p>
          <form class="stack" onsubmit={saveAI}>
            <label class="field"><span>API address</span><input class="input" bind:value={ai.baseUrl} placeholder="http://10.0.10.20:11434/v1" /><span class="hint">Leave empty to turn AI assist off.</span></label>
            <label class="field"><span>Model</span><input class="input" bind:value={ai.model} placeholder="gemma3:12b" /></label>
            <label class="field"><span>API key</span><input class="input" type="password" bind:value={ai.apiKey} placeholder={ai.hasKey ? 'Saved. Type to replace it.' : 'Not needed for Ollama'} autocomplete="off" /></label>
            <button class="btn primary" style:width="fit-content">Save</button>
          </form>
        </section>
      {:else}
        <section>
          <h2>Your account</h2>
          <form class="row" onsubmit={saveName}>
            <label class="field grow"><span>Name</span><input class="input" bind:value={name} required /></label>
            <button class="btn" style:align-self="end">Save</button>
          </form>
          <h3>Change password</h3>
          <form class="stack" onsubmit={changePw}>
            <div class="two">
              <label class="field"><span>Current password</span><input class="input" type="password" bind:value={curPw} autocomplete="current-password" required /></label>
              <label class="field"><span>New password</span><input class="input" type="password" bind:value={newPw} autocomplete="new-password" minlength="8" required /></label>
            </div>
            <button class="btn" style:width="fit-content">Change password</button>
          </form>
        </section>
        <section>
          <h2>Signed-in devices</h2>
          <ul class="list">
            {#each sessions as s (s.id)}
              <li>
                <div class="grow"><b>{device(s.userAgent)}{s.current ? ' (this device)' : ''}</b><span class="hint">Last used {timeAgo(s.lastSeen)} from {s.ip}</span></div>
                {#if !s.current}<button class="btn sm quiet" onclick={async () => { await del(`/me/sessions/${s.id}`); sessions = await get('/me/sessions'); }}>Sign out</button>{/if}
              </li>
            {/each}
          </ul>
        </section>
        <section>
          <h2>Access tokens</h2>
          <p class="muted">Tokens let shortcuts and scripts save recipes as you. Remove one to cut off whatever uses it.</p>
          <ul class="list">
            {#each tokens as t (t.id)}
              <li>
                <div class="grow"><b>{t.name}</b><span class="hint">{t.lastUsed ? `Last used ${timeAgo(t.lastUsed)}` : 'Never used'}</span></div>
                <button class="icon-btn" onclick={async () => { await del(`/me/tokens/${t.id}`); tokens = await get('/me/tokens'); }} aria-label="Remove token"><Trash /></button>
              </li>
            {/each}
          </ul>
          <div class="row wrap">
            <button class="btn sm" onclick={() => makeToken('Token')}><KeyRound />Create token</button>
            {#if freshToken}<code class="code">{freshToken}</code><button class="btn sm quiet" onclick={() => copy(freshToken, 'Token copied')}><Copy />Copy</button>{/if}
          </div>
        </section>
        <p class="faint small">Fond {app.version}</p>
      {/if}
    </div>
  </div>
</div>

<style>
  .settings { width: min(1040px, 100%); }
  .layout { display: grid; grid-template-columns: 230px minmax(0, 1fr); gap: 40px; margin-top: 24px; align-items: start; }
  nav { position: sticky; top: 84px; display: grid; gap: 2px; }
  nav a, nav button { display: flex; align-items: center; gap: 10px; padding: 10px 12px; border-radius: 10px; font-weight: 600; font-size: 0.95rem; color: var(--ink-2); text-align: left; }
  nav a:hover, nav button:hover { background: var(--sunk); color: var(--ink); }
  nav a.on { background: var(--wash); color: var(--bay); }
  nav :global(svg) { width: 18px; height: 18px; }
  .pane { display: grid; gap: 34px; min-width: 0; }
  section { display: grid; gap: 14px; }
  h2 { font: 600 1.5rem/1.2 var(--serif); }
  h3 { font: 600 1.1rem/1.2 var(--serif); margin-top: 12px; }
  .opt { display: flex; align-items: center; justify-content: space-between; gap: 18px; padding: 14px 0; border-bottom: 1px solid var(--line); }
  .opt > div:first-child { display: grid; gap: 2px; }
  .opt span { font-size: 0.875rem; color: var(--ink-2); max-width: 52ch; }
  .two { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
  .list li { display: flex; align-items: center; gap: 10px; padding: 12px 0; border-bottom: 1px solid var(--line); }
  .list li > div { display: grid; }
  .steps { list-style: decimal; padding-left: 22px; display: grid; gap: 14px; line-height: 1.55; }
  .steps li::marker { font: 600 1.1rem var(--serif); color: var(--bay); }
  .code { display: inline-block; padding: 3px 8px; border-radius: 6px; background: var(--sunk); font: 500 0.82rem ui-monospace, 'SF Mono', Menlo, Consolas, monospace; word-break: break-all; }
  .kv { display: grid; grid-template-columns: auto 1fr; gap: 8px 14px; margin-top: 10px; align-items: center; font-size: 0.92rem; }
  .kv dt { font-weight: 600; color: var(--ink-2); }
  .kv dd { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; }
  .job { display: grid; gap: 8px; padding: 16px 18px; border-radius: var(--r-l); background: var(--butter-soft); }
  .meter { height: 8px; border-radius: 999px; background: color-mix(in srgb, var(--ink) 12%, transparent); overflow: hidden; }
  .meter i { display: block; height: 100%; border-radius: 999px; background: var(--bay); transition: width 0.4s var(--ease); }
  @media (max-width: 820px) {
    .layout { grid-template-columns: 1fr; gap: 20px; }
    nav { position: static; display: flex; overflow-x: auto; scrollbar-width: none; margin: 0 -16px; padding: 0 16px; }
    nav a, nav button { white-space: nowrap; }
    .two { grid-template-columns: 1fr; }
    .opt { flex-wrap: wrap; }
  }
</style>
