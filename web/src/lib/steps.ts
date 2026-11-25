// Finds timers and oven temperatures inside the text of a step.

export type Token =
  | { t: 'text'; v: string }
  | { t: 'timer'; v: string; seconds: number }
  | { t: 'temp'; v: string; shown: string; converted: boolean };

const WORDS: Record<string, number> = { a: 1, an: 1, one: 1, two: 2, three: 3, four: 4, five: 5, six: 6, eight: 8, ten: 10, fifteen: 15, twenty: 20, thirty: 30, 'forty-five': 45 };
const N = String.raw`(?:\d+\s+\d\/\d|\d+\/\d|\d+(?:\.\d+)?\s*[½¼¾]?|[½¼¾]|half an?|an?|one|two|three|four|five|six|eight|ten|fifteen|twenty|thirty|forty-five)`;
const UNIT = String.raw`(hours?|hrs?|hr|h|minutes?|mins?|min|seconds?|secs?|sec)`;
const RE_TIME = new RegExp(
  String.raw`\b(${N})(?:\s*(?:-|–|—|to|or)\s*(${N}))?[\s-]*(?:more\s+|additional\s+|further\s+|full\s+)?${UNIT}\b(?:\s*(?:and\s+|,\s*)?(${N})[\s-]*${UNIT}\b)?`,
  'gi',
);
const RE_TEMP = /(\d{2,3})\s*(?:°|º|˚|degrees?)\s*([FC])?(?![a-z])|\b(\d{3})\s*([FC])\b/gi;

function num(s: string): number {
  s = s.trim().toLowerCase();
  if (s.startsWith('half')) return 0.5;
  if (s in WORDS) return WORDS[s];
  const fr: Record<string, number> = { '½': 0.5, '¼': 0.25, '¾': 0.75 };
  let extra = 0;
  const last = s[s.length - 1];
  if (last in fr) {
    extra = fr[last];
    s = s.slice(0, -1).trim();
    if (!s) return extra;
  }
  const mixed = s.match(/^(\d+)\s+(\d)\/(\d)$/);
  if (mixed) return +mixed[1] + +mixed[2] / +mixed[3];
  const frac = s.match(/^(\d+)\/(\d)$/);
  if (frac) return +frac[1] / +frac[2];
  return parseFloat(s) + extra;
}

function unitSeconds(u: string): number {
  u = u.toLowerCase();
  return u.startsWith('h') ? 3600 : u.startsWith('m') ? 60 : 1;
}

export function convertTemp(value: number, from: 'F' | 'C'): number {
  if (from === 'F') return Math.round(((value - 32) * 5) / 9 / 5) * 5;
  const f = (value * 9) / 5 + 32;
  return f >= 250 ? Math.round(f / 25) * 25 : Math.round(f / 5) * 5;
}

/** Splits step text into plain text, tappable timers and temperatures. */
export function tokenize(text: string, temp: 'original' | 'C' | 'F' = 'original'): Token[] {
  type Hit = { i: number; len: number; tok: Token };
  const hits: Hit[] = [];
  for (const m of text.matchAll(RE_TIME)) {
    const first = m[1].trim().toLowerCase();
    // "a minute" is a timer; a bare article before "second" is prose ("a second bowl").
    if ((first === 'a' || first === 'an') && /^s/i.test(m[3])) continue;
    let seconds = num(m[1]) * unitSeconds(m[3]);
    if (m[4] && m[5]) seconds += num(m[4]) * unitSeconds(m[5]);
    if (!isFinite(seconds) || seconds < 5 || seconds > 86400) continue;
    hits.push({ i: m.index!, len: m[0].length, tok: { t: 'timer', v: m[0], seconds: Math.round(seconds) } });
  }
  for (const m of text.matchAll(RE_TEMP)) {
    const value = +(m[1] ?? m[3]);
    let unit = (m[2] ?? m[4])?.toUpperCase() as 'F' | 'C' | undefined;
    if (!unit) unit = value > 260 ? 'F' : 'C';
    if (value < 30 || value > 600) continue;
    const i = m.index!;
    if (hits.some((h) => i < h.i + h.len && h.i < i + m[0].length)) continue;
    let shown = m[0];
    let converted = false;
    if (temp !== 'original' && temp !== unit) {
      // Skip when the author already gives the other unit right after, e.g. "350°F (175°C)".
      const after = text.slice(i + m[0].length, i + m[0].length + 14);
      if (!new RegExp(String.raw`\d{2,3}\s*(?:°|º|˚)?\s*${temp}\b`).test(after)) {
        shown = `${convertTemp(value, unit)}°${temp}`;
        converted = true;
      }
    }
    hits.push({ i, len: m[0].length, tok: { t: 'temp', v: m[0], shown, converted } });
  }
  hits.sort((a, b) => a.i - b.i);
  const out: Token[] = [];
  let pos = 0;
  for (const h of hits) {
    if (h.i < pos) continue;
    if (h.i > pos) out.push({ t: 'text', v: text.slice(pos, h.i) });
    out.push(h.tok);
    pos = h.i + h.len;
  }
  if (pos < text.length) out.push({ t: 'text', v: text.slice(pos) });
  return out;
}

export function clock(seconds: number): string {
  const s = Math.max(0, Math.ceil(seconds));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const ss = String(s % 60).padStart(2, '0');
  return h ? `${h}:${String(m).padStart(2, '0')}:${ss}` : `${m}:${ss}`;
}

export function minutesLabel(min: number): string {
  if (!min) return '';
  const h = Math.floor(min / 60);
  const m = min % 60;
  return h ? (m ? `${h} hr ${m} min` : `${h} hr`) : `${m} min`;
}
