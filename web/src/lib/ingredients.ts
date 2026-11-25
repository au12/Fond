// Ingredient understanding. Recipes store ingredients as plain text; this
// module reads a line like "2 1/2 cups (300 g) flour, sifted" and can scale
// it, convert it between measurement systems, and weigh it for baker's maths.

export type System = 'original' | 'metric' | 'us';

export interface DisplayOptions {
  scale: number;
  system: System;
  /** Prefer weights over volumes when a standard density is known. */
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
const RE_ALT = new RegExp(String.raw`^\s*(?:\(|/|,?\s*or\s+)\s*(?:about\s+|approx\.?\s+|~\s*)?(${NUM})(?:\s*(?:-|–|to)\s*(${NUM}))?\s*-?\s*([a-zA-Z.]+(?:\s+oz\.?)?)\s*\)?`, 'i');
const RE_SIZE = new RegExp(String.raw`^\s*(\(\s*(?:about\s+)?${NUM}(?:\s*(?:-|–|to)\s*${NUM})?\s*-?\s*[a-zA-Z.]+(?:\s+oz\.?)?\s*\)|${NUM}\s*-?\s*(?:ounces?|oz\.?|grams?|g|ml|lbs?|pounds?|kg|litres?|liters?|l)\b)\s*`, 'i');

export function toNumber(s: string): number {
  s = s.trim().replace(',', '.');
  const mixed = s.match(/^(\d+)\s+(\d+)\s*\/\s*(\d+)$/);
  if (mixed) return +mixed[1] + +mixed[2] / +mixed[3];
  const frac = s.match(/^(\d+)\s*\/\s*(\d+)$/);
  if (frac) return +frac[2] ? +frac[1] / +frac[2] : 0;
  return parseFloat(s) || 0;
}

function normalize(text: string): string {
  let t = text.replace(/[   ]/g, ' ').replace(/⁄/g, '/').trim();
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
  /** Everything after the measure: the ingredient name and any notes. */
  rest: string;
  /** The measure exactly as written, for display when nothing changes. */
  head: string;
}

export function parseIngredient(text: string): Parsed {
  const raw = normalize(text);
  const p: Parsed = { raw, prefix: '', qty: null, qtyMax: null, unit: null, size: '', alt: null, rest: raw, head: '' };
  let s = raw;
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
    const alt = s.match(RE_ALT);
    if (alt) {
      const au = matchUnit(alt[3].trim());
      if (au && au.unit.kind !== 'other' && au.len >= alt[3].trim().replace(/\.$/, '').length && au.unit.key !== p.unit.key) {
        p.alt = { qty: toNumber(alt[1]), qtyMax: alt[2] ? toNumber(alt[2]) : null, unit: au.unit };
        s = s.slice(alt[0].length);
      }
    }
  }
  p.head = raw.slice(p.prefix.length, raw.length - s.length).trim();
  p.rest = s.replace(/^\s*(?:of\s+)?/i, '').replace(/^[\s,;-]+/, '').trim();
  if (!m && p.qty === 1) p.head = raw.slice(0, raw.length - s.length).trim();
  return p;
}

// ---------------------------------------------------------------- densities

/** Grams per US cup, matched against the ingredient name (first match wins). */
const DENSITY: [RegExp, number][] = [
  [/almond (flour|meal)/, 96], [/coconut flour/, 112], [/(whole[- ]?wheat|wholemeal|whole grain) flour/, 113], [/(rye|pumpernickel) flour/, 106],
  [/(pastry) flour/, 106], [/spelt flour/, 99], [/oat flour/, 92], [/(chickpea|gram|besan) flour/, 85], [/(rice) flour/, 142],
  [/(tapioca) (flour|starch)/, 113], [/potato starch/, 152], [/semolina/, 163], [/corn ?meal|polenta/, 138],
  [/corn ?starch|cornflour|corn flour/, 112], [/flour/, 120],
  [/(powdered|confectioners'?|icing) sugar/, 113], [/brown sugar|muscovado/, 213], [/coconut sugar/, 150], [/sugar/, 198],
  [/honey/, 336], [/maple syrup/, 312], [/molasses|treacle/, 340], [/(corn|golden|agave) syrup|agave/, 312],
  [/peanut butter|almond butter|tahini|nut butter/, 270], [/cocoa butter/, 218], [/buttermilk/, 240], [/butter/, 227],
  [/shortening|lard/, 190], [/coconut oil/, 216], [/oil/, 216],
  [/(condensed) milk/, 306], [/(evaporated) milk/, 252], [/coconut milk|coconut cream/, 240], [/milk/, 240],
  [/(heavy|double|whipping|single|light) cream|half[- ]and[- ]half/, 232], [/sour cream|cr[eè]me fra[iî]che/, 230], [/cream cheese|mascarpone|ricotta/, 227],
  [/yogh?urt|kefir/, 230], [/water|stock|broth|wine|beer|juice|vinegar|coffee/, 237],
  [/cocoa/, 84], [/chocolate chips|chocolate chunks/, 170], [/(rolled|old[- ]fashioned|quick) oats|oats|oatmeal/, 89],
  [/panko/, 50], [/bread ?crumbs/, 112], [/parmesan|pecorino/, 100], [/(shredded|grated) (cheddar|mozzarella|cheese)|cheddar|mozzarella|gruy[eè]re/, 113],
  [/raisins|sultanas|currants|dried cranberries/, 149], [/walnuts|pecans/, 113], [/sliced almonds|flaked almonds/, 86], [/almonds|hazelnuts|peanuts|cashews|pistachios/, 142],
  [/(shredded|desiccated) coconut/, 85], [/sesame seeds/, 142], [/chia seeds/, 170], [/flax/, 150],
  [/(basmati|jasmine|long[- ]grain|white|brown|arborio|sushi) rice|rice/, 190], [/quinoa/, 177], [/couscous|bulgur/, 160], [/lentils/, 192],
  [/baking powder/, 192], [/baking soda|bicarbonate/, 288], [/yeast/, 149], [/kosher salt/, 190], [/salt/, 288],
  [/(sourdough )?starter|levain/, 240], [/pumpkin pur[eé]e|apple ?sauce|mashed banana/, 240], [/tomato paste|tomato pur[eé]e/, 262],
  [/mayonnaise/, 220], [/ketchup/, 272], [/soy sauce|tamari/, 255], [/jam|jelly|preserves|marmalade/, 320], [/vanilla/, 208],
];

/** Typical weight in grams of one unitless item. */
const EACH: [RegExp, number][] = [[/egg yolk/, 18], [/egg white/, 33], [/eggs?\b/, 50]];

export function density(name: string): number | null {
  const n = name.toLowerCase();
  for (const [re, g] of DENSITY) if (re.test(n)) return g;
  return null;
}

/** Best estimate of an ingredient's weight in grams, or null. */
export function grams(p: Parsed, scale = 1): number | null {
  if (p.qty == null) return null;
  const q = (p.qtyMax != null ? (p.qty + p.qtyMax) / 2 : p.qty) * scale;
  if (p.unit?.kind === 'weight') return q * p.unit.base;
  if (p.alt?.unit.kind === 'weight') return p.alt.qty * p.alt.unit.base * scale;
  if (p.unit?.kind === 'volume') {
    const d = density(p.rest);
    return d ? ((q * p.unit.base) / 236.588) * d : null;
  }
  if (!p.unit) {
    const n = p.rest.toLowerCase();
    for (const [re, g] of EACH) if (re.test(n)) return q * g;
  }
  return null;
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

interface Measure {
  qty: number;
  qtyMax: number | null;
  unit: Unit;
}

function metricWeight(g: number, gMax: number | null): { text: string; unit: string } {
  const big = g >= 1000;
  const f = (v: number) => (big ? trimNum(Math.round(v / 10) / 100, 2) : trimNum(roundMetric(v), 1));
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
    return formatNumber(o < 4 ? Math.max(Math.round(o * 4) / 4, 0.25) : Math.round(o * 2) / 2, fractions);
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

export interface Shown {
  /** Quantity and unit, e.g. "1½ cups". Empty when the line has no measure. */
  measure: string;
  /** Ingredient name and notes. */
  rest: string;
  /** True when the measure was converted from what the author wrote. */
  converted: boolean;
  /** The original measure, set when converted. */
  from: string;
  /** True when the conversion used a standard density (an estimate). */
  estimated: boolean;
}

/** Renders a parsed ingredient for the reader's scale and unit preferences. */
export function show(p: Parsed, o: DisplayOptions): Shown {
  const out: Shown = { measure: '', rest: p.raw, converted: false, from: '', estimated: false };
  if (p.qty == null) return out;
  const scaled = Math.abs(o.scale - 1) > 1e-6;
  out.rest = p.rest;
  const lead = p.prefix;

  // What do we want to end up with?
  let target: 'keep' | 'metric-w' | 'metric-v' | 'us-w' | 'us-v' = 'keep';
  const unit = p.unit;
  const dens = unit && unit.kind !== 'other' ? density(p.rest) : null;
  if (unit && unit.kind !== 'other') {
    const spoon = unit.key === 'tsp' || unit.key === 'tbsp';
    if (o.system === 'metric') {
      if (unit.kind === 'weight') target = unit.system === 'us' ? 'metric-w' : 'keep';
      else if (o.weigh && dens && !spoon) target = 'metric-w';
      else target = unit.system === 'us' && !spoon ? 'metric-v' : 'keep';
    } else if (o.system === 'us') {
      if (unit.kind === 'weight') target = unit.system === 'metric' ? (dens && !o.weigh ? 'us-v' : 'us-w') : 'keep';
      else if (o.weigh && dens && !spoon) target = 'us-w';
      else target = unit.system === 'metric' ? 'us-v' : 'keep';
    } else if (o.weigh && unit.kind === 'volume' && dens && !spoon) {
      target = 'metric-w';
    }
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
      const r = unit.kind === 'weight' ? metricWeight(q * unit.base, qm != null ? qm * unit.base : null) : metricVolume(q * unit.base, qm != null ? qm * unit.base : null);
      [text, unitText] = [r.text, r.unit];
    } else if (unit && unit.kind === 'volume') {
      const r = usVolume(q * unit.base, qm != null ? qm * unit.base : null, o.fractions);
      // Keep big-batch units (pints, quarts) as written rather than recasting to cups.
      if (['pint', 'quart', 'gallon', 'floz'].includes(unit.key) || qm != null) {
        text = formatNumber(q, o.fractions) + (qm != null ? '–' + formatNumber(qm, o.fractions) : '');
        unitText = q > 1 || qm != null ? unit.many : unit.one;
      } else [text, unitText] = [r.text, r.unit];
    } else if (unit && unit.kind === 'weight') {
      const r = usWeight(q * unit.base, qm != null ? qm * unit.base : null, o.fractions);
      [text, unitText] = [r.text, r.unit];
    } else {
      const f = (v: number) => formatNumber(v, true);
      text = f(q) + (qm != null ? '–' + f(qm) : '');
      if (unit) unitText = q > 1 || qm != null ? unit.many : unit.one;
      else if (p.qty <= 1 && q > 1) {
        const w = out.rest.match(/^([a-z]+)\b/);
        if (w && PLURALS[w[1]]) out.rest = PLURALS[w[1]] + out.rest.slice(w[1].length);
      }
    }
    const size = p.size ? ' ' + p.size : '';
    out.measure = `${lead}${text}${size}${unitText ? ' ' + unitText : ''}`.trim();
    if (p.alt) {
      // Scale the author's alternate too, so the pair stays consistent.
      const a = p.alt;
      const aq = a.qty * o.scale;
      const am = a.qtyMax != null ? a.qtyMax * o.scale : null;
      const r = a.unit.kind === 'weight'
        ? a.unit.system === 'metric' ? metricWeight(aq * a.unit.base, am != null ? am * a.unit.base : null) : usWeight(aq * a.unit.base, am != null ? am * a.unit.base : null, o.fractions)
        : a.unit.system === 'metric' ? metricVolume(aq * a.unit.base, am != null ? am * a.unit.base : null) : usVolume(aq * a.unit.base, am != null ? am * a.unit.base : null, o.fractions);
      out.measure += ` (${r.text} ${r.unit})`;
    }
    return out;
  }

  // Convert.
  let baseQty = src.qty * src.unit.base * o.scale;
  let baseMax = src.qtyMax != null ? src.qtyMax * src.unit.base * o.scale : null;
  if (src.unit.kind !== wantKind) {
    // volume <-> weight via standard density
    const d = dens as number;
    const k = wantKind === 'weight' ? d / 236.588 : 236.588 / d;
    baseQty *= k;
    if (baseMax != null) baseMax *= k;
    out.estimated = true;
  }
  const r =
    target === 'metric-w' ? metricWeight(baseQty, baseMax)
    : target === 'metric-v' ? metricVolume(baseQty, baseMax)
    : target === 'us-w' ? usWeight(baseQty, baseMax, o.fractions)
    : usVolume(baseQty, baseMax, o.fractions);
  out.measure = `${lead}${r.text} ${r.unit}`;
  out.converted = !usedAlt;
  out.from = usedAlt ? '' : p.head;
  return out;
}

// ---------------------------------------------------------------- baker's maths

const NOT_FLOUR = /tortilla|for dusting|for the (work )?surface|for rolling|for kneading|to dust/i;
const WATER_SHARE: [RegExp, number][] = [
  [/starter|levain|poolish|biga/, 0.5], [/egg yolk/, 0.5], [/egg white/, 0.88], [/eggs?\b/, 0.75], [/buttermilk/, 0.9], [/butter/, 0.16],
  [/cream cheese/, 0.5], [/(heavy|double|whipping) cream/, 0.58], [/sour cream/, 0.72], [/yogh?urt|kefir/, 0.85], [/milk/, 0.87],
  [/honey|syrup|molasses/, 0.18], [/oil|shortening|lard/, 0], [/water|beer|coffee|juice|stock|broth|wine/, 1],
];

export function looksLikeFlour(name: string): boolean {
  return /flour|semolina|\bmeal\b/i.test(name) && !NOT_FLOUR.test(name);
}

export interface BakerRow {
  grams: number | null;
  flour: boolean;
  percent: number | null;
}

/**
 * Baker's percentages: every ingredient as a share of total flour weight.
 * `flourOverride` lets the cook correct the automatic flour detection.
 */
export function bakers(items: Parsed[], flourOverride: Record<string, boolean> = {}) {
  const rows: BakerRow[] = items.map((p) => {
    const flour = flourOverride[p.raw] ?? looksLikeFlour(p.rest);
    return { grams: grams(p), flour, percent: null };
  });
  let flourTotal = 0;
  let water = 0;
  let total = 0;
  items.forEach((p, i) => {
    const g = rows[i].grams;
    if (g == null) return;
    total += g;
    const name = p.rest.toLowerCase();
    const starter = /starter|levain|poolish|biga/.test(name);
    if (rows[i].flour) flourTotal += g;
    else if (starter) flourTotal += g / 2; // assume a 100%-hydration preferment
    for (const [re, share] of WATER_SHARE) {
      if (re.test(name)) {
        water += g * share;
        break;
      }
    }
  });
  if (flourTotal > 0) rows.forEach((r) => (r.percent = r.grams != null ? (r.grams / flourTotal) * 100 : null));
  return { rows, flour: flourTotal, total, hydration: flourTotal > 0 ? (water / flourTotal) * 100 : null };
}
