<script lang="ts">
  import Logo from '../components/Logo.svelte';
  import { post } from '../lib/api';
  import { app, go, setSession } from '../lib/state.svelte';
  import { Eye, EyeOff, Loader } from '../lib/icons';

  let username = $state('');
  let name = $state('');
  let password = $state('');
  let show = $state(false);
  let busy = $state(false);
  let error = $state('');
  let shake = $state(false);

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (busy) return;
    busy = true;
    error = '';
    try {
      const me = await post(app.setup ? '/auth/setup' : '/auth/login', { username, name, password });
      setSession(me);
      const next = new URLSearchParams(app.query).get('next');
      if (app.path === '/login') go(next && next.startsWith('/') && !next.startsWith('//') ? next : '/', { replace: true });
    } catch (err: any) {
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
</style>
