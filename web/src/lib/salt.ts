// "1 teaspoon kosher salt" is 2.8 g of Diamond Crystal or 4.8 g of Morton, and
// most recipes do not say which. Cooks salt to a fairly narrow range for each
// kind of dish, though, so the amount usually gives the brand away: a loaf
// with 500 g of flour and a tablespoon of kosher salt is 1.7 % salt with
// Diamond Crystal and 2.9 % with Morton, and bread is made at about 2 %.
//
// This module makes that inference. It is a suggestion: the weight is still
// shown as an estimate, and the cook can overrule it for the recipe.

import { foodById } from './foods';
import { bakers, lookup, parseIngredient, weigh, type Parsed, type WeighOptions } from './ingredients';
import type { Recipe } from './types';

export type Kind = 'bread' | 'pizza' | 'enriched' | 'pastry' | 'cookies' | 'cake' | 'pasta' | 'baked' | 'meat' | 'soup' | 'dish';

/** Typical salt, as a percentage of the flour, the meat or the whole dish. */
export const SALT_RANGES: Record<Kind, { label: string; of: 'flour' | 'meat' | 'dish'; lo: number; hi: number }> = {
  bread: { label: 'bread', of: 'flour', lo: 1.8, hi: 2.2 },
  pizza: { label: 'pizza and focaccia dough', of: 'flour', lo: 2, hi: 3 },
  enriched: { label: 'enriched dough', of: 'flour', lo: 1.5, hi: 2 },
  pastry: { label: 'pastry', of: 'flour', lo: 1.5, hi: 2.5 },
  cookies: { label: 'cookies', of: 'flour', lo: 0.5, hi: 1.5 },
  cake: { label: 'cakes and quick breads', of: 'flour', lo: 0.5, hi: 1.25 },
  pasta: { label: 'pasta dough', of: 'flour', lo: 0.5, hi: 1.5 },
  baked: { label: 'baking', of: 'flour', lo: 0.5, hi: 2.2 },
  meat: { label: 'seasoned meat', of: 'meat', lo: 0.5, hi: 1.5 },
  soup: { label: 'soups, stews and sauces', of: 'dish', lo: 0.5, hi: 1 },
  dish: { label: 'savoury dishes', of: 'dish', lo: 0.5, hi: 1.2 },
};

export interface SaltGuess {
  /** The brand the amounts point to, or null when they fit both or neither. */
  brand: 'dc' | 'morton' | null;
  kind: Kind;
  /** Teaspoons of unbranded kosher salt in the recipe. */
  spoons: number;
  /** Total salt as a percentage of the basis, for each brand. */
  percent: { dc: number; morton: number } | null;
  /** The figures behind the judgement, for the cook. */
  facts: string;
  /** What Fond concluded from them. */
  verdict: string;
}

const TSP = 4.92892;
const KEYWORDS: [RegExp, Kind][] = [
  [/pizza|focaccia/, 'pizza'],
  [/pasta|noodle|gnocchi|ravioli|tagliatelle|dumpling|pierogi/, 'pasta'],
  [/brioche|challah|babka|cinnamon roll|sweet roll|sticky bun|milk bread|doughnut|donut|panettone|stollen|kolache|hot cross/, 'enriched'],
  [/cookie|shortbread|brownie|blondie|biscotti|snickerdoodle|macaroon|\bbars?\b/, 'cookies'],
  [/cake|cupcake|muffin|banana bread|zucchini bread|pumpkin bread|quick bread|cornbread|pancake|waffle|gingerbread|crepe/, 'cake'],
  [/\bpie\b|tart|galette|crust|pastry|scone|biscuit|croissant|danish|puff|empanada|turnover/, 'pastry'],
  [/bread|loaf|sourdough|baguette|\brolls?\b|bagel|\bbuns?\b|ciabatta|pretzel|naan|pita|flatbread|tortilla|boule|batard/, 'bread'],
];
const MEAT = /\b(chicken|beef|pork|lamb|turkey|veal|duck|steaks?|brisket|ribs?|roast|chuck|sirloin|tenderloin|loin|shoulder|thighs?|breasts?|drumsticks?|wings?|sausages?|mince|salmon|cod|halibut|tilapia|shrimp|prawns?|tuna|fish|fillets?)\b/i;
const SOUP = /soup|stew|chili|chowder|broth|sauce|curry|braise|stock|ragu|bolognese|dal|gravy|bisque/;
/** Ingredients that bring plenty of salt of their own, so a dish may be right with little added. */
const SALTY = /soy sauce|tamari|fish sauce|miso|bouillon|broth|stock|parmesan|pecorino|feta|bacon|pancetta|ham|anchov|olives?|capers?|sausage|chorizo|salted butter|worcestershire|oyster sauce|hoisin/i;

/** How far a percentage sits outside a range, as a ratio; 1 means inside. */
const off = (x: number, lo: number, hi: number) => (x <= 0 ? Infinity : x < lo ? lo / x : x > hi ? x / hi : 1);
const pct = (x: number) => (x >= 10 ? x.toFixed(0) : x.toFixed(1)).replace(/\.0$/, '');

export function saltFor(r: Pick<Recipe, 'title' | 'tags' | 'ingredients' | 'extra'>, o: WeighOptions = {}): SaltGuess | null {
  const items: Parsed[] = r.ingredients.filter((l) => !l.h).map((l) => parseIngredient(l.text));
  const plain: WeighOptions = { ...o, salt: null };
  let kosherMl = 0;
  let otherSalt = 0;
  let total = 0;
  let meat = 0;
  let liquid = 0;
  let sugar = 0;
  let fat = 0;
  let leavened = false;
  let salty = false;
  let measurable = 0;
  let weighed = 0;
  for (const p of items) {
    if (p.qty == null) continue;
    const m = lookup(p, plain);
    if (m.skip) continue;
    const food = m.food;
    if (food?.t === 'kosher' && p.unit?.kind === 'volume' && !p.alt) {
      const q = p.qtyMax != null ? (p.qty + p.qtyMax) / 2 : p.qty;
      kosherMl += q * p.unit.base + p.more.reduce((s, x) => s + x.qty * x.unit.base, 0);
      continue;
    }
    const w = weigh(p, plain);
    if (p.unit?.kind === 'volume' || p.unit?.kind === 'weight') {
      measurable++;
      if (w) weighed++;
    }
    if (SALTY.test(p.rest)) salty = true;
    if (!w) continue;
    if (food?.c === 'salt') {
      otherSalt += w.g;
      continue;
    }
    total += w.g;
    if (MEAT.test(p.rest) && !food) meat += w.g;
    if (food?.c === 'liquid' || (food?.c === 'dairy' && (food.w ?? 0) >= 0.8)) liquid += w.g;
    if (food?.c === 'sugar' || food?.c === 'syrup') sugar += w.g;
    if (food?.c === 'fat' || food?.c === 'oil') fat += w.g;
    if (food?.id === 'yeast' || food?.id === 'starter') leavened = true;
  }
  if (kosherMl <= 0) return null;
  const spoons = kosherMl / TSP;

  const bake = bakers(items, { ...plain, flourLines: r.extra?.flour, starter: r.extra?.starter });
  const flour = bake.complete ? bake.flour : 0;
  const text = `${r.title} ${r.tags.join(' ')}`.toLowerCase();
  const named = KEYWORDS.find(([re]) => re.test(text))?.[1];
  let kind: Kind;
  let basis = 0;
  // Something baked: the title says so, or flour is a good part of it (not a spoonful in a stew).
  if (flour > 0 && (named ? flour >= 0.08 * total : flour >= 0.2 * total)) {
    basis = flour;
    kind = named ?? 'baked';
    if (kind === 'baked') {
      const s = sugar / flour;
      const f = fat / flour;
      if (leavened) kind = s > 0.12 || f > 0.15 ? 'enriched' : 'bread';
      else if (s >= 0.6) kind = f >= 0.45 ? 'cookies' : 'cake';
      else if (f >= 0.45 && s < 0.25) kind = 'pastry';
    }
  } else if (meat > 0 && meat >= 0.35 * total) {
    kind = 'meat';
    basis = meat;
  } else {
    kind = SOUP.test(text) || liquid >= 0.4 * total ? 'soup' : 'dish';
    // The whole dish only means something when most of it could be weighed.
    basis = measurable > 0 && weighed / measurable >= 0.7 ? total : 0;
  }

  const range = SALT_RANGES[kind];
  const dc = foodById('salt-dc')?.d ?? 2.8 / TSP;
  const morton = foodById('salt-morton')?.d ?? 4.8 / TSP;
  const amount = spoons >= 3 && Math.abs(spoons / 3 - Math.round((spoons / 3) * 2) / 2) < 0.01 ? `${pct(spoons / 3)} tbsp` : `${pct(spoons)} tsp`;
  if (basis <= 0) {
    return { brand: null, kind, spoons, percent: null, facts: '', verdict: `Fond could not weigh enough of this recipe to tell which kosher salt ${amount} means.` };
  }
  const x = { dc: ((kosherMl * dc + otherSalt) / basis) * 100, morton: ((kosherMl * morton + otherSalt) / basis) * 100 };
  const d = { dc: off(x.dc, range.lo, range.hi), morton: off(x.morton, range.lo, range.hi) };
  let brand: SaltGuess['brand'] = null;
  if (range.of === 'flour') {
    if (Math.abs(d.dc - d.morton) >= 0.05) brand = d.dc < d.morton ? 'dc' : 'morton';
  } else if (x.morton > range.hi && x.dc <= range.hi * 1.15) {
    // Morton would oversalt it; Diamond Crystal would not.
    brand = 'dc';
  } else if (x.dc < range.lo && d.morton === 1 && !salty) {
    // Diamond Crystal would leave it flat, and nothing else in the dish is salty.
    brand = 'morton';
  }
  const of = range.of === 'dish' ? 'the dish' : `the ${range.of}`;
  const facts = `${amount} is ${pct(x.dc)} % of ${of} as Diamond Crystal and ${pct(x.morton)} % as Morton. The usual range for ${range.label} is ${pct(range.lo)}–${pct(range.hi)} %.`;
  const verdict = brand
    ? `That points to ${brand === 'dc' ? 'Diamond Crystal' : 'Morton'}.`
    : `That fits ${d.dc === 1 && d.morton === 1 ? 'either' : 'neither clearly'}, so Fond uses a weight halfway between.`;
  return { brand, kind, spoons, percent: x, facts, verdict };
}
