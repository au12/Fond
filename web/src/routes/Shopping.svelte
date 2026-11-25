<script lang="ts">
  import { flip } from 'svelte/animate';
  import { fade } from 'svelte/transition';
  import { del, get, post, put } from '../lib/api';
  import { oops, toast } from '../lib/state.svelte';
  import { Check, Plus, ShoppingBasket, X } from '../lib/icons';

  interface Item { id: number; text: string; checked: boolean; recipeId: number | null; recipeTitle: string }
  let items = $state<Item[]>([]);
  let loaded = $state(false);
  let draft = $state('');

  const AISLES: [string, RegExp][] = [
    ['Produce', /\b(onions?|garlic|shallots?|scallions?|leeks?|tomato(es)?|potato(es)?|carrots?|celery|peppers?|chil(i|e|li)(es)?|jalape|lettuce|spinach|kale|arugula|cabbage|broccoli|cauliflower|zucchini|squash|cucumber|mushrooms?|ginger|lemons?|limes?|oranges?|apples?|bananas?|berries|strawberr|blueberr|avocados?|basil|parsley|cilantro|coriander leaves|mint|thyme|rosemary|dill|chives|herbs?|corn on|eggplant|aubergine|beets?|radish|fennel|asparagus|green beans|peas|sprouts|pears?|peach|mango|pineapple|grapes?|melon)\b/i],
    ['Meat and fish', /\b(chicken|beef|pork|lamb|turkey|bacon|sausages?|ham|prosciutto|pancetta|steak|mince|ground (beef|pork|turkey)|salmon|tuna|cod|shrimp|prawns?|fish|anchov|chorizo|duck|veal|ribs|brisket|thighs?|breasts?)\b/i],
    ['Dairy and eggs', /\b(milk|butter|cream|cheese|cheddar|mozzarella|parmesan|feta|ricotta|yogh?urt|eggs?|buttermilk|mascarpone|cr[eè]me fra[iî]che|half[- ]and[- ]half|ghee)\b/i],
    ['Bakery', /\b(bread|baguette|buns?|rolls?|tortillas?|pita|naan|bagels?|croissants?|brioche)\b/i],
    ['Frozen', /\bfrozen\b/i],
    ['Pantry', /./],
  ];
  const aisle = (t: string) => AISLES.find(([, re]) => re.test(t))![0];

  get<Item[]>('/shopping').then((x) => { items = x; loaded = true; }).catch(oops);

  const open = $derived(items.filter((i) => !i.checked));
  const done = $derived(items.filter((i) => i.checked));
  const groups = $derived(AISLES.map(([name]) => ({ name, items: open.filter((i) => aisle(i.text) === name) })).filter((g) => g.items.length));

  async function add(e: SubmitEvent) {
    e.preventDefault();
    const texts = draft.split(/[\n,]/).map((s) => s.trim()).filter(Boolean);
    if (!texts.length) return;
    draft = '';
    try {
      items = await post('/shopping', { items: texts.map((text) => ({ text })) });
    } catch (err) {
      oops(err);
    }
  }
  function toggle(it: Item) {
    it.checked = !it.checked;
    navigator.vibrate?.(8);
    put(`/shopping/${it.id}`, { checked: it.checked }).catch(oops);
  }
  function remove(it: Item) {
    items = items.filter((x) => x.id !== it.id);
    del(`/shopping/${it.id}`).catch(oops);
  }
  async function clear(all: boolean) {
    if (all && !confirm('Clear the whole shopping list?')) return;
    items = await post('/shopping/clear', { all });
    toast(all ? 'Shopping list cleared' : 'Ticked items removed');
  }
</script>

<div class="page narrow">
  <div class="spread" style:margin-bottom="18px">
    <h1 class="page-title">Shopping list</h1>
    {#if items.length}<button class="btn quiet sm" onclick={() => clear(true)}>Clear all</button>{/if}
  </div>
  <form class="add" onsubmit={add}>
    <input class="input" bind:value={draft} placeholder="Add an item, or several separated by commas" aria-label="Add an item" enterkeyhint="done" />
    <button class="btn primary" disabled={!draft.trim()} aria-label="Add"><Plus /></button>
  </form>

  {#if loaded && items.length === 0}
    <div class="empty">
      <span style="color:var(--bay)"><ShoppingBasket size={56} strokeWidth={1.25} /></span>
      <h2>Nothing to buy</h2>
      <p>Add items here, or open a recipe and choose “Add to shopping list”. Ingredients you’ve already ticked off in a recipe are left out.</p>
    </div>
  {/if}

  {#each groups as g (g.name)}
    <section>
      <h2>{g.name}</h2>
      <ul>
        {#each g.items as it (it.id)}
          <li animate:flip={{ duration: 220 }} out:fade={{ duration: 120 }}>
            <button class="item" onclick={() => toggle(it)} aria-pressed="false"><span class="box"><Check /></span><span class="grow">{it.text}{#if it.recipeTitle}<small>{it.recipeTitle}</small>{/if}</span></button>
            <button class="icon-btn rm" onclick={() => remove(it)} aria-label="Remove"><X /></button>
          </li>
        {/each}
      </ul>
    </section>
  {/each}

  {#if done.length}
    <section class="got">
      <div class="spread"><h2>In the basket</h2><button class="btn quiet sm" onclick={() => clear(false)}>Remove ticked</button></div>
      <ul>
        {#each done as it (it.id)}
          <li animate:flip={{ duration: 220 }}>
            <button class="item done" onclick={() => toggle(it)} aria-pressed="true"><span class="box checked"><Check /></span><span class="grow">{it.text}</span></button>
          </li>
        {/each}
      </ul>
    </section>
  {/if}
</div>

<style>
  .add { display: flex; gap: 8px; margin-bottom: 26px; }
  .add .input { height: 50px; border-radius: 999px; padding: 0 20px; }
  .add .btn { width: 50px; height: 50px; padding: 0; flex: none; }
  section { margin-bottom: 24px; }
  h2 { font: italic 600 1.15rem var(--serif); color: var(--ink-2); margin-bottom: 4px; }
  li { display: flex; align-items: center; border-bottom: 1px solid var(--line); }
  .item { flex: 1; display: flex; align-items: center; gap: 14px; padding: 13px 2px; text-align: left; font-size: 1.05rem; line-height: 1.3; }
  .item small { display: block; font-size: 0.78rem; color: var(--ink-3); margin-top: 1px; }
  .box { flex: none; width: 24px; height: 24px; border-radius: 50%; display: grid; place-items: center; box-shadow: inset 0 0 0 1.5px var(--line-2); color: transparent; transition: background 0.2s, transform 0.25s var(--spring); }
  .box :global(svg) { width: 14px; height: 14px; stroke-width: 3; }
  .box.checked { background: var(--bay); box-shadow: none; color: var(--on-bay); }
  .item.done { color: var(--ink-3); text-decoration: line-through; }
  .rm { opacity: 0.5; }
  .got { margin-top: 34px; }
</style>
