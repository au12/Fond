<script lang="ts">
  import Logo from '../components/Logo.svelte';
  import { post } from '../lib/api';
  import { app, go, setSession } from '../lib/state.svelte';
  import { ArrowLeft, Eye, EyeOff, Loader, ShieldCheck } from '../lib/icons';

  let username = $state('');
  let name = $state('');
  let password = $state('');
  let show = $state(false);
  let busy = $state(false);
  let error = $state('');
  let shake = $state(false);
  // After a right password, an account with two-step sign-in waits here for its code.
  let ticket = $state('');
  let code = $state('');
  let recovery = $state(false);
  let codeInput = $state<HTMLInputElement | null>(null);

  function done(me: any) {
    setSession(me);
    const next = new URLSearchParams(app.query).get('next');
    if (app.path === '/login') go(next && next.startsWith('/') && !next.startsWith('//') ? next : '/', { replace: true });
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (busy) return;
    busy = true;
    error = '';
    try {
      if (ticket) {
        done(await post('/auth/code', { ticket, code }));
        return;
      }
      const me = await post(app.setup ? '/auth/setup' : '/auth/login', { username, name, password });
      if (me.twoFactor) {
        ticket = me.ticket;
        password = '';
        code = '';
        setTimeout(() => codeInput?.focus(), 60);
        return;
      }
      done(me);
    } catch (err: any) {
      if (ticket && /expired/i.test(err.message)) {
        ticket = '';
        code = '';
      }
      error = err.message;
      shake = true;
      setTimeout(() => (shake = false), 400);
    } finally {
      busy = false;
    }
  }
</script>

<div class="login">
  <form class="panel" class:shake onsubmit={submit}>
    <div class="brand"><Logo size={40} /><span>Fond</span></div>
    {#if ticket}
      <div class="shield"><ShieldCheck /></div>
      <h1>Enter your code</h1>
      <p class="muted">{recovery ? 'Type one of the recovery codes you saved when you turned on two-step sign-in. Each works once.' : 'Open your authenticator app and enter the six-digit code for Fond.'}</p>
      <label class="field">
        <span>{recovery ? 'Recovery code' : 'Code'}</span>
        {#if recovery}
          <input class="input code" bind:this={codeInput} bind:value={code} autocomplete="off" autocapitalize="none" spellcheck="false" placeholder="xxxx-xxxx" required />
        {:else}
          <input class="input code" bind:this={codeInput} bind:value={code} inputmode="numeric" autocomplete="one-time-code" pattern="[0-9 ]*" maxlength="7" placeholder="123 456" required
            oninput={() => { if (code.replace(/\D/g, '').length === 6 && !busy) (codeInput?.form as HTMLFormElement | null)?.requestSubmit(); }} />
        {/if}
      </label>
      {#if error}<p class="error-text" role="alert">{error}</p>{/if}
      <button class="btn primary" disabled={busy}>{#if busy}<span class="spin" style="display:grid"><Loader /></span>{/if}Sign in</button>
      <div class="spread small">
        <button type="button" class="linkish" onclick={() => { ticket = ''; error = ''; }}><ArrowLeft />Back</button>
        <button type="button" class="linkish" onclick={() => { recovery = !recovery; code = ''; error = ''; setTimeout(() => codeInput?.focus(), 30); }}>{recovery ? 'Use the authenticator app' : 'Use a recovery code'}</button>
      </div>
    {:else}
    {#if app.setup}
      <h1>Set up your kitchen</h1>
      <p class="muted">Create the first account. It becomes the admin, and you can add the rest of the household afterwards.</p>
      <label class="field"><span>Your name</span><input class="input" bind:value={name} autocomplete="name" placeholder="Adam" /></label>
    {:else}
      <h1>Welcome back</h1>
    {/if}
    <label class="field"><span>Username</span><input class="input" bind:value={username} autocomplete="username" autocapitalize="none" spellcheck="false" required /></label>
    <label class="field">
      <span>Password</span>
      <div class="pw">
        <input class="input" type={show ? 'text' : 'password'} bind:value={password} autocomplete={app.setup ? 'new-password' : 'current-password'} minlength={app.setup ? 8 : 1} required />
        <button type="button" class="icon-btn" onclick={() => (show = !show)} aria-label={show ? 'Hide password' : 'Show password'}>{#if show}<EyeOff />{:else}<Eye />{/if}</button>
      </div>
      {#if app.setup}<span class="hint">At least 8 characters. A short phrase works well.</span>{/if}
    </label>
    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
    <button class="btn primary" disabled={busy}>{#if busy}<span class="spin" style="display:grid"><Loader /></span>{/if}{app.setup ? 'Create account' : 'Sign in'}</button>
    {/if}
  </form>
</div>

<style>
  .login { min-height: 100dvh; display: grid; place-items: center; padding: 24px; background: radial-gradient(1200px 600px at 50% -10%, var(--wash), transparent 70%), var(--paper); }
  .panel { width: min(400px, 100%); display: grid; gap: 16px; padding: 34px 30px; background: var(--surface); border-radius: 24px; box-shadow: var(--shadow), inset 0 0 0 1px var(--line); animation: rise 0.5s var(--ease) both; }
  .brand { display: flex; align-items: center; gap: 10px; color: var(--bay); font: italic 600 2.2rem/1 var(--serif); }
  h1 { font: 600 1.5rem/1.2 var(--serif); margin-top: 6px; }
  .pw { position: relative; }
  .pw .input { padding-right: 46px; }
  .pw .icon-btn { position: absolute; right: 2px; top: 1px; }
  .btn { height: 46px; margin-top: 4px; }
  .shield { width: 52px; height: 52px; border-radius: 50%; display: grid; place-items: center; background: var(--wash); color: var(--bay); animation: rise 0.4s var(--ease) both; }
  .shield :global(svg) { width: 26px; height: 26px; }
  .code { font: 600 1.35rem/1 var(--sans); letter-spacing: 0.18em; text-align: center; font-variant-numeric: tabular-nums; }
  .linkish { display: inline-flex; align-items: center; gap: 6px; color: var(--bay); font-weight: 600; }
  .linkish :global(svg) { width: 16px; height: 16px; }
</style>
