// Ingredient understanding. Recipes store ingredients as plain text; this
// module reads a line like "2 1/2 cups (300 g) flour, sifted" and can scale
// it, convert it between measurement systems, and weigh it for baker's maths.
// What an ingredient weighs comes from the dictionary in ./foods.

import { basisText, densityOf, eachOf, foodById, matchFood, type Food, type Match } from './foods';

export type System = 'original' | 'metric' | 'us';

/** A correction for one line of one recipe, keyed by the line as written. */
export interface Fix {
  /** Treat the line as this dictionary food. */
  foodId?: string;
  gPerMl?: number;
  gEach?: number;
  /** Leave the line as written. */
  skip?: boolean;
}

export interface WeighOptions {
  /** How a cup of flour is filled: spooned and levelled (default) or dipped and swept. */
  flour?: 'spoon' | 'dip';
  /** Which kosher salt the recipe means, once known or inferred. */
  salt?: { brand: 'dc' | 'morton'; sure: boolean } | null;
  fixes?: Record<string, Fix>;
  /** Whose convention set the flour method, when the recipe's source did: "King Arthur Baking". */
  house?: string;
}

export interface DisplayOptions extends WeighOptions {
  scale: number;
  system: System;
  /** Prefer weights over volumes when the ingredient is known. */
  weigh: boolean;
  /** Vulgar fractions (½) instead of decimals for cups, spoons and counts. */
  fractions: boolean;
}

interface Unit {
  key: string;
  kind: 'volume' | 'weight' | 'other';
  system: 'us' | 'metric' | 'any';
  /** millilitres (volume) or grams (weight) per unit */
  base: number;
  one: string;
  many: string;
  names: string[];
  container?: boolean;
}

const U = (key: string, kind: Unit['kind'], system: Unit['system'], base: number, one: string, many: string, names: string[], container = false): Unit => ({
  key, kind, system, base, one, many, names, container,
});

export const UNITS: Unit[] = [
  U('tsp', 'volume', 'us', 4.92892, 'tsp', 'tsp', ['teaspoons', 'teaspoon', 'tsps', 'tsp', 'tspn', 't']),
  U('tbsp', 'volume', 'us', 14.7868, 'tbsp', 'tbsp', ['tablespoons', 'tablespoon', 'tbsps', 'tbsp', 'tbs', 'tbl', 'tb', 'T']),
  U('floz', 'volume', 'us', 29.5735, 'fl oz', 'fl oz', ['fluid ounces', 'fluid ounce', 'fl. oz.', 'fl oz', 'fl. oz', 'floz']),
  U('cup', 'volume', 'us', 236.588, 'cup', 'cups', ['cups', 'cup', 'c']),
  U('pint', 'volume', 'us', 473.176, 'pint', 'pints', ['pints', 'pint', 'pt']),
  U('quart', 'volume', 'us', 946.353, 'quart', 'quarts', ['quarts', 'quart', 'qts', 'qt']),
  U('gallon', 'volume', 'us', 3785.41, 'gallon', 'gallons', ['gallons', 'gallon', 'gal']),
  U('ml', 'volume', 'metric', 1, 'ml', 'ml', ['milliliters', 'milliliter', 'millilitres', 'millilitre', 'mls', 'ml']),
  U('cl', 'volume', 'metric', 10, 'cl', 'cl', ['centiliters', 'centilitres', 'cl']),
  U('dl', 'volume', 'metric', 100, 'dl', 'dl', ['deciliters', 'decilitres', 'dl']),
  U('l', 'volume', 'metric', 1000, 'l', 'l', ['liters', 'liter', 'litres', 'litre', 'l']),
  U('mg', 'weight', 'metric', 0.001, 'mg', 'mg', ['milligrams', 'milligram', 'mg']),
  U('g', 'weight', 'metric', 1, 'g', 'g', ['grams', 'gram', 'grammes', 'gr', 'g']),
  U('kg', 'weight', 'metric', 1000, 'kg', 'kg', ['kilograms', 'kilogram', 'kilos', 'kilo', 'kgs', 'kg']),
  U('oz', 'weight', 'us', 28.3495, 'oz', 'oz', ['ounces', 'ounce', 'oz']),
  U('lb', 'weight', 'us', 453.592, 'lb', 'lb', ['pounds', 'pound', 'lbs', 'lb']),
  ...(
    [
      ['clove', 'cloves'], ['pinch', 'pinches'], ['dash', 'dashes'], ['bunch', 'bunches'], ['sprig', 'sprigs'], ['head', 'heads'],
      ['stalk', 'stalks'], ['handful', 'handfuls'], ['drop', 'drops'], ['slice', 'slices'], ['piece', 'pieces'], ['sheet', 'sheets'],
      ['strip', 'strips'], ['ear', 'ears'], ['fillet', 'fillets'], ['rib', 'ribs'], ['knob', 'knobs'], ['cube', 'cubes'], ['leaf', 'leaves'],
      ['inch', 'inches'], ['cm', 'cm'], ['splash', 'splashes'], ['rasher', 'rashers'], ['wedge', 'wedges'], ['scoop', 'scoops'],
    ] as const
  ).map(([one, many]) => U(one, 'other', 'any', 0, one, many, [many, one])),
  ...(
    [
      ['can', 'cans'], ['tin', 'tins'], ['jar', 'jars'], ['package', 'packages'], ['packet', 'packets'], ['pack', 'packs'], ['pkg', 'pkgs'],
      ['stick', 'sticks'], ['bottle', 'bottles'], ['bag', 'bags'], ['box', 'boxes'], ['block', 'blocks'], ['carton', 'cartons'],
      ['tub', 'tubs'], ['container', 'containers'], ['envelope', 'envelopes'], ['loaf', 'loaves'], ['sachet', 'sachets'],
    ] as const
  ).map(([one, many]) => U(one, 'other', 'any', 0, one, many, [many, one], true)),
];

const UNIT_BY_KEY = new Map(UNITS.map((u) => [u.key, u]));
// Longest names first so "tablespoons" wins over "t".
const UNIT_NAMES: [string, Unit][] = UNITS.flatMap((u) => u.names.map((n) => [n, u] as [string, Unit])).sort((a, b) => b[0].length - a[0].length);

const VULGAR: Record<string, string> = {
  '½': '1/2', '⅓': '1/3', '⅔': '2/3', '¼': '1/4', '¾': '3/4', '⅕': '1/5', '⅖': '2/5', '⅗': '3/5', '⅘': '4/5',
  '⅙': '1/6', '⅚': '5/6', '⅛': '1/8', '⅜': '3/8', '⅝': '5/8', '⅞': '7/8',
};

const NUM = String.raw`(?:\d+\s+\d+\s*/\s*\d+|\d+\s*/\s*\d+|\d+(?:[.,]\d+)?)`;
const RE_QTY = new RegExp(String.raw`^(${NUM})(?:\s*(?:-|–|—|to|or)\s*(${NUM})(?![\d/]))?`, 'i');
const RE_PREFIX = /^(about|approx\.?|approximately|roughly|around|scant|heaping|heaped|generous|rounded|level)\s+/i;
// "1 heaping tablespoon", "1 packed cup": how the measure is filled, between the number and the unit.
const RE_FILL = /^\s*((?:lightly |firmly |tightly |loosely |slightly )?(?:heaping|heaped|scant|generous|rounded|level|packed))\s+/i;
const RE_ALT = new RegExp(String.raw`^\s*(?:\(|/|,?\s*or\s+)\s*(?:about\s+|approx\.?\s+|~\s*)?(${NUM})(?:\s*(?:-|–|to)\s*(${NUM}))?\s*-?\s*([a-zA-Z.]+(?:\s+oz\.?)?)\s*\)?`, 'i');
// After a counted unit only a closed bracket is the author's weight: "1 stick (113 g) butter".
const RE_ALT_CLOSED = new RegExp(String.raw`^\s*\(\s*(?:about\s+|approx\.?\s+|~\s*)?(${NUM})\s*-?\s*([a-zA-Z.]+)\s*\)`, 'i');
const RE_SIZE = new RegExp(String.raw`^\s*(\(\s*(?:about\s+)?${NUM}(?:\s*(?:-|–|to)\s*${NUM})?\s*-?\s*[a-zA-Z.]+(?:\s+oz\.?)?\s*\)|${NUM}\s*-?\s*(?:ounces?|oz\.?|grams?|g|ml|lbs?|pounds?|kg|litres?|liters?|l)\b)\s*`, 'i');
// "1 cup plus 2 tablespoons", "1 cup minus 1 tbsp"
const RE_MORE = new RegExp(String.raw`^\s*(plus|\+|and|minus|less)\s+(${NUM})\s*`, 'i');
// "juice of 1 lemon", "zest of 2 limes"
const RE_YIELD = new RegExp(String.raw`^(?:the\s+)?(juice|zest)\s+(?:of|from)\s+(${NUM}|an?|one|half an?)\s+(?:(small|medium|large)\s+)?([a-z]+?)s?\b(.*)$`, 'i');

export function toNumber(s: string): number {
  s = s.trim().replace(',', '.');
  const mixed = s.match(/^(\d+)\s+(\d+)\s*\/\s*(\d+)$/);
  if (mixed) return +mixed[1] + +mixed[2] / +mixed[3];
  const frac = s.match(/^(\d+)\s*\/\s*(\d+)$/);
  if (frac) return +frac[2] ? +frac[1] / +frac[2] : 0;
  return parseFloat(s) || 0;
}

function normalize(text: string): string {
  let t = text.replace(/[\u00a0\u2009\u202f]/g, ' ').replace(/\u2044/g, '/').trim();
  t = t.replace(/(\d)?([½⅓⅔¼¾⅕⅖⅗⅘⅙⅚⅛⅜⅝⅞])/g, (_, d, f) => (d ? d + ' ' : '') + VULGAR[f]);
  return t.replace(/\s+/g, ' ');
}

function matchUnit(s: string): { unit: Unit; len: number } | null {
  for (const [name, unit] of UNIT_NAMES) {
    if (s.length < name.length) continue;
    const head = s.slice(0, name.length);
    // Single letters are case-sensitive: "T" is a tablespoon, "t" a teaspoon.
    const same = name.length === 1 ? (name === 'T' || name === 't' ? head === name : head.toLowerCase() === name) : head.toLowerCase() === name.toLowerCase();
    if (!same) continue;
    const next = s[name.length];
    if (next && /[a-zA-Z]/.test(next)) continue;
    let len = name.length;
    if (s[len] === '.') len++;
    return { unit, len };
  }
  return null;
}

export interface Parsed {
  raw: string;
  prefix: string;
  qty: number | null;
  qtyMax: number | null;
  unit: Unit | null;
  /** Container size kept verbatim, e.g. "(14.5-ounce)". */
  size: string;
  /** The author's own alternate measure, e.g. the "300 g" in "2 cups (300 g)". */
  alt: { qty: number; qtyMax: number | null; unit: Unit } | null;
  /** "plus 2 tablespoons": further amounts of the same ingredient. */
  more: { qty: number; unit: Unit }[];
  /** "packed", "heaping": how the measure is filled, when written before the unit. */
  fill: string;
  /** What is being counted in "juice of 1 lemon": "lemon". */
  per: string;
  /** Everything after the measure: the ingredient name and any notes. */
  rest: string;
  /** The measure exactly as written, for display when nothing changes. */
  head: string;
}

export function parseIngredient(text: string): Parsed {
  const raw = normalize(text);
  const p: Parsed = { raw, prefix: '', qty: null, qtyMax: null, unit: null, size: '', alt: null, more: [], fill: '', per: '', rest: raw, head: '' };
  let s = raw;
  const y = s.match(RE_YIELD);
  if (y) {
    // "juice of 1 lemon" is one lemon's worth of lemon juice.
    const n = /^(an?|one)$/i.test(y[2]) ? 1 : /^half/i.test(y[2]) ? 0.5 : toNumber(y[2]);
    p.qty = n;
    p.per = y[4].toLowerCase();
    p.head = '';
    p.rest = `${p.per} ${y[1].toLowerCase()}${y[5]}`;
    return p;
  }
  const pre = s.match(RE_PREFIX);
  if (pre) {
    p.prefix = pre[0];
    s = s.slice(pre[0].length);
  }
  let m = s.match(RE_QTY);
  if (!m) {
    const art = s.match(/^(an?)\s+(?=\S)/i);
    const unit = art ? matchUnit(s.slice(art[0].length)) : null;
    if (!art || !unit) return { ...p, prefix: '' };
    // "a pinch of salt": an implicit quantity of one.
    p.qty = 1;
    s = s.slice(art[0].length);
    m = null;
  } else {
    p.qty = toNumber(m[1]);
    p.qtyMax = m[2] ? toNumber(m[2]) : null;
    s = s.slice(m[0].length);
    // "2 x 400g tins"
    s = s.replace(/^\s*x\s+(?=\d)/i, ' ');
  }
  const fill = s.match(RE_FILL);
  if (fill && matchUnit(s.slice(fill[0].length))) {
    p.fill = fill[1].toLowerCase();
    s = ' ' + s.slice(fill[0].length);
  }
  const afterQty = s;
  const size = s.match(RE_SIZE);
  if (size) {
    const tail = s.slice(size[0].length);
    const cu = matchUnit(tail);
    if (cu && cu.unit.container) {
      p.size = size[1].trim();
      s = ' ' + tail;
    }
  }
  const um = matchUnit(s.replace(/^[\s-]+/, ''));
  if (um) {
    const lead = s.length - s.replace(/^[\s-]+/, '').length;
    p.unit = um.unit;
    s = s.slice(lead + um.len);
  } else if (p.size) {
    s = afterQty;
    p.size = '';
  }
  if (p.unit && p.unit.kind !== 'other') {
    // "1 cup plus 2 tablespoons flour"
    for (let more = s.match(RE_MORE); more; more = s.match(RE_MORE)) {
      const mu = matchUnit(s.slice(more[0].length));
      if (!mu || mu.unit.kind !== p.unit.kind) break;
      const sign = /minus|less/i.test(more[1]) ? -1 : 1;
      p.more.push({ qty: sign * toNumber(more[2]), unit: mu.unit });
      s = s.slice(more[0].length + mu.len);
    }
    const alt = s.match(RE_ALT);
    if (alt) {
      const au = matchUnit(alt[3].trim());
      if (au && au.unit.kind !== 'other' && au.len >= alt[3].trim().replace(/\.$/, '').length && au.unit.key !== p.unit.key) {
        p.alt = { qty: toNumber(alt[1]), qtyMax: alt[2] ? toNumber(alt[2]) : null, unit: au.unit };
        s = s.slice(alt[0].length);
      }
    }
  } else if (p.unit) {
    const alt = s.match(RE_ALT_CLOSED);
    const au = alt ? matchUnit(alt[2].trim()) : null;
    if (alt && au && au.unit.kind === 'weight' && au.len >= alt[2].trim().replace(/\.$/, '').length) {
      p.alt = { qty: toNumber(alt[1]), qtyMax: null, unit: au.unit };
      s = s.slice(alt[0].length);
    }
  }
  p.head = raw.slice(p.prefix.length, raw.length - s.length).trim();
  p.rest = s.replace(/^\s*(?:of\s+)?/i, '').replace(/^[\s,;-]+/, '').trim();
  if (!m && p.qty === 1) p.head = raw.slice(0, raw.length - s.length).trim();
  return p;
}

// ---------------------------------------------------------------- weighing

export interface Weight {
  /** Grams for the line as written (the middle of a range), unscaled. */
  g: number;
  /** Grams at the top of a range. */
  gMax: number | null;
  /** True when the figure is an estimate (≈). */
  approx: boolean;
  how: 'weight' | 'author' | 'density' | 'each' | 'size';
  match: Match | null;
}

/** Units a food can carry a per-item weight for. */
const EACH_UNITS: Record<string, string> = {
  clove: 'clove', stick: 'stick', slice: 'slice', stalk: 'stalk', rib: 'stalk', sprig: 'sprig', leaf: 'leaf', head: 'head', ear: 'ear',
  bunch: 'bunch', wedge: 'wedge', strip: 'strip', packet: 'packet', envelope: 'envelope', sachet: 'sachet', package: 'packet', pkg: 'packet',
};

/** The line's match, with any per-recipe correction applied. */
export function lookup(p: Parsed, o: WeighOptions = {}): Match {
  const m = matchFood(p.rest);
  const fix = o.fixes?.[p.raw];
  if (!fix) return m;
  if (fix.skip) return { ...m, skip: true, via: 'rule' };
  if (fix.foodId) {
    const food = foodById(fix.foodId);
    if (food) return { ...m, food, rule: null, via: 'rule', approx: false, skip: false };
  }
  if (fix.gPerMl || fix.gEach) {
    return { ...m, food: null, via: 'rule', approx: false, skip: false, rule: { phrase: m.phrase, action: 'custom', gPerMl: fix.gPerMl ?? null, gEach: fix.gEach ?? null } };
  }
  return m;
}

/** Grams per millilitre for this line, or null. */
function gml(m: Match, o: WeighOptions): { gml: number; approx: boolean } | null {
  if (m.skip) return null;
  if (m.food?.t === 'kosher') {
    // Kosher salt is two different salts by volume; use the brand once it is known.
    const brand = o.salt ? foodById(o.salt.brand === 'dc' ? 'salt-dc' : 'salt-morton') : null;
    if (brand?.d) return { gml: brand.d, approx: !o.salt!.sure };
  }
  return densityOf(m, o);
}

/** Grams inside a stated container size: "(14.5-ounce)" or "400g". */
function sizeGrams(size: string): number | null {
  const m = size.match(new RegExp(String.raw`(${NUM})\s*-?\s*([a-zA-Z.]+)`));
  if (!m) return null;
  const u = matchUnit(m[2]);
  return u && u.unit.kind === 'weight' ? toNumber(m[1]) * u.unit.base : null;
}

/** Best estimate of what an ingredient line weighs, or null when it cannot be known. */
export function weigh(p: Parsed, o: WeighOptions = {}): Weight | null {
  if (p.qty == null) return null;
  const both = (per: number, plus = 0) => ({
    g: ((p.qtyMax != null ? (p.qty! + p.qtyMax) / 2 : p.qty!) * per + plus),
    gMax: p.qtyMax != null ? p.qtyMax * per + plus : null,
  });
  if (p.unit?.kind === 'weight') {
    const plus = p.more.reduce((s, x) => s + x.qty * x.unit.base, 0);
    return { ...both(p.unit.base, plus), approx: false, how: 'weight', match: null };
  }
  if (p.alt?.unit.kind === 'weight') {
    return { g: p.alt.qty * p.alt.unit.base, gMax: p.alt.qtyMax != null ? p.alt.qtyMax * p.alt.unit.base : null, approx: false, how: 'author', match: null };
  }
  if (p.unit?.container && p.size) {
    const g = sizeGrams(p.size);
    if (g != null) return { ...both(g), approx: false, how: 'size', match: null };
  }
  const m = lookup(p, o);
  if (m.skip) return null;
  if (p.unit?.kind === 'volume') {
    let state = m;
    if (/packed/.test(p.fill) && !m.state) state = { ...m, state: /loosely/.test(p.fill) ? 'loose' : 'packed' };
    const d = gml(state, o);
    if (!d) return null;
    const plus = p.more.reduce((s, x) => s + x.qty * x.unit.base, 0) * d.gml;
    return { ...both(p.unit.base * d.gml, plus), approx: d.approx, how: 'density', match: m };
  }
  // Counted: "3 large eggs", "2 cloves garlic", "1 stick butter", "juice of 1 lemon".
  let key = p.unit ? EACH_UNITS[p.unit.key] : p.per;
  if (p.unit && !key) return null;
  let food = m;
  if (!p.unit && m.food?.id === 'lemon' && /\bjuiced?\b/i.test(p.rest) && !/zest/i.test(p.rest)) {
    // "1 lemon, juiced" is the juice, not the fruit.
    const juice = foodById('lemon-juice');
    if (juice) {
      food = { ...m, food: juice };
      key = /\blimes?\b/i.test(m.name) ? 'lime' : 'lemon';
    }
  }
  const e = eachOf(food, key ?? '');
  if (!e) return null;
  // An egg, a stick of butter and a packet of yeast are standard sizes; an onion is not.
  const standard = food.food?.c === 'egg' || ['stick', 'packet', 'envelope', 'sachet'].includes(key ?? '') || food.rule?.action === 'custom';
  return { ...both(e.gml), approx: e.approx || !standard, how: 'each', match: food };
}

/** Grams for a line at a given scale, or null. Kept for callers that only need the number. */
export function grams(p: Parsed, scale = 1, o: WeighOptions = {}): number | null {
  const w = weigh(p, o);
  return w ? w.g * scale : null;
}

// ---------------------------------------------------------------- formatting

const FRACTIONS: [number, string][] = [
  [1 / 8, '⅛'], [1 / 4, '¼'], [1 / 3, '⅓'], [3 / 8, '⅜'], [1 / 2, '½'], [5 / 8, '⅝'], [2 / 3, '⅔'], [3 / 4, '¾'], [7 / 8, '⅞'],
];

function trimNum(n: number, places = 2): string {
  return String(+n.toFixed(places));
}

/** 1.5 → "1½"; falls back to decimals when no tidy fraction is close. */
export function formatNumber(n: number, fractions: boolean): string {
  if (n <= 0) return '0';
  if (!fractions) return trimNum(n, n < 10 ? 2 : 1);
  const whole = Math.floor(n + 1e-9);
  const frac = n - whole;
  if (frac < 0.03) return String(whole);
  if (frac > 0.97) return String(whole + 1);
  let best: [number, string] | null = null;
  for (const f of FRACTIONS) if (!best || Math.abs(f[0] - frac) < Math.abs(best[0] - frac)) best = f;
  if (best && Math.abs(best[0] - frac) < 0.035) return (whole ? String(whole) : '') + best[1];
  return trimNum(n, n < 10 ? 2 : 1);
}

function roundMetric(v: number): number {
  if (v >= 100) return Math.round(v / 5) * 5;
  if (v >= 10) return Math.round(v);
  if (v >= 2) return Math.round(v * 2) / 2;
  return Math.round(v * 10) / 10;
}

/** Grams as a cook would read them off a scale: finer the smaller the amount. */
export function roundGrams(v: number): number {
  if (v >= 1000) return Math.round(v / 5) * 5;
  if (v >= 10) return Math.round(v);
  if (v >= 1) return Math.round(v * 10) / 10;
  return Math.round(v * 100) / 100;
}

interface Measure {
  qty: number;
  qtyMax: number | null;
  unit: Unit;
}

function metricWeight(g: number, gMax: number | null, fine = false): { text: string; unit: string } {
  const big = g >= 1000;
  const f = (v: number) => (big ? trimNum(Math.round(v / 10) / 100, 2) : trimNum(fine ? roundGrams(v) : roundMetric(v), 2));
  return { text: gMax != null ? `${f(g)}–${f(gMax)}` : f(g), unit: big ? 'kg' : 'g' };
}

function metricVolume(ml: number, mlMax: number | null): { text: string; unit: string } {
  const big = ml >= 1000;
  const f = (v: number) => (big ? trimNum(Math.round(v / 10) / 100, 2) : trimNum(roundMetric(v), 1));
  return { text: mlMax != null ? `${f(ml)}–${f(mlMax)}` : f(ml), unit: big ? 'l' : 'ml' };
}

/** Picks the US spoon/cup measure a cook would actually reach for. */
function usVolume(ml: number, mlMax: number | null, fractions: boolean): { text: string; unit: string } {
  // Try each measure and keep the largest one that lands within 2.5% of the
  // true amount; otherwise whichever is closest. So 66 ml is 4½ tbsp, not ¼ cup.
  const pick = (v: number): [number, Unit] => {
    const cands: [number, Unit][] = [];
    if (v >= 50) {
      const cups = v / 236.588;
      const whole = Math.floor(cups);
      let best = 0;
      for (const s of [0, 0.25, 1 / 3, 0.5, 2 / 3, 0.75, 1]) if (Math.abs(cups - whole - s) < Math.abs(cups - whole - best)) best = s;
      if (whole + best > 0) cands.push([whole + best, UNIT_BY_KEY.get('cup')!]);
    }
    if (v >= 14 && v <= 125) cands.push([Math.max(Math.round((v / 14.7868) * 2) / 2, 1), UNIT_BY_KEY.get('tbsp')!]);
    if (v < 30) cands.push([Math.max(Math.round((v / 4.92892) * 8) / 8, 0.125), UNIT_BY_KEY.get('tsp')!]);
    const err = (c: [number, Unit]) => Math.abs(c[0] * c[1].base - v) / v;
    return cands.find((c) => err(c) <= 0.025) ?? cands.reduce((a, c) => (err(c) < err(a) ? c : a));
  };
  const [q, unit] = pick(ml);
  let text = formatNumber(q, fractions);
  let many = q > 1;
  if (mlMax != null) {
    const qm = mlMax / unit.base;
    text += '–' + formatNumber(unit.key === 'cup' ? Math.round(qm * 4) / 4 : Math.round(qm * 2) / 2, fractions);
    many = true;
  }
  return { text, unit: many ? unit.many : unit.one };
}

function usWeight(g: number, gMax: number | null, fractions: boolean): { text: string; unit: string } {
  const oz = g / 28.3495;
  if (oz >= 16) {
    const f = (v: number) => formatNumber(Math.round((v / 453.592) * 4) / 4, fractions);
    return { text: gMax != null ? `${f(g)}–${f(gMax)}` : f(g), unit: 'lb' };
  }
  const f = (v: number) => {
    const o = v / 28.3495;
    return formatNumber(Math.max(Math.round(o * 4) / 4, 0.25), fractions);
  };
  return { text: gMax != null ? `${f(g)}–${f(gMax)}` : f(g), unit: 'oz' };
}

const PRETTY: Record<string, string> = { '1/2': '½', '1/3': '⅓', '2/3': '⅔', '1/4': '¼', '3/4': '¾', '1/8': '⅛', '3/8': '⅜', '5/8': '⅝', '7/8': '⅞' };
/** "1 1/2 cups" → "1½ cups": typeset the fractions without changing the words. */
function pretty(s: string): string {
  return s.replace(/(\d+ )?(\d)\/(\d)(?![\d/])/g, (m, whole, a, b) => (PRETTY[`${a}/${b}`] ? (whole ? whole.trim() : '') + PRETTY[`${a}/${b}`] : m));
}

const PLURALS: Record<string, string> = {
  egg: 'eggs', onion: 'onions', lemon: 'lemons', lime: 'limes', carrot: 'carrots', shallot: 'shallots', apple: 'apples', banana: 'bananas',
  tomato: 'tomatoes', potato: 'potatoes', orange: 'oranges', avocado: 'avocados', cucumber: 'cucumbers', scallion: 'scallions',
};

const SINGULARS: Record<string, string> = Object.fromEntries(Object.entries(PLURALS).map(([one, many]) => [many, one]));
/** "large egg" ↔ "large eggs": fixes the noun's number when scaling crosses one. */
function renumber(rest: string, table: Record<string, string>): string {
  const name = rest.split(/[,(]/)[0];
  const word = name.split(/\s+/).slice(0, 3).find((w) => table[w.toLowerCase()]);
  return word ? rest.replace(word, table[word.toLowerCase()]) : rest;
}

export interface Shown {
  /** Quantity and unit, e.g. "1½ cups". Empty when the line has no measure. */
  measure: string;
  /** Ingredient name and notes. */
  rest: string;
  /** True when the measure was converted from what the author wrote. */
  converted: boolean;
  /** The original measure, set when converted. */
  from: string;
  /** True when the conversion went between volume and weight. */
  estimated: boolean;
  /** True when that weight is a rough figure and should carry "≈". */
  approx: boolean;
  /** A weight to show beside a count: "3 large eggs" + "150 g". */
  also: string;
  /** The line has an amount a scale could measure. */
  measurable: boolean;
  /** ...and Fond knows what it weighs. */
  weighed: boolean;
  /** What it weighs at this scale, in grams. */
  grams: number | null;
  /** Where the weight comes from, for the tooltip. */
  basis: string;
  match: Match | null;
}

function basisFor(p: Parsed, w: Weight, o: WeighOptions): string {
  const food = w.match?.food;
  if (w.how === 'author') return 'the author’s own weight';
  if (w.how === 'size') return 'the size stated on the container';
  if (w.match?.rule?.action === 'custom') return 'your own measurement';
  if (!food) return '';
  if (w.how === 'each') return `${food.n}, typical weight (USDA)`;
  const spoon = p.unit?.key === 'tsp' || p.unit?.key === 'tbsp';
  const per = spoon ? p.unit! : UNIT_BY_KEY.get('cup')!;
  if (food.t === 'kosher') {
    const brand = o.salt ? foodById(o.salt.brand === 'dc' ? 'salt-dc' : 'salt-morton') : null;
    if (!brand?.d) return 'kosher salt, brand unknown: halfway between Diamond Crystal (2.8 g a teaspoon) and Morton (4.8 g)';
    return `kosher salt, ${o.salt!.sure ? 'set to' : 'read as'} ${o.salt!.brand === 'dc' ? 'Diamond Crystal' : 'Morton'} · ${+(brand.d * UNIT_BY_KEY.get('tsp')!.base).toFixed(1)} g per tsp`;
  }
  if (o.flour === 'dip' && food.x?.dip) return `${food.n}, dipped and swept${o.house ? `, as ${o.house} measures` : ''} · ${Math.round(food.x.dip * per.base)} g per ${per.one}`;
  if (o.house && food.f && food.x?.dip) return `${food.n}, spooned and levelled, as ${o.house} measures · ${Math.round(food.d! * per.base)} g per ${per.one}`;
  const state = w.match?.state && w.match.state !== food.ds ? `, ${w.match.state}` : '';
  const said = state || food.id === 'salt-kosher' ? '' : basisText(food, per.base);
  return `${food.n}${state}${said ? ` · ${said} g per ${per.one}` : ''}`;
}

/** Renders a parsed ingredient for the reader's scale and unit preferences. */
export function show(p: Parsed, o: DisplayOptions): Shown {
  const out: Shown = {
    measure: '', rest: p.raw, converted: false, from: '', estimated: false, approx: false, also: '',
    measurable: false, weighed: false, grams: null, basis: '', match: null,
  };
  if (p.qty == null) return out;
  const scaled = Math.abs(o.scale - 1) > 1e-6;
  out.rest = p.rest;
  const lead = p.prefix;
  const unit = p.unit;
  const w = weigh(p, o);
  const m = w?.match ?? (unit?.kind === 'weight' ? null : lookup(p, o));
  out.match = m;
  if (w) {
    out.grams = w.g * o.scale;
    out.basis = basisFor(p, w, o);
  }
  const byVolume = unit?.kind === 'volume';
  const dens = byVolume && w?.how === 'density' ? w.g / volumeOf(p) : null;
  out.measurable = !(m?.skip) && (unit?.kind === 'weight' || byVolume || !!w);
  out.weighed = out.measurable && !!w;

  if (p.per) {
    // "juice of 1 lemon" reads best as written.
    const n = p.qty * o.scale;
    out.measure = '';
    out.rest = scaled ? `Juice of ${formatNumber(n, true)} ${p.per}${n > 1 ? 's' : ''}`.replace(/^Juice/, /zest/.test(p.rest) ? 'Zest' : 'Juice') : p.raw;
    if (o.weigh && w) out.also = alsoText(w, o);
    out.approx = !!w?.approx;
    return out;
  }

  // What do we want to end up with?
  let target: 'keep' | 'metric-w' | 'metric-v' | 'us-w' | 'us-v' = 'keep';
  const spoon = unit?.key === 'tsp' || unit?.key === 'tbsp';
  const canWeigh = byVolume && !!w && (w.how === 'density' || w.how === 'author');
  if (unit && unit.kind !== 'other') {
    if (o.system === 'metric') {
      if (unit.kind === 'weight') target = unit.system === 'us' ? 'metric-w' : 'keep';
      else if (o.weigh && canWeigh) target = 'metric-w';
      else target = unit.system === 'us' && !spoon ? 'metric-v' : 'keep';
    } else if (o.system === 'us') {
      // Ounces are too coarse for spoonfuls, so those stay as spoons.
      const ownOunces = p.alt?.unit.kind === 'weight' && p.alt.unit.system === 'us';
      if (unit.kind === 'weight') target = unit.system === 'metric' ? (volumeDensity(p, o) && !o.weigh && !ownOunces ? 'us-v' : 'us-w') : 'keep';
      else if (o.weigh && canWeigh && !spoon) target = 'us-w';
      else target = unit.system === 'metric' ? 'us-v' : 'keep';
    } else if (o.weigh && canWeigh) {
      target = 'metric-w';
    }
  } else if (unit?.key === 'stick' && o.weigh && w) {
    // A stick of butter is a measure, not a count.
    const r = o.system === 'us' ? usWeight(w.g * o.scale, null, o.fractions) : metricWeight(w.g * o.scale, null, true);
    out.measure = `${lead}${r.text} ${r.unit}`;
    out.converted = true;
    out.estimated = true;
    out.from = p.head;
    return out;
  }

  const wantKind = target.endsWith('-w') ? 'weight' : 'volume';
  const wantSystem = target.startsWith('us') ? 'us' : 'metric';
  let src: Measure = { qty: p.qty, qtyMax: p.qtyMax, unit: unit as Unit };
  let usedAlt = false;
  if (target !== 'keep' && p.alt && p.alt.unit.kind === wantKind && (p.alt.unit.system === wantSystem || wantKind === 'weight')) {
    // The author already gave this measure — trust it over arithmetic.
    src = p.alt;
    usedAlt = true;
  }

  if (target === 'keep') {
    if (o.weigh && w && w.how === 'each') {
      out.also = alsoText(w, o);
      out.approx = w.approx;
    }
    if (!scaled) {
      out.measure = (lead + p.head).trim();
      if (o.fractions) out.measure = pretty(out.measure);
      return out;
    }
    const q = p.qty * o.scale;
    const qm = p.qtyMax != null ? p.qtyMax * o.scale : null;
    let text: string;
    let unitText = '';
    if (unit && unit.kind !== 'other' && unit.system === 'metric') {
      const more = p.more.reduce((s, x) => s + x.qty * x.unit.base, 0) * o.scale;
      // A weight the recipe gives in grams stays exact; only conversions are rounded to the nearest five.
      const r = unit.kind === 'weight' ? metricWeight(q * unit.base + more, qm != null ? qm * unit.base + more : null, true) : metricVolume(q * unit.base + more, qm != null ? qm * unit.base + more : null);
      [text, unitText] = [r.text, r.unit];
    } else if (unit && unit.kind === 'volume') {
      const more = p.more.reduce((s, x) => s + x.qty * x.unit.base, 0) * o.scale;
      const r = usVolume(q * unit.base + more, qm != null ? qm * unit.base + more : null, o.fractions);
      // Keep big-batch units (pints, quarts) as written rather than recasting to cups.
      if ((['pint', 'quart', 'gallon', 'floz'].includes(unit.key) || qm != null) && !more) {
        text = formatNumber(q, o.fractions) + (qm != null ? '–' + formatNumber(qm, o.fractions) : '');
        unitText = q > 1 || qm != null ? unit.many : unit.one;
      } else [text, unitText] = [r.text, r.unit];
    } else if (unit && unit.kind === 'weight') {
      const more = p.more.reduce((s, x) => s + x.qty * x.unit.base, 0) * o.scale;
      const r = usWeight(q * unit.base + more, qm != null ? qm * unit.base + more : null, o.fractions);
      [text, unitText] = [r.text, r.unit];
    } else {
      const f = (v: number) => formatNumber(v, true);
      text = f(q) + (qm != null ? '–' + f(qm) : '');
      if (unit) unitText = q > 1 || qm != null ? unit.many : unit.one;
      else if (p.qty <= 1 && q > 1) out.rest = renumber(out.rest, PLURALS);
      else if (p.qty > 1 && q <= 1 && qm == null) out.rest = renumber(out.rest, SINGULARS);
    }
    const size = p.size ? ' ' + p.size : '';
    out.measure = `${lead}${text}${size}${unitText ? ' ' + unitText : ''}`.trim();
    if (p.alt) {
      // Scale the author's alternate too, so the pair stays consistent.
      const a = p.alt;
      const aq = a.qty * o.scale;
      const am = a.qtyMax != null ? a.qtyMax * o.scale : null;
      const r = a.unit.kind === 'weight'
        ? a.unit.system === 'metric' ? metricWeight(aq * a.unit.base, am != null ? am * a.unit.base : null, true) : usWeight(aq * a.unit.base, am != null ? am * a.unit.base : null, o.fractions)
        : a.unit.system === 'metric' ? metricVolume(aq * a.unit.base, am != null ? am * a.unit.base : null) : usVolume(aq * a.unit.base, am != null ? am * a.unit.base : null, o.fractions);
      out.measure += ` (${r.text} ${r.unit})`;
    }
    return out;
  }

  // Convert.
  const more = usedAlt ? 0 : p.more.reduce((s, x) => s + x.qty * x.unit.base, 0);
  let baseQty = (src.qty * src.unit.base + more) * o.scale;
  let baseMax = src.qtyMax != null ? (src.qtyMax * src.unit.base + more) * o.scale : null;
  if (src.unit.kind !== wantKind) {
    // volume <-> weight through the dictionary
    const d = wantKind === 'weight' ? (dens as number) : (volumeDensity(p, o) as number);
    const k = wantKind === 'weight' ? d : 1 / d;
    baseQty *= k;
    if (baseMax != null) baseMax *= k;
    out.estimated = true;
    out.approx = wantKind === 'weight' && !!w?.approx;
  }
  const r =
    target === 'metric-w' ? metricWeight(baseQty, baseMax, out.estimated || (src.unit.kind === 'weight' && src.unit.system === 'metric'))
    : target === 'metric-v' ? metricVolume(baseQty, baseMax)
    : target === 'us-w' ? usWeight(baseQty, baseMax, o.fractions)
    : usVolume(baseQty, baseMax, o.fractions);
  out.measure = `${lead}${r.text} ${r.unit}`;
  out.converted = !usedAlt;
  out.from = usedAlt ? '' : p.head;
  return out;
}

/** How much a line measures out: millilitres, or a count of items. For "this much weighs…" */
export function amountOf(p: Parsed): { ml: number } | { count: number } | null {
  if (p.qty == null) return null;
  if (p.unit?.kind === 'volume') return { ml: volumeOf(p) };
  if (!p.unit || p.per) return { count: p.qtyMax != null ? (p.qty + p.qtyMax) / 2 : p.qty };
  return null;
}

/** Millilitres in a volume line as written, including "plus 2 tablespoons". */
function volumeOf(p: Parsed): number {
  const q = p.qtyMax != null ? (p.qty! + p.qtyMax) / 2 : p.qty!;
  return q * p.unit!.base + p.more.reduce((s, x) => s + x.qty * x.unit.base, 0);
}

/** Grams per millilitre for a line measured by weight, so it can be shown in cups. */
function volumeDensity(p: Parsed, o: WeighOptions): number | null {
  const m = lookup(p, o);
  // Only everyday staples read better in cups; a kilo of onions does not.
  if (m.skip || !m.food || m.food.ds || m.approx || m.via === 'head' || m.via === 'typo') return null;
  return gml(m, o)?.gml ?? null;
}

function alsoText(w: Weight, o: DisplayOptions): string {
  const r = o.system === 'us' ? usWeight(w.g * o.scale, null, o.fractions) : metricWeight(w.g * o.scale, null, true);
  return `${r.text} ${r.unit}`;
}

/** Rewrites a line so that it carries its weight: "1 cup flour" → "1 cup (120 g) flour". */
export function withWeight(p: Parsed, o: WeighOptions = {}): string | null {
  if (p.qty == null || !p.unit || p.unit.kind !== 'volume' || p.alt || p.per) return null;
  const w = weigh(p, o);
  if (!w || w.how !== 'density' || w.approx) return null;
  const r = metricWeight(w.g, w.gMax, true);
  const at = p.raw.indexOf(p.head) + p.head.length;
  if (!p.head || at < p.head.length) return null;
  return `${p.raw.slice(0, at)} (${r.text} ${r.unit})${p.raw.slice(at)}`;
}

// ---------------------------------------------------------------- baker's maths

const NOT_FLOUR = /tortilla|for dusting|for the (work )?surface|for rolling|for kneading|to dust/i;
// Used only for ingredients the dictionary does not know.
const WATER_SHARE: [RegExp, number][] = [
  [/starter|levain|poolish|biga/, 0.5], [/egg yolk/, 0.5], [/egg white/, 0.88], [/eggs?\b/, 0.75], [/buttermilk/, 0.9], [/butter/, 0.16],
  [/cream cheese/, 0.5], [/(heavy|double|whipping) cream/, 0.58], [/sour cream/, 0.72], [/yogh?urt|kefir/, 0.85], [/milk/, 0.87],
  [/honey|syrup|molasses/, 0.18], [/oil|shortening|lard/, 0], [/water|beer|coffee|juice|stock|broth|wine/, 1],
];
/** Categories whose water does not count towards a dough's hydration. */
const DRY = new Set(['flour', 'starch', 'sugar', 'salt', 'leaven', 'spice', 'herb', 'nut', 'seed', 'grain', 'baking', 'bread', 'misc']);

export function looksLikeFlour(name: string): boolean {
  return /flour|semolina|\bmeal\b/i.test(name) && !NOT_FLOUR.test(name);
}

export interface BakerRow {
  grams: number | null;
  flour: boolean;
  percent: number | null;
  /** The line has an amount but Fond cannot weigh it, so the totals leave it out. */
  missing: boolean;
  salt: boolean;
}

export interface BakerOptions extends WeighOptions {
  /** Per-line corrections to the automatic flour detection, keyed by the line as written. */
  flourLines?: Record<string, boolean>;
  /** Hydration of a starter or preferment, in percent of its flour. Default 100. */
  starter?: number;
}

export interface Baker {
  rows: BakerRow[];
  /** Total flour in grams, including the flour inside a starter. */
  flour: number;
  total: number;
  hydration: number | null;
  /** Salt in grams and as a share of flour. */
  salt: number;
  saltPercent: number | null;
  /** Every flour line could be weighed, so the percentages can be trusted. */
  complete: boolean;
  /** Names of lines the totals had to leave out. */
  missing: string[];
  hasStarter: boolean;
}

/**
 * Baker's percentages: every ingredient as a share of total flour weight.
 * A starter counts partly as flour and partly as water, by its hydration.
 */
export function bakers(items: Parsed[], opts: BakerOptions | Record<string, boolean> = {}): Baker {
  // Older callers pass the flour overrides directly.
  const o: BakerOptions = 'flourLines' in opts || 'fixes' in opts || 'flour' in opts || 'salt' in opts || 'starter' in opts ? (opts as BakerOptions) : { flourLines: opts as Record<string, boolean> };
  const h = Math.max(0, o.starter ?? 100) / 100;
  let flourTotal = 0;
  let water = 0;
  let total = 0;
  let salt = 0;
  let complete = true;
  let hasStarter = false;
  const missing: string[] = [];
  const rows: BakerRow[] = items.map((p) => {
    const w = weigh(p, o);
    const m = w?.match ?? lookup(p, o);
    const food: Food | null = m.food;
    const name = p.rest.toLowerCase();
    const auto = food ? !!food.f && !m.skip : looksLikeFlour(p.rest);
    const flour = o.flourLines?.[p.raw] ?? auto;
    const isSalt = food?.c === 'salt';
    const g = w ? w.g : null;
    const wants = p.qty != null && !m.skip && (p.unit?.kind === 'volume' || p.unit?.kind === 'weight' || (!p.unit && !!food));
    const row: BakerRow = { grams: g, flour, percent: null, missing: g == null && wants, salt: isSalt };
    if (g == null) {
      if (flour) complete = false;
      if (row.missing) missing.push(m.name || p.rest);
      return row;
    }
    total += g;
    const starter = food ? food.id === 'starter' : /starter|levain|poolish|biga/.test(name);
    if (isSalt) salt += g;
    if (flour) flourTotal += g;
    else if (starter) {
      hasStarter = true;
      flourTotal += g / (1 + h);
      water += (g * h) / (1 + h);
      return row;
    }
    if (food) {
      if (food.w != null && !DRY.has(food.c) && !flour) water += g * food.w;
    } else {
      for (const [re, share] of WATER_SHARE) {
        if (re.test(name)) {
          water += g * share;
          break;
        }
      }
    }
    return row;
  });
  if (flourTotal > 0) rows.forEach((r) => (r.percent = r.grams != null ? (r.grams / flourTotal) * 100 : null));
  return {
    rows,
    flour: flourTotal,
    total,
    hydration: flourTotal > 0 ? (water / flourTotal) * 100 : null,
    salt,
    saltPercent: flourTotal > 0 && salt > 0 ? (salt / flourTotal) * 100 : null,
    complete,
    missing,
    hasStarter,
  };
}
