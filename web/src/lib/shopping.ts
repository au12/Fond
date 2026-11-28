// The shopping list adds itself up. Lines for the same food become one line with one amount:
// "1 cup flour" and "200 g flour" are one entry of 320 g. The cook can also merge lines by hand,
// or keep one apart. Lines are stored as written; everything here is worked out when the list is shown.

import { foodById, type Food } from './foods';
import {
  amountOf, lookup, metricVolume, metricWeight, parseIngredient, usVolume, usWeight, weigh, formatNumber,
  type Parsed, type System, type WeighOptions,
} from './ingredients';

export interface ShopItem { id: number; text: string; checked: boolean; recipeId: number | null; recipeTitle: string; group: string }

export interface ShopLine {
  key: string;
  items: ShopItem[];
  /** The amount, or '' when the line has none ("salt"). */
  amount: string;
  name: string;
  /** For a single line: the line as written. For a merged line: amount and name. */
  text: string;
  approx: boolean;
  checked: boolean;
  aisle: string;
  /** Recipe titles the line came from. */
  from: string[];
  /** Merged by the cook rather than by Fond. */
  manual: boolean;
}

export const AISLES = ['Produce', 'Meat and fish', 'Dairy and eggs', 'Bakery', 'Baking', 'Spices', 'Pantry', 'Frozen', 'Drinks', 'Other'] as const;

const BY_CATEGORY: Record<string, string> = {
  veg: 'Produce', fruit: 'Produce', herb: 'Produce',
  meat: 'Meat and fish',
  dairy: 'Dairy and eggs', cheese: 'Dairy and eggs', egg: 'Dairy and eggs', fat: 'Dairy and eggs',
  bread: 'Bakery',
  flour: 'Baking', sugar: 'Baking', starch: 'Baking', leaven: 'Baking', baking: 'Baking', nut: 'Baking', seed: 'Baking',
  spice: 'Spices', salt: 'Spices',
  grain: 'Pantry', legume: 'Pantry', sauce: 'Pantry', oil: 'Pantry', syrup: 'Pantry', misc: 'Pantry',
  liquid: 'Pantry',
};

const BY_NAME: [string, RegExp][] = [
  ['Frozen', /\bfrozen\b/i],
  ['Meat and fish', /\b(chicken|beef|pork|lamb|turkey|bacon|sausages?|ham|prosciutto|pancetta|steak|mince|ground (beef|pork|turkey|lamb)|salmon|tuna|cod|shrimp|prawns?|fish|anchov|chorizo|duck|veal|ribs|brisket|thighs?|breasts?|drumsticks?|scallops?|mussels|clams|crab|lobster)\b/i],
  ['Produce', /\b(onions?|garlic|shallots?|scallions?|leeks?|tomato(es)?|potato(es)?|carrots?|celery|peppers?|chil(i|e|li)(es)?|jalape|lettuce|spinach|kale|arugula|rocket|cabbage|broccoli|cauliflower|zucchini|courgettes?|squash|cucumbers?|mushrooms?|ginger|lemons?|limes?|oranges?|apples?|bananas?|berries|strawberr|blueberr|raspberr|avocados?|basil|parsley|cilantro|coriander leaves|mint|thyme|rosemary|dill|chives|sage|herbs?|eggplant|aubergine|beets?|radish|fennel|asparagus|green beans|peas|sprouts|pears?|peach|mango|pineapple|grapes?|melon|corn)\b/i],
  ['Dairy and eggs', /\b(milk|butter|cream|cheese|cheddar|mozzarella|parmesan|feta|ricotta|yogh?urt|eggs?|buttermilk|mascarpone|cr[eè]me fra[iî]che|half[- ]and[- ]half|ghee)\b/i],
  ['Bakery', /\b(bread|baguette|buns?|rolls?|tortillas?|pita|naan|bagels?|croissants?|brioche|sourdough)\b/i],
  ['Drinks', /\b(beer|wine|soda|sparkling water|coffee beans|juice|kombucha|seltzer)\b/i],
];

export function aisleOf(name: string, food: Food | null): string {
  if (/\bfrozen\b/i.test(name)) return 'Frozen';
  if (food) {
    if (food.c === 'liquid' && /\b(beer|wine|juice|coffee|tea|bourbon|rum|whisk|vodka|gin)\b/i.test(food.n + ' ' + name)) return 'Drinks';
    if (food.c === 'herb' && /\bdried\b/i.test(name + ' ' + food.n)) return 'Spices';
    const a = BY_CATEGORY[food.c];
    if (a) return a;
  }
  return BY_NAME.find(([, re]) => re.test(name))?.[0] ?? 'Other';
}

interface Read {
  item: ShopItem;
  p: Parsed;
  name: string;
  key: string;
  food: Food | null;
  /** A count unit the name carries: "garlic cloves" counts cloves. */
  tag: string;
}

/** The ingredient's name as a shopper would say it: no amount, no notes after the comma. */
function nameOf(p: Parsed, matched: string): string {
  const n = (matched || p.rest).split(/\s*[,(;]/)[0].trim();
  return n || p.raw;
}

function autoKey(p: Parsed, name: string, food: Food | null, via: string, skip: boolean): string {
  // A food Fond knows by name (or by the household's correction) joins every line of that food.
  if (food && !skip && via !== 'typo') return 'f:' + food.id;
  return 'n:' + name.toLowerCase().replace(/\b(fresh|large|small|medium|chopped|diced|minced|sliced)\b/g, '').replace(/(e?s)\b/g, '').replace(/[^a-z0-9]+/g, ' ').trim();
}

export function read(item: ShopItem, o: WeighOptions = {}): Read {
  const p = parseIngredient(item.text);
  const m = lookup(p, o);
  const name = nameOf(p, m.name);
  const food = m.food ?? null;
  let key = autoKey(p, name, food, m.via, !!m.skip);
  if (item.group.startsWith('m:')) key = item.group;
  else if (item.group.startsWith('solo:')) key = 'solo:' + item.id;
  return { item, p, name, key, food, tag: m.tag };
}

function plural(name: string, n: number): string {
  if (n <= 1 || /s$/i.test(name)) return name;
  const words = name.split(' ');
  const last = words.pop()!;
  const many = /(o)$/i.test(last) && /tomato|potato|mango/i.test(last) ? last + 'es' : /[^aeiou]y$/i.test(last) ? last.slice(0, -1) + 'ies' : /(ch|sh|x)$/i.test(last) ? last + 'es' : last + 's';
  return [...words, many].join(' ');
}

const COUNTED: Record<string, string> = { clove: 'cloves', stick: 'sticks', slice: 'slices', stalk: 'stalks', sprig: 'sprigs', head: 'heads', ear: 'ears', bunch: 'bunches', wedge: 'wedges', strip: 'strips' };
const US_UNITS = new Set(['tsp', 'tbsp', 'floz', 'cup', 'pint', 'quart', 'gallon', 'oz', 'lb']);

export interface SumOptions extends WeighOptions {
  system: System;
  fractions: boolean;
  /** The reader prefers weights: give a merged line in grams whenever every part can be weighed. */
  weigh: boolean;
}

/** One amount for several lines of the same food. */
export function sum(reads: Read[], o: SumOptions): { amount: string; approx: boolean; count: number } {
  let g = 0, ml = 0, bare = 0;
  let gN = 0, mlN = 0, usG = 0, usMl = 0;
  const counts = new Map<string, { n: number; one: string; many: string }>();
  let weighable = true;
  let gAll = 0;
  let approx = false;
  for (const r of reads) {
    const p = r.p;
    const w = p.qty != null ? weigh(p, o) : null;
    if (w) {
      gAll += w.g;
      approx ||= w.approx;
    } else weighable = false;
    if (p.qty == null) {
      bare++;
      continue;
    }
    if (p.unit?.kind === 'weight' || (p.alt?.unit.kind === 'weight' && p.unit?.kind !== 'volume')) {
      g += w?.g ?? 0;
      gN++;
      if (US_UNITS.has((p.unit?.kind === 'weight' ? p.unit : p.alt!.unit).key)) usG++;
      continue;
    }
    const a = amountOf(p);
    if (a && 'ml' in a) {
      ml += a.ml;
      mlN++;
      if (US_UNITS.has(p.unit!.key)) usMl++;
      continue;
    }
    const n = a && 'count' in a ? a.count : p.qty;
    const tagged = !p.unit && r.tag && COUNTED[r.tag] ? r.tag : '';
    const k = p.unit ? p.unit.key : tagged;
    const c = counts.get(k) ?? { n: 0, one: p.unit?.one ?? tagged, many: p.unit?.many ?? COUNTED[tagged] ?? '' };
    c.n += n;
    counts.set(k, c);
  }
  const kinds = (gN ? 1 : 0) + (mlN ? 1 : 0) + counts.size;
  const usWeights = o.system === 'us' || (o.system === 'original' && gN > 0 && usG === gN);
  const usVolumes = o.system === 'us' || (o.system === 'original' && (mlN === 0 || usMl > 0));
  const wt = (grams: number) => {
    const r = usWeights ? usWeight(grams, null, o.fractions) : metricWeight(grams, null, true);
    return `${r.text} ${r.unit}`;
  };
  // Different kinds of measure that can all be weighed are added up as one weight.
  if (weighable && gAll > 0 && (kinds > 1 || (o.weigh && mlN > 0))) {
    // Fond weighs in grams; a recipe that weighed in ounces keeps ounces.
    const usual = o.system === 'us' || (o.system === 'original' && gN > 0 && usG === gN);
    const r = usual ? usWeight(gAll, null, o.fractions) : metricWeight(gAll, null, true);
    return { amount: `${r.text} ${r.unit}`, approx, count: 0 };
  }
  const parts: string[] = [];
  if (gN) parts.push(wt(g));
  if (mlN) parts.push(usVolumes ? usVolumeSum(ml, o.fractions) : (({ text, unit }) => `${text} ${unit}`)(metricVolume(ml, null)));
  let count = 0;
  for (const [k, c] of counts) {
    if (!k) {
      count = c.n;
      parts.push(formatNumber(c.n, o.fractions));
    } else parts.push(`${formatNumber(c.n, o.fractions)} ${c.n > 1 ? c.many : c.one}`);
  }
  return { amount: parts.join(' + '), approx: false, count: counts.size === 1 && counts.has('') && !gN && !mlN ? count : 0 };
}

/** A sum of cups and spoons, kept exact: 1 cup and 2 tablespoons is "1 cup + 2 tbsp", not "1¼ cups". */
function usVolumeSum(ml: number, fractions: boolean): string {
  const r = usVolume(ml, null, fractions);
  const one = `${r.text} ${r.unit}`;
  const CUP = 236.588, TBSP = 14.7868;
  if (ml < CUP / 2) return one;
  const cups = Math.floor((ml / CUP) * 4 + 1e-6) / 4;
  const tbsp = Math.round(((ml - cups * CUP) / TBSP) * 2) / 2;
  const back = cups * CUP + tbsp * TBSP;
  const VULGAR: Record<string, number> = { '¼': 0.25, '½': 0.5, '¾': 0.75, '⅓': 1 / 3, '⅔': 2 / 3, '⅛': 0.125, '⅜': 0.375, '⅝': 0.625, '⅞': 0.875 };
  const PER: Record<string, number> = { cup: CUP, cups: CUP, tbsp: TBSP, tsp: TBSP / 3 };
  let q = 0;
  for (const t of r.text.match(/[\d.]+|[¼½¾⅓⅔⅛⅜⅝⅞]/g) ?? []) q += VULGAR[t] ?? (parseFloat(t) || 0);
  const shownMl = q * (PER[r.unit] ?? 0) || ml;
  if (tbsp < 1 || Math.abs(shownMl - ml) / ml <= 0.03 || Math.abs(back - ml) / ml > 0.03) return one;
  return `${formatNumber(cups, fractions)} cup${cups > 1 ? 's' : ''} + ${formatNumber(tbsp, fractions)} tbsp`;
}

/** Joins the list into lines, one per food (or per hand-made group), sorted into aisles. */
export function combine(items: ShopItem[], o: SumOptions): ShopLine[] {
  const groups = new Map<string, Read[]>();
  for (const it of items) {
    const r = read(it, o);
    const g = groups.get(r.key);
    if (g) g.push(r);
    else groups.set(r.key, [r]);
  }
  const out: ShopLine[] = [];
  for (const [key, reads] of groups) {
    const first = reads[0];
    const from = [...new Set(reads.map((r) => r.item.recipeTitle).filter(Boolean))];
    const checked = reads.every((r) => r.item.checked);
    const food = reads.find((r) => r.food)?.food ?? null;
    const manual = key.startsWith('m:');
    if (reads.length === 1) {
      // A line on its own reads as written, with its measure picked out.
      const p = first.p;
      const raw = first.item.text.trim();
      const at = p.qty != null && p.head ? raw.indexOf(p.head) : -1;
      const amount = at === 0 || (at > 0 && at <= p.prefix.length + 1) ? raw.slice(0, at + p.head.length).trim() : '';
      const name = amount ? raw.slice(amount.length).replace(/^\s*(of\s+)?/, '').trim() : raw;
      out.push({ key, items: [first.item], amount, name, text: raw, approx: false, checked, aisle: aisleOf(raw, food), from, manual: false });
      continue;
    }
    // The name most lines use, and the shorter one when it's a tie: "eggs" over "large free-range eggs".
    const tally = new Map<string, number>();
    for (const r of reads) tally.set(r.name.toLowerCase(), (tally.get(r.name.toLowerCase()) ?? 0) + 1);
    const best = [...tally.entries()].sort((a, b) => b[1] - a[1] || a[0].length - b[0].length)[0][0];
    const written = reads.find((r) => r.name.toLowerCase() === best)!.name;
    const s = sum(reads, o);
    const name = s.count ? plural(written, s.count) : written;
    const amount = s.amount;
    out.push({
      key, items: reads.map((r) => r.item), amount, name, text: amount ? `${amount} ${name}` : name, approx: s.approx, checked,
      aisle: aisleOf(reads.map((r) => r.item.text).join(' '), food), from, manual,
    });
  }
  return out;
}

export const foodName = (id: string) => foodById(id)?.n ?? '';
