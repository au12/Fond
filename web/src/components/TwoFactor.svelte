<script lang="ts">
  // Two-step sign-in: an authenticator app's six-digit code after the password.
  import QR from './QR.svelte';
  import Sheet from './Sheet.svelte';
  import { get, post } from '../lib/api';
  import { app, oops, toast } from '../lib/state.svelte';
  import { Check, Copy, Download, Loader, ShieldCheck } from '../lib/icons';

  let status = $state<{ on: boolean; recoveryLeft: number } | null>(null);
  const load = () => get('/me/2fa').then((s) => (status = s)).catch(() => {});
  load();

  // Turning it on: scan, confirm a code, then keep the recovery codes.
  let enrolling = $state(false);
  let stage = $state<'scan' | 'codes'>('scan');
  let secret = $state('');
  let uri = $state('');
  let code = $state('');
  let busy = $state(false);
  let error = $state('');
  let codes = $state<string[]>([]);
  let kept = $state(false);
  // Turning it off, or dealing new recovery codes, asks for the password again.
  let asking = $state<'' | 'off' | 'codes'>('');
  let password = $state('');

  async function start() {
    error = '';
    code = '';
    kept = false;
    try {
      const s = await post('/me/2fa/setup');
      secret = s.secret;
      uri = s.uri;
      stage = 'scan';
      enrolling = true;
    } catch (e) {
      oops(e);
    }
  }
  async function confirm(e: SubmitEvent) {
    e.preventDefault();
    busy = true;
    error = '';
    try {
      const res = await post('/me/2fa/enable', { secret, code });
      codes = res.recovery;
      stage = 'codes';
      load();
      if (app.user) app.user.twoFactor = true;
    } catch (err: any) {
      error = err.message;
    } finally {
      busy = false;
    }
  }
  async function withPassword(e: SubmitEvent) {
    e.preventDefault();
    busy = true;
    error = '';
    try {
      if (asking === 'off') {
        await post('/me/2fa/disable', { password });
        toast('Two-step sign-in is off');
        if (app.user) app.user.twoFactor = false;
        asking = '';
      } else {
        const res = await post('/me/2fa/recovery', { password });
        codes = res.recovery;
        kept = false;
        stage = 'codes';
        asking = '';
        enrolling = true;
      }
      password = '';
      load();
    } catch (err: any) {
      error = err.message;
    } finally {
      busy = false;
    }
  }
  const grouped = $derived(secret.replace(/(.{4})/g, '$1 ').trim());
  async function copy(text: string, what: string) {
    await navigator.clipboard.writeText(text);
    toast(what);
  }
  function download() {
    const text = `Fond recovery codes for ${app.user?.username}\nEach code signs in once, in place of an authenticator code.\n\n${codes.join('\n')}\n`;
    const a = document.createElement('a');
    a.href = URL.createObjectURL(new Blob([text], { type: 'text/plain' }));
    a.download = 'fond-recovery-codes.txt';
    a.click();
    URL.revokeObjectURL(a.href);
    kept = true;
  }
</script>

<section>
  <h2>Two-step sign-in</h2>
  {#if !status}
    <p class="muted">&nbsp;</p>
  {:else if status.on}
    <div class="state on">
      <ShieldCheck />
      <div class="grow"><b>On</b><span class="hint">Signing in asks for a code from your authenticator app. {status.recoveryLeft} recovery code{status.recoveryLeft === 1 ? '' : 's'} left.</span></div>
    </div>
    <div class="row wrap">
      <button class="btn sm" onclick={() => { asking = 'codes'; error = ''; }}>New recovery codes</button>
      <button class="btn sm quiet danger" onclick={() => { asking = 'off'; error = ''; }}>Turn off</button>
    </div>
  {:else}
    <p class="muted">After your password, Fond also asks for a six-digit code from an authenticator app on your phone, such as 1Password, Google Authenticator, Authy or the iPhone’s Passwords app. Worth turning on if Fond is reachable from the internet.</p>
    <button class="btn" style:width="fit-content" onclick={start}><ShieldCheck />Turn on two-step sign-in</button>
  {/if}
</section>

<Sheet bind:open={enrolling} title={stage === 'scan' ? 'Add Fond to your authenticator' : 'Save your recovery codes'}>
  {#if stage === 'scan'}
    <form class="stack" onsubmit={confirm}>
      <ol class="how">
        <li>
          <p>Scan this with your authenticator app.</p>
          <div class="qr"><QR value={uri} size={176} /></div>
          <p class="hint">Can’t scan? Enter this key instead:</p>
          <div class="row"><code class="key">{grouped}</code><button type="button" class="icon-btn" onclick={() => copy(secret, 'Key copied')} aria-label="Copy key"><Copy /></button></div>
        </li>
        <li>
          <p>Enter the six-digit code it shows.</p>
          <!-- svelte-ignore a11y_autofocus -->
          <input class="input code" bind:value={code} inputmode="numeric" autocomplete="one-time-code" maxlength="7" placeholder="123 456" required aria-label="Code" />
        </li>
      </ol>
      {#if error}<p class="error-text" role="alert">{error}</p>{/if}
      <button class="btn primary" disabled={busy || code.replace(/\D/g, '').length !== 6}>{#if busy}<span class="spin" style="display:grid"><Loader /></span>{/if}Turn on</button>
    </form>
  {:else}
    <div class="stack">
      <p class="muted">If you lose your phone, each of these signs you in once. Keep them somewhere safe, like a password manager. Fond won’t show them again.</p>
      <ul class="codes">{#each codes as c}<li>{c}</li>{/each}</ul>
      <div class="row wrap">
        <button class="btn sm" onclick={download}><Download />Download</button>
        <button class="btn sm" onclick={() => { copy(codes.join('\n'), 'Codes copied'); kept = true; }}><Copy />Copy</button>
      </div>
      <label class="row small"><input type="checkbox" bind:checked={kept} /> I’ve saved these codes</label>
      <button class="btn primary" disabled={!kept} onclick={() => (enrolling = false)}><Check />Done</button>
    </div>
  {/if}
</Sheet>

<Sheet open={!!asking} title={asking === 'off' ? 'Turn off two-step sign-in' : 'New recovery codes'} onclose={() => (asking = '')}>
  <form class="stack" onsubmit={withPassword}>
    <p class="muted">{asking === 'off' ? 'Signing in will only ask for your password.' : 'Your old recovery codes stop working.'} Enter your password to go on.</p>
    <!-- svelte-ignore a11y_autofocus -->
    <label class="field"><span>Password</span><input class="input" type="password" bind:value={password} autocomplete="current-password" required autofocus /></label>
    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
    <button class="btn" class:primary={asking === 'codes'} class:danger={asking === 'off'} disabled={busy}>{asking === 'off' ? 'Turn off' : 'Make new codes'}</button>
  </form>
</Sheet>

<style>
  section { display: grid; gap: 14px; }
  h2 { font: 600 1.5rem/1.2 var(--serif); }
  .state { display: flex; align-items: center; gap: 14px; padding: 14px 16px; border-radius: var(--r-l); background: var(--wash); }
  .state > :global(svg) { width: 26px; height: 26px; color: var(--bay); flex: none; }
  .state div { display: grid; }
  .how { display: grid; gap: 22px; counter-reset: s; }
  .how li { display: grid; gap: 10px; padding-left: 38px; position: relative; counter-increment: s; }
  .how li::before { content: counter(s); position: absolute; left: 0; top: -2px; width: 26px; height: 26px; border-radius: 50%; display: grid; place-items: center; background: var(--bay); color: var(--on-bay); font: 700 0.85rem/1 var(--sans); }
  .qr { width: fit-content; padding: 10px; border-radius: 14px; background: #fff; box-shadow: inset 0 0 0 1px var(--line); }
  .qr :global(svg) { display: block; }
  .key { padding: 6px 10px; border-radius: 8px; background: var(--sunk); font: 600 0.9rem/1.4 ui-monospace, 'SF Mono', Menlo, Consolas, monospace; letter-spacing: 0.04em; word-break: normal; }
  .code { font: 600 1.3rem/1 var(--sans); letter-spacing: 0.18em; text-align: center; max-width: 220px; }
  .codes { display: grid; grid-template-columns: repeat(2, 1fr); gap: 6px 18px; padding: 16px 18px; border-radius: var(--r-l); background: var(--sunk); font: 600 1rem/1.5 ui-monospace, 'SF Mono', Menlo, Consolas, monospace; letter-spacing: 0.06em; }
  input[type='checkbox'] { width: 18px; height: 18px; accent-color: var(--bay); }
</style>
