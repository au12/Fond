// The ingredient dictionary and the matcher that connects a recipe line to it.
//
// The dictionary (../data/foods.json) is built by scripts/foods/build.py from
// USDA and AUSNUT data. Matching is deliberately conservative: a line is tied
// to a food by its head noun, never by words in its notes, and anything it
// cannot place is left alone and marked rather than guessed.

export interface Food {
  id: string;
  /** Display name. */
  n: string;
  /** Other names; "garlic clove@clove" also carries a unit or a state. */
  a?: string[];
  /** Category: flour, sugar, dairy, veg ... */
  c: string;
  /** Grams per millilitre for the plain measure. */
  d?: number;
  /** The state the plain measure assumes ("chopped" for onions). */
  ds?: string;
  /** Grams per millilitre by state. */
  s?: Record<string, number>;
  /** Grams each, by size (small, medium, large, xl, jumbo, each) or unit (clove, stick). */
  e?: Record<string, number>;
  /** Density under another convention (dip-and-sweep flour). */
  x?: Record<string, number>;
  /** Water fraction, for hydration. */
  w?: number;
  /** Counts as flour in baker's percentages. */
  f?: 1;
  /** Unknown describing words are fine. */
  l?: 1;
  /** A catch-all: even the names of other foods are fine as describing words. */
  y?: 1;
  /** Never weighed. */
  z?: 1;
  /** Kind of salt. */
  t?: string;
  /** B: an estimate. */
  k?: 'B';
  /** What each source family says, grams per millilitre. */
  b?: [string, number][];
}

/** A household correction, or a per-recipe one. */
export interface Rule {
  id?: number;
  phrase: string;
  action: 'map' | 'custom' | 'skip';
  foodId?: string;
  gPerMl?: number | null;
  gEach?: number | null;
  source?: string;
}

export interface Match {
  food: Food | null;
  rule: Rule | null;
  via: 'rule' | 'exact' | 'head' | 'typo' | 'none';
  /** The state the line asks for: chopped, packed, sifted ... */
  state: string;
  /** small, medium, large, xl, jumbo or ''. */
  size: string;
  /** A unit or state the matched name itself carries. */
  tag: string;
  /** The ingredient's name with measuring words removed; the key for rules. */
  phrase: string;
  /** The name as written, without notes. */
  name: string;
  /** Everything after the name: "sifted", "for the glaze". */
  notes: string;
  approx: boolean;
  /** Not something anyone weighs. */
  skip: boolean;
}

// ---------------------------------------------------------------- vocabulary

const list = (s: string) => new Set(s.split(/\s+/).filter(Boolean));

/** Words that describe without changing what the ingredient is. */
const SAFE = list(`
  fresh freshly organic raw ripe overripe cold warm hot chilled frozen thawed defrosted unsalted salted low sodium
  reduced fat free full nonfat lowfat good quality best pure real natural plain regular unsweetened sweetened
  store bought storebought homemade leftover prepared european style grass fed range boneless skinless seedless pitted
  peeled unpeeled trimmed rinsed drained toasted roasted blanched unbleached bleached enriched vegan dairy softened
  room temperature lukewarm tepid optional heaping heaped scant level rounded generous about approximately each total
  of the a an your favorite favourite choice any some additional extra plus more divided very well shaken beaten
  lightly finely roughly coarsely thinly thickly firmly tightly loosely gently separated cooled cut into inch stem
  removed cored seeded deseeded deveined washed patted sized size mild sharp aged fine coarse light dark hulled shelled
  unsulphured unsulfured all local canned tinned jarred bottled packaged can tin jar bag box in on with without from
  at for and or to if needed as desired use used using i we you like preferably such see note recipe follows
  virgin premium quick old fashioned traditional classic style brand name wild caught farm skin bone
  halved quartered torn picked stripped snipped julienned unwrapped scrubbed stemmed zested juiced squeezed
  thick thin long short round flat heavy level packed
  glutenfree dairyfree nutfree eggfree grainfree soyfree lactosefree oilfree wheatfree meatfree
  temp mccormick gourmet hershey kirkland heinz kraft kikkoman lurpak kerrygold pillsbury domino nestle ghirardelli
  california usa british imported
`);
SAFE.delete('packed');

/** Words that change what the ingredient is: an unknown match must not swallow them. */
const BLOCK = list(`
  dried dry dehydrated powdered instant condensed evaporated malted pickled candied crystallized crystallised glace
  freeze concentrate concentrated fried crispy puffed popped quail duck goose imitation
  cooked steamed boiled baked grilled sauteed caramelized caramelised braised poached scrambled hash sugarfree
`);

/** Words left behind by a second measurement ("1 tbsp cornstarch + 1 tbsp water"): the line holds two things. */
const MEASURES = list('cup tbsp tsp tablespoon teaspoon ml g gr gram gm oz ounce fl lb');
/** Heads of some names ("egg white", "mixed greens") that are plain describing words everywhere else. */
const COLOURS = list('white green red black brown yellow golden purple pink blue');
/** Real words that sit one letter from a food's name. */
const NOT_TYPOS = list('fillet filet finger pocket packet bottle butler paper water waiter');

const STATES: Record<string, string> = {
  chopped: 'chopped', diced: 'chopped', minced: 'chopped', dice: 'chopped',
  cubed: 'cubed', cubes: 'cubed', chunks: 'cubed', chunk: 'cubed',
  sliced: 'sliced', slices: 'sliced', shredded: 'shredded', grated: 'shredded', shaved: 'shredded',
  mashed: 'mashed', smashed: 'mashed', crushed: 'crushed', crumbled: 'crumbled', crumbles: 'crumbled',
  packed: 'packed', sifted: 'sifted', melted: 'melted', whipped: 'whipped', ground: 'ground',
  pureed: 'pureed', halves: 'halves', pieces: 'pieces', slivered: 'slivered', whole: 'whole',
};
/** States that still count when they appear after the comma ("1 cup onion, chopped"). */
const NOTE_STATES = new Set(['chopped', 'cubed', 'sliced', 'shredded', 'mashed', 'crushed', 'crumbled', 'packed', 'pureed']);
/** When the state asked for is unknown for a food, the next best thing. */
const STATE_FALLBACK: Record<string, string[]> = {
  cubed: ['chopped', 'pieces'], chopped: ['cubed', 'pieces'], pieces: ['chopped', 'halves'], halves: ['pieces', 'whole'],
  crushed: ['chopped'], crumbled: ['chopped'], mashed: ['pureed'], pureed: ['mashed'], slivered: ['sliced'], sliced: ['slivered'],
};

/** States an alias can carry ("sliced almonds@sliced"). */
const TAG_STATES = new Set([...Object.values(STATES), 'cooked', 'loose']);

const SIZES: Record<string, string> = {
  small: 'small', medium: 'medium', large: 'large', big: 'large', jumbo: 'jumbo', xl: 'xl',
  // Too small to stand in for the usual one: "baby carrots", "mini cucumbers".
  baby: 'tiny', mini: 'tiny', miniature: 'tiny', tiny: 'tiny', petite: 'tiny', cocktail: 'tiny', pearl: 'tiny',
};

/** Trailing words that name a part or a cut rather than the ingredient. */
const TRAIL = list('leaf floret kernel piece chunk slice wedge strip sprig stalk stem segment cube round ribbon spear frond thread strand needle petal top');

const SINGULAR: Record<string, string> = {
  leaves: 'leaf', halves: 'half', loaves: 'loaf', knives: 'knife', tomatoes: 'tomato', potatoes: 'potato',
  mangoes: 'mango', avocadoes: 'avocado', chilies: 'chili', chillies: 'chilli', chiles: 'chile',
};
const KEEP_S = /(ss|us|is|as|ous)$/;

function singular(w: string): string {
  if (SINGULAR[w]) return SINGULAR[w];
  if (w.length <= 3 || KEEP_S.test(w) || /\d/.test(w)) return w;
  if (w.endsWith('ies') && w.length > 4) return w.slice(0, -3) + 'y';
  if (/(ches|shes|xes|zes|sses)$/.test(w)) return w.slice(0, -2);
  if (w.endsWith('s')) return w.slice(0, -1);
  return w;
}

function rawWords(text: string): string[] {
  return text
    .toLowerCase()
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .replace(/[’`´]/g, "'")
    .replace(/'s\b/g, '')
    // "gluten-free" is one word, so that "gluten" is never mistaken for the ingredient.
    .replace(/\b(gluten|sugar|dairy|nut|egg|grain|soy|lactose|oil|wheat|meat)[- ]free\b/g, '$1free')
    .replace(/&/g, ' and ')
    .replace(/[^a-z0-9%']+/g, ' ')
    .replace(/'/g, '')
    .trim()
    .split(/\s+/)
    .filter(Boolean);
}

/** The words of a name, lower-cased, unaccented and singular. */
export function words(text: string): string[] {
  return rawWords(text).map(singular);
}

// ---------------------------------------------------------------- dictionary

interface Alias {
  tokens: string[];
  head: string;
  food: Food;
  tag: string;
}

let FOODS: Food[] = [];
let VERSION = '';
const BY_ID = new Map<string, Food>();
const BY_HEAD = new Map<string, Alias[]>();
const VOCAB = new Set<string>();
let RULES = new Map<string, Rule>();

const isFiller = (t: string) => SAFE.has(t) || t in STATES || t in SIZES || MEASURES.has(t) || /^\d/.test(t);

function headOf(tokens: string[]): string {
  for (let i = tokens.length - 1; i >= 0; i--) {
    const t = tokens[i];
    if (!isFiller(t) && !TRAIL.has(t)) return t;
  }
  return tokens[tokens.length - 1] ?? '';
}

export function loadFoods(data: { version: string; foods: Food[] }): void {
  CACHE.clear();
  FOODS = data.foods;
  VERSION = data.version;
  BY_ID.clear();
  BY_HEAD.clear();
  VOCAB.clear();
  for (const food of FOODS) {
    BY_ID.set(food.id, food);
    for (const name of [food.n, ...(food.a ?? [])]) {
      const [text, tag = ''] = name.split('@');
      const tokens = words(text);
      if (!tokens.length) continue;
      const alias: Alias = { tokens, head: headOf(tokens), food, tag };
      const bucket = BY_HEAD.get(alias.head);
      if (bucket) bucket.push(alias);
      else BY_HEAD.set(alias.head, [alias]);
      for (const t of tokens) VOCAB.add(t);
    }
  }
}

export const foodsLoaded = () => FOODS.length > 0;
export const foodsVersion = () => VERSION;
export const allFoods = () => FOODS;
export const foodById = (id: string) => BY_ID.get(id) ?? null;

export function setRules(rules: Rule[]): void {
  CACHE.clear();
  RULES = new Map(rules.map((r) => [r.phrase.toLowerCase(), r]));
}
export const allRules = () => [...RULES.values()];

// ---------------------------------------------------------------- reading a name

const RE_NOTE = /\s*(?:[,;(+]|\s[-–—]\s|\s(?:and|or|&)\s+(?=[\d¼½¾⅓⅔⅛])|\s(?:for|plus|with|without|such as|like|from the|from a|to taste|as needed|at room temperature|if needed|divided|optional)\b)/i;
const RE_SKIP = /\b(for (?:dusting|greasing|garnish|garnishing|serving|sprinkling|frying|deep[- ]frying|brushing|the pan|rolling|the work surface|drizzling|topping|finishing|decorating|decoration|coating|kneading|oiling|the grill)|to serve|to garnish|to dust|to grease|for the (?:pan|bowl|baking sheet|tin))\b/i;
/** "plus more for dusting": the extra is unmeasured, the amount before it is not. */
const RE_EXTRA = /[,;(]?\s*\b(?:plus|and|with)\s+(?:more|extra|additional|a little|some)\b.*$/i;

export interface Reading {
  /** The name as written, without notes. */
  name: string;
  notes: string;
  tokens: string[];
  state: string;
  size: string;
  /** Tokens with measuring and describing words removed. */
  core: string[];
  skip: boolean;
}

/** Splits "all-purpose flour, sifted" into a name and notes and reads its describing words. */
export function readName(rest: string): Reading {
  // A leading parenthetical is a measure ("(2 sticks) butter"), not the name.
  let text = rest.replace(/^\s*(?:\([^)]*\)\s*)+/, '');
  // A stray unit left by an odd format ("113 g / 4 oz butter") and "of".
  text = text.replace(/^\s*(?:\/\s*)?(?:g|gms?|grams?|ml|oz|ounces?|tsp|tbsp|teaspoons?|tablespoons?)\.?\s+(?=[a-z])/i, '').replace(/^\s*of\s+/i, '');
  // "low-sodium", "no salt added", "fat-free": diet labels, not ingredients. ("Gluten-free" and
  // "sugar-free" stay: those are different foods.)
  text = text
    .replace(/\b(?:low|lower|reduced|less|no|zero|without)[- ](?:added[- ])?(?:salt|sodium|sugar|fat|carb|calorie)s?(?:[- ]added)?\b/gi, ' ')
    .replace(/\b(?:salt|sodium|fat|calorie|alcohol)[- ]free\b/gi, ' ');
  // "butter/margarine" is a choice.
  text = text.replace(/([a-z])\s*\/\s*(?=[a-z])/gi, '$1 or ');
  const skip = RE_SKIP.test(text.replace(RE_EXTRA, ''));
  let notes = '';
  // "chopped, toasted walnuts", "medium (about 1 cup) mushrooms": a name made only of
  // describing words runs on past the comma or the bracket.
  for (let from = 0; ; ) {
    const cut = text.slice(from).search(RE_NOTE);
    if (cut < 0) break;
    const at = from + cut;
    const head = at > 0 ? rawWords(text.slice(0, at).replace(/\([^)]*\)/g, ' ')).map(singular) : [];
    const sep = text.slice(at).trimStart()[0];
    const next = sep === ',' ? text.indexOf(',', at) + 1 : sep === '(' ? text.indexOf(')', at) + 1 : 0;
    if (head.some((t) => !isFiller(t)) || !next) {
      if (at > 0) {
        notes = text.slice(at);
        text = text.slice(0, at);
      }
      break;
    }
    from = next;
  }
  const raw = rawWords(text.replace(/\([^)]*\)/g, ' '));
  let state = '';
  let size = '';
  for (let i = 0; i < raw.length; i++) {
    const w = raw[i];
    if (w in STATES && !state) state = w === 'packed' && raw[i - 1] === 'loosely' ? 'loose' : STATES[w];
    if (w === 'unpacked') state = 'loose';
    if (w === 'large' && (raw[i - 1] === 'extra' || raw[i - 1] === 'x')) size = 'xl';
    else if (w in SIZES && !size) size = SIZES[w];
  }
  if (!state) {
    // "1 cup onion, chopped": a cut named in the notes still describes what is in the cup.
    for (const w of rawWords(notes)) {
      if (w in STATES && NOTE_STATES.has(STATES[w])) {
        state = STATES[w];
        break;
      }
    }
  }
  const tokens = raw.map(singular);
  const core = tokens.filter((t) => !isFiller(t));
  return { name: text.trim(), notes: notes.trim(), tokens, state, size, core, skip };
}

// ---------------------------------------------------------------- matching

function editDistance(a: string, b: string, max: number): number {
  if (Math.abs(a.length - b.length) > max) return max + 1;
  const prev = new Array(b.length + 1);
  const cur = new Array(b.length + 1);
  let before: number[] | null = null;
  for (let j = 0; j <= b.length; j++) prev[j] = j;
  for (let i = 1; i <= a.length; i++) {
    cur[0] = i;
    let best = cur[0];
    for (let j = 1; j <= b.length; j++) {
      const cost = a[i - 1] === b[j - 1] ? 0 : 1;
      cur[j] = Math.min(prev[j] + 1, cur[j - 1] + 1, prev[j - 1] + cost);
      if (before && i > 1 && j > 1 && a[i - 1] === b[j - 2] && a[i - 2] === b[j - 1]) cur[j] = Math.min(cur[j], before[j - 2] + 1);
      best = Math.min(best, cur[j]);
    }
    if (best > max) return max + 1;
    before = prev.slice();
    for (let j = 0; j <= b.length; j++) prev[j] = cur[j];
  }
  return prev[b.length];
}

/** The alias that best covers these tokens, or null. */
function bestAlias(tokens: string[]): { alias: Alias; exact: boolean } | null {
  if (!tokens.length) return null;
  const head = headOf(tokens);
  const bucket = BY_HEAD.get(head);
  if (!bucket) return null;
  const have = new Map<string, number>();
  for (const t of tokens) have.set(t, (have.get(t) ?? 0) + 1);
  let best: { alias: Alias; exact: boolean; score: number } | null = null;
  // The longest name that fitted but was turned down because the line also names another food.
  let refused = 0;
  for (const alias of bucket) {
    const need = new Map<string, number>();
    for (const t of alias.tokens) need.set(t, (need.get(t) ?? 0) + 1);
    let covered = true;
    for (const [t, n] of need) {
      if ((have.get(t) ?? 0) < n) {
        covered = false;
        break;
      }
    }
    if (!covered) continue;
    // What the line says beyond the alias.
    const unknown: string[] = [];
    for (const [t, n] of have) {
      const extra = n - (need.get(t) ?? 0);
      if (extra > 0 && !isFiller(t) && !TRAIL.has(t)) unknown.push(t);
    }
    // "dried", "cooked" and the names of other foods ("pepper corn", "macaroni cheese") make it something else.
    if (unknown.some((t) => BLOCK.has(t))) continue;
    if (!alias.food.y && unknown.some((t) => !COLOURS.has(t) && BY_HEAD.get(t)?.some((a) => a.food !== alias.food))) {
      refused = Math.max(refused, alias.tokens.length);
      continue;
    }
    if (unknown.length && alias.tokens.length === 1 && !alias.food.l) continue;
    // Prefer the most specific alias; among equals, the one that explains the whole line.
    const score = alias.tokens.length * 10 - unknown.length + (alias.food.k === 'B' ? -0.5 : 0);
    if (!best || score > best.score) best = { alias, exact: unknown.length === 0, score };
  }
  // "low-fat soy sauce with honey" is not just any sauce: leave it for the cook to settle.
  if (best && best.alias.food.y && !best.exact && refused > best.alias.tokens.length) return null;
  return best;
}

function fixTypos(tokens: string[]): string[] | null {
  let changed = false;
  const out = tokens.map((t) => {
    if (VOCAB.has(t) || isFiller(t) || t.length < 5 || BLOCK.has(t) || NOT_TYPOS.has(t) || TRAIL.has(t)) return t;
    const max = t.length >= 8 ? 2 : 1;
    let best = '';
    let bestD = max + 1;
    let ties = 0;
    for (const v of VOCAB) {
      if (v.length < 4 || v[0] !== t[0]) continue;
      const d = editDistance(t, v, max);
      if (d < bestD) {
        best = v;
        bestD = d;
        ties = 1;
      } else if (d === bestD) ties++;
    }
    if (best && ties === 1) {
      changed = true;
      return best;
    }
    return t;
  });
  return changed ? out : null;
}

const NONE = (r: Reading): Match => ({
  food: null, rule: null, via: 'none', state: r.state, size: r.size, tag: '', phrase: r.core.join(' '), name: r.name, notes: r.notes, approx: false, skip: r.skip,
});

function attempt(r: Reading, tokens: string[]): { alias: Alias; exact: boolean } | null {
  const whole = bestAlias(tokens);
  if (whole?.exact) return whole;
  // "walnuts or pecans", "vegetable or canola oil": the first choice is the one to weigh.
  const or = tokens.indexOf('or');
  if (or > 0 && or < tokens.length - 1) {
    const left = tokens.slice(0, or);
    const right = tokens.slice(or + 1);
    // "sherry or cider vinegar" is sherry vinegar: the first choice borrows the shared noun.
    const shared = bestAlias([...left, headOf(right)]);
    if (shared?.exact) return shared;
    const side = bestAlias(left) ?? bestAlias(right);
    if (side) return side;
  }
  return whole;
}

const CACHE = new Map<string, Match>();

/** Ties the remainder of an ingredient line ("all-purpose flour, sifted") to a food. */
export function matchFood(rest: string): Match {
  let m = CACHE.get(rest);
  if (!m) {
    m = match(rest);
    if (CACHE.size > 5000) CACHE.clear();
    CACHE.set(rest, m);
  }
  return m;
}

function match(rest: string): Match {
  const r = readName(rest);
  const base = NONE(r);
  if (!r.core.length) return base;
  // "olive oil 1 cup water" names two things; "15 oz black beans" names one.
  const second = r.tokens.findIndex((t) => MEASURES.has(t));
  if (second >= 0 && r.tokens.slice(0, second).some((t) => !isFiller(t)) && r.tokens.slice(second + 1).some((t) => !isFiller(t))) return base;

  const rule = RULES.get(base.phrase);
  if (rule) {
    if (rule.action === 'skip') return { ...base, rule, via: 'rule', skip: true };
    if (rule.action === 'custom') return { ...base, rule, via: 'rule' };
    const food = rule.foodId ? BY_ID.get(rule.foodId) : null;
    if (food) return { ...base, food, rule, via: 'rule', approx: false, skip: r.skip || !!food.z };
  }
  if (!FOODS.length) return base;

  let hit = attempt(r, r.tokens);
  let via: Match['via'] = hit?.exact ? 'exact' : 'head';
  if (!hit) {
    const fixed = fixTypos(r.tokens);
    if (fixed) {
      hit = attempt(r, fixed);
      via = 'typo';
    }
  }
  if (!hit) return base;
  const { food, tag } = hit.alias;
  const state = tag && TAG_STATES.has(tag) ? tag : r.state;
  return {
    ...base,
    food,
    via,
    state,
    tag: state === tag ? '' : tag,
    approx: food.k === 'B' || via === 'typo',
    skip: r.skip || !!food.z,
  };
}

export interface Density {
  gml: number;
  approx: boolean;
}

/** Grams per millilitre for a matched line, honouring its state and the cook's flour convention. */
export function densityOf(m: Match, opts: { flour?: 'spoon' | 'dip' } = {}): Density | null {
  if (m.rule?.action === 'custom') return m.rule.gPerMl ? { gml: m.rule.gPerMl, approx: false } : null;
  const food = m.food;
  if (!food) return null;
  const approx = m.approx;
  const wanted = m.state && m.state !== food.ds ? m.state : '';
  if (wanted) {
    for (const s of [wanted, ...(STATE_FALLBACK[wanted] ?? [])]) {
      if (food.s?.[s] != null) return { gml: food.s[s], approx };
      if (s === food.ds && food.d != null) return { gml: food.d, approx };
    }
  }
  if (food.d == null) return null;
  if (opts.flour === 'dip' && food.x?.dip) return { gml: food.x.dip, approx };
  return { gml: food.d, approx };
}

/** Grams for one counted item ("1 large egg", "2 cloves garlic"). */
export function eachOf(m: Match, unit = ''): Density | null {
  if (m.rule?.action === 'custom') return m.rule.gEach ? { gml: m.rule.gEach, approx: false } : null;
  const e = m.food?.e;
  if (!e) return null;
  const key = unit || m.tag;
  if (key) return e[key] != null ? { gml: e[key], approx: m.approx } : null;
  if (m.size === 'tiny') return null;
  if (m.size && e[m.size] != null) return { gml: e[m.size], approx: m.approx };
  const g = e.each ?? e.medium ?? e.large;
  // A size the food has no weight for ("jumbo shallot") is still a guess at the usual one.
  return g != null ? { gml: g, approx: m.approx || !!m.size } : null;
}

/** Foods whose names resemble the query, best first; for the "pick a food" search. */
export function searchFoods(query: string, limit = 8): Food[] {
  const q = words(query).filter((t) => !SAFE.has(t));
  if (!q.length) return [];
  const scored: [number, Food][] = [];
  for (const food of FOODS) {
    if (food.z) continue;
    let best = 0;
    for (const name of [food.n, ...(food.a ?? [])]) {
      const tokens = words(name.split('@')[0]);
      let hits = 0;
      for (const t of q) {
        if (tokens.includes(t)) hits += 1;
        else if (tokens.some((x) => x.startsWith(t) && t.length >= 3)) hits += 0.6;
      }
      if (!hits) continue;
      const score = hits / q.length - (tokens.length - hits) * 0.04 + (tokens[tokens.length - 1] === q[q.length - 1] ? 0.2 : 0);
      if (score > best) best = score;
    }
    if (best > 0.3) scored.push([best, food]);
  }
  return scored.sort((a, b) => b[0] - a[0]).slice(0, limit).map((x) => x[1]);
}

/** "USDA 125 · King Arthur 120" for one measure of this food. */
export function basisText(food: Food, ml: number): string {
  if (!food.b?.length) return '';
  return food.b.map(([who, gml]) => `${who} ${formatGrams(gml * ml)}`).join(' · ');
}

function formatGrams(g: number): string {
  return g >= 20 ? String(Math.round(g)) : String(+g.toFixed(1));
}
