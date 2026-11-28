#!/usr/bin/env python3
"""Builds Fond's ingredient dictionary (web/src/data/foods.json).

Fond weighs ingredients without asking anyone to maintain a food database.
This script does the heavy lifting once, at build time:

  1. download   USDA SR Legacy, USDA FNDDS and FSANZ AUSNUT 2023 food measures
  2. extract    every cup/spoon portion as grams per millilitre (by state:
                chopped, packed, sifted ...) and every counted portion as
                grams each (large egg, clove, stick ...)
  3. curate     foods.txt maps the names cooks actually write to those records
  4. cross-check take the median across independent source families and flag
                anything whose sources disagree by more than 10 %
  5. emit       web/src/data/foods.json and scripts/foods/report.md

Usage:
  python3 scripts/foods/build.py              build (downloads on first run)
  python3 scripts/foods/build.py find onion   search source foods and portions

Only the Python standard library is needed. Source data is public domain
(USDA, CC0) or CC BY 4.0 (© Food Standards Australia New Zealand).
"""
from __future__ import annotations

import csv
import glob
import json
import os
import re
import statistics
import sys
import urllib.request
import zipfile
from xml.etree import ElementTree as ET

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(os.path.dirname(HERE))
CACHE = os.environ.get('FOND_FOODS_CACHE', os.path.join(HERE, '.cache'))
OUT = os.path.join(ROOT, 'web', 'src', 'data', 'foods.json')
REPORT = os.path.join(HERE, 'report.md')

SOURCES = {
    'sr': 'https://fdc.nal.usda.gov/fdc-datasets/FoodData_Central_sr_legacy_food_csv_2018-04.zip',
    'fndds': 'https://fdc.nal.usda.gov/fdc-datasets/FoodData_Central_survey_food_csv_2024-10-31.zip',
    'ausnut': 'https://www.foodstandards.gov.au/sites/default/files/2025-08/AUSNUT%202023%20-%20Food%20measures.xlsx',
}
VERSION = 'SR Legacy 2018-04 · FNDDS 2021-2023 · AUSNUT 2023'

CUP, TBSP, TSP, FLOZ = 236.588, 14.7868, 4.92892, 29.5735
UNIT_ML = {
    'cup': CUP, 'cups': CUP, 'tbsp': TBSP, 'tablespoon': TBSP, 'tablespoons': TBSP,
    'tsp': TSP, 'teaspoon': TSP, 'teaspoons': TSP, 'fl oz': FLOZ, 'quart': 946.353, 'pint': 473.176,
}
RE_VOL = re.compile(r'^(cups?|tbsp|tablespoons?|tsp|teaspoons?|fl oz|quart|pint)\b(.*)$', re.I)

# How source wording maps to the states a recipe line can ask for.
STATE_WORDS = [
    (r'unsifted', ''), (r'sifted', 'sifted'), (r'stirred', ''), (r'uncooked|\bdry\b', ''), (r'slivered', 'slivered'),
    (r'not packed|unpacked|loosely packed|loose', 'loose'), (r'packed', 'packed'),
    (r'whipped', 'whipped'), (r'unwhipped', ''), (r'melted', 'melted'),
    (r'mashed', 'mashed'), (r'pur[eé]e', 'pureed'), (r'crushed', 'crushed'), (r'crumbled|crumbs', 'crumbled'),
    (r'shredded|grated', 'shredded'), (r'ground', 'ground'),
    (r'chopped|diced|dice\b|minced', 'chopped'), (r'slice|strips', 'sliced'),
    (r'cubes|cubed|chunks', 'cubed'), (r'halves', 'halves'), (r'pieces', 'pieces'),
    (r'whole|kernels', 'whole'), (r'cooked', 'cooked'),
]
SKIP_PORTION = re.compile(r'yields|as purchased|with pits|in shell|with refuse|with juice|undrained|prepared|unthawed|condensed|\(\d+/\d+ cup\)', re.I)

SIZES = [(r'^extra[- ]large|^xl\b', 'xl'), (r'^jumbo', 'jumbo'), (r'^large', 'large'), (r'^medium', 'medium'), (r'^small', 'small')]
COUNT_UNITS = ['clove', 'stick', 'slice', 'stalk', 'sprig', 'leaf', 'head', 'ear', 'bunch', 'wedge', 'strip', 'rib', 'pat']


def state_of(text: str) -> str | None:
    """The state a portion describes; '' for the plain measure, None to skip it."""
    t = text.lower()
    if SKIP_PORTION.search(t):
        return None
    for pat, state in STATE_WORDS:
        if re.search(pat, t):
            return state
    return ''


# ------------------------------------------------------------------ sources

def download() -> None:
    os.makedirs(CACHE, exist_ok=True)
    for key, url in SOURCES.items():
        dest = os.path.join(CACHE, key)
        if glob.glob(os.path.join(dest, '**', '*.*'), recursive=True):
            continue
        os.makedirs(dest, exist_ok=True)
        name = 'food_measures.xlsx' if url.endswith('.xlsx') else 'data.zip'
        path = os.path.join(dest, name)
        print(f'downloading {key} …', file=sys.stderr)
        req = urllib.request.Request(url, headers={'User-Agent': 'fond-foods-build'})
        with urllib.request.urlopen(req, timeout=300) as r, open(path, 'wb') as f:
            f.write(r.read())
        if name.endswith('.zip'):
            with zipfile.ZipFile(path) as z:
                z.extractall(dest)
            os.remove(path)


def _csv(key: str, name: str):
    hits = glob.glob(os.path.join(CACHE, key, '**', name), recursive=True)
    if not hits:
        sys.exit(f'{name} for {key} not found under {CACHE}; run without FOND_FOODS_CACHE to download')
    with open(hits[0], newline='', encoding='utf-8') as f:
        yield from csv.DictReader(f)


class Food:
    """One record from a source database."""

    def __init__(self, family: str, name: str):
        self.family, self.name = family, name
        self.gml: dict[str, list[float]] = {}   # state -> densities
        self._ml: dict[str, float] = {}         # state -> largest measure seen
        self.each: dict[str, float] = {}        # size or unit -> grams
        self.water: float | None = None
        self.nut: list[float] | None = None     # per 100 g, in the order of NUTRIENTS
        self.raw: list[str] = []                # the portions as the source words them

    def add_volume(self, state: str, gml: float, ml: float = CUP) -> None:
        # A cup is weighed far more precisely than a teaspoon ("1 tsp = 4.2 g"),
        # so keep only the readings taken with the largest measure.
        best = self._ml.get(state, 0)
        if ml > best * 1.5:
            self.gml[state], self._ml[state] = [gml], ml
        elif ml > best / 1.5:
            self.gml[state].append(gml)

    def density(self, state: str = '') -> float | None:
        v = self.gml.get(state)
        return statistics.median(v) if v else None


def _add_portion(food: Food, amount: float, text: str, grams: float) -> None:
    text = text.strip()
    if grams <= 0 or amount <= 0:
        return
    food.raw.append(f'{amount:g} {text} = {grams:g} g')
    m = RE_VOL.match(text)
    if m:
        state = state_of(m.group(2))
        if state is not None:
            ml = UNIT_ML[m.group(1).lower()]
            food.add_volume(state, grams / (amount * ml), ml)
        return
    low = text.lower()
    for pat, size in SIZES:
        if re.search(pat, low):
            food.each.setdefault(size, grams / amount)
            return
    for unit in COUNT_UNITS:
        if re.match(rf'^{unit}s?\b', low):
            food.each.setdefault(unit, grams / amount)
            return
    if re.match(r'^(fruit|whole|each|egg|item|pepper|tomato|carrot|potato|onion|shallot|avocado|banana|apple|lemon|lime|orange|peach|pear|plum|fig|date|olive|scallion|mushroom|radish|beet|leek|artichoke|cucumber|zucchini|squash|eggplant|bulb|pod|chili|pepper)\b', low):
        food.each.setdefault('each', grams / amount)


# What a portion carries, per 100 g: SR Legacy names nutrients by id, FNDDS by number.
NUTRIENTS = [  # (key, SR id, FNDDS number)
    ('kcal', '1008', '208'), ('protein', '1003', '203'), ('fat', '1004', '204'), ('satfat', '1258', '606'),
    ('carbs', '1005', '205'), ('fiber', '1079', '291'), ('sugar', '2000', '269'), ('sodium', '1093', '307'),
]


def _nutrients(foods: dict[str, Food], key: str, col: int) -> None:
    index = {n[col]: i for i, n in enumerate(NUTRIENTS)}
    for r in _csv(key, 'food_nutrient.csv'):
        i = index.get(r['nutrient_id'])
        f = foods.get(r['fdc_id'])
        if i is None or f is None or not r['amount']:
            continue
        if f.nut is None:
            f.nut = [0.0] * len(NUTRIENTS)
        f.nut[i] = float(r['amount'])


def load_sr() -> dict[str, Food]:
    names = {r['fdc_id']: r['description'] for r in _csv('sr', 'food.csv')}
    foods = {fid: Food('USDA', n) for fid, n in names.items()}
    for r in _csv('sr', 'food_portion.csv'):
        f = foods.get(r['fdc_id'])
        if f:
            _add_portion(f, float(r['amount'] or 1), r['modifier'], float(r['gram_weight'] or 0))
    for r in _csv('sr', 'food_nutrient.csv'):
        if r['nutrient_id'] == '1051' and r['fdc_id'] in foods:
            foods[r['fdc_id']].water = float(r['amount']) / 100
    _nutrients(foods, 'sr', 1)
    return {f.name: f for f in foods.values()}


def load_fndds() -> dict[str, Food]:
    names = {r['fdc_id']: r['description'] for r in _csv('fndds', 'food.csv')}
    foods = {fid: Food('USDA', n) for fid, n in names.items()}
    for r in _csv('fndds', 'food_portion.csv'):
        f = foods.get(r['fdc_id'])
        m = re.match(r'^([\d./]+)\s+(.*)$', r['portion_description'])
        if f and m:
            try:
                amount = float(m.group(1))
            except ValueError:
                continue
            _add_portion(f, amount, m.group(2), float(r['gram_weight'] or 0))
    _nutrients(foods, 'fndds', 2)
    return {f.name: f for f in foods.values()}


def _xlsx_rows(path: str, sheet: str):
    """Rows of one worksheet as lists of cell values (strings or floats)."""
    ns = {'m': 'http://schemas.openxmlformats.org/spreadsheetml/2006/main', 'r': 'http://schemas.openxmlformats.org/officeDocument/2006/relationships'}
    with zipfile.ZipFile(path) as z:
        strings = [''.join(t.text or '' for t in si.iter('{%s}t' % ns['m'])) for si in ET.fromstring(z.read('xl/sharedStrings.xml'))]
        book = ET.fromstring(z.read('xl/workbook.xml'))
        rels = {r.get('Id'): r.get('Target') for r in ET.fromstring(z.read('xl/_rels/workbook.xml.rels'))}
        target = next(rels[s.get('{%s}id' % ns['r'])] for s in book.find('m:sheets', ns) if s.get('name') == sheet)
        root = ET.fromstring(z.read('xl/' + target.lstrip('/').removeprefix('xl/')))
    for row in root.find('m:sheetData', ns):
        out: list = []
        for c in row:
            col = 0
            for ch in re.match(r'[A-Z]+', c.get('r')).group(0):
                col = col * 26 + ord(ch) - 64
            v = c.find('m:v', ns)
            val: object = None
            if c.get('t') == 's' and v is not None:
                val = strings[int(v.text)]
            elif c.get('t') == 'inlineStr':
                val = ''.join(t.text or '' for t in c.iter('{%s}t' % ns['m']))
            elif v is not None:
                try:
                    val = float(v.text)
                except ValueError:
                    val = v.text
            out.extend([None] * (col - len(out)))
            out[col - 1] = val
        yield out


def load_ausnut() -> dict[str, Food]:
    hits = glob.glob(os.path.join(CACHE, 'ausnut', '**', '*.xlsx'), recursive=True)
    if not hits:
        sys.exit(f'AUSNUT food measures workbook not found under {CACHE}')
    foods: dict[str, Food] = {}
    for row in _xlsx_rows(hits[0], 'AUSNUT 2023'):
        row = row + [None] * 10
        name, d1, grams = row[2], row[5], row[9]
        if not isinstance(name, str) or not isinstance(grams, float) or not isinstance(d1, str):
            continue
        f = foods.setdefault(name, Food('AUSNUT', name))
        extra = ' '.join(str(x) for x in row[6:9] if x)
        f.raw.append(f'{d1.strip()} {extra} = {grams:g}'.replace('  ', ' '))
        if d1.strip().lower() == 'density':
            low = extra.lower()
            if re.search(r'piece|wedge|block|solid|whole fruit', low):
                continue
            state = state_of(extra) if extra else ''
            if 'not defined' in low:
                state = ''
            if state is not None:
                f.add_volume(state, grams)
    return foods


# ------------------------------------------------------------------ curation

class Entry:
    def __init__(self, line: str, lineno: int):
        parts = [p.strip() for p in re.split(r'\s*\|\s*', line)]
        if len(parts) < 5:
            sys.exit(f'foods.txt:{lineno}: expected "id | name | aliases | category | options"')
        self.id, self.name = parts[0], parts[1]
        self.aliases = [a.strip() for a in parts[2].split(',') if a.strip()]
        self.cat = parts[3]
        self.opts = parts[4].split()
        self.fields: list[tuple[str, str, str]] = []  # key, state, value
        for p in parts[5:]:
            m = re.match(r'^([a-z]+)(?:\[([a-z]*)\])?:\s*(.*)$', p)
            if not m:
                sys.exit(f'foods.txt:{lineno}: cannot read "{p}"')
            self.fields.append((m.group(1), m.group(2) or '', m.group(3)))
        self.lineno = lineno

    def opt(self, key: str) -> str | None:
        for o in self.opts:
            if o == key:
                return ''
            if o.startswith(key + '='):
                return o[len(key) + 1:]
        return None


def read_entries() -> list[Entry]:
    out = []
    with open(os.path.join(HERE, 'foods.txt'), encoding='utf-8') as f:
        for i, line in enumerate(f, 1):
            line = line.rstrip('\n')
            if line.strip() and not line.lstrip().startswith('#'):
                out.append(Entry(line, i))
    return out


def measure(text: str, lineno: int) -> tuple[float, str]:
    """'120/cup (King Arthur)' -> (g per ml, label)."""
    m = re.match(r'^([\d.]+)\s*/\s*(cup|tbsp|tsp|ml|floz)\s*(?:\((.*)\))?$', text.strip())
    if not m:
        sys.exit(f'foods.txt:{lineno}: cannot read measure "{text}"')
    per = {'cup': CUP, 'tbsp': TBSP, 'tsp': TSP, 'ml': 1, 'floz': FLOZ}[m.group(2)]
    return float(m.group(1)) / per, (m.group(3) or '').strip()


ERRORS: list[str] = []


def resolve(db: dict[str, Food], name: str, e: Entry, key: str) -> Food:
    if name in db:
        return db[name]
    words = set(re.findall(r'[a-z]+', name.lower()))
    near = sorted(db, key=lambda n: -len(words & set(re.findall(r'[a-z]+', n.lower()))) / (1 + abs(len(n) - len(name)) / 40))[:4]
    ERRORS.append(f'foods.txt:{e.lineno}: {key} food not found: "{name}"\n      nearest: ' + ' ¦ '.join(near))
    return Food('?', name)


STATE_ORDER = ['chopped', 'sliced', 'whole', 'pieces', 'shredded', 'halves', 'cubed', 'ground', 'packed', 'loose', 'crushed', 'mashed']


def build() -> None:
    download()
    sr, fn, au = load_sr(), load_fndds(), load_ausnut()
    entries = read_entries()
    foods, report, seen_alias, seen_id = [], [], {}, set()
    likes: list = []
    flagged = 0
    for e in entries:
        if e.id in seen_id:
            sys.exit(f'foods.txt:{e.lineno}: duplicate id {e.id}')
        seen_id.add(e.id)
        # Gather what each family says, per state.
        fam: dict[str, dict[str, list[float]]] = {}   # state -> family -> values
        each: dict[str, float] = {}
        water = None
        nut: list[float] | None = None
        nut_from = ''
        pins: dict[str, tuple[float, str]] = {}
        alts: dict[str, float] = {}
        usda_primary = False
        for key, st, val in e.fields:
            if key in ('sr', 'fn', 'au'):
                rec = resolve({'sr': sr, 'fn': fn, 'au': au}[key], val, e, key)
                label = rec.family
                for state, vals in rec.gml.items():
                    target = st if (st and state == '') else state
                    if st and state != '' and st != state:
                        continue
                    slot = fam.setdefault(target, {}).setdefault(label, [])
                    # SR is the primary USDA record; FNDDS only fills what it lacks.
                    if key == 'fn' and usda_primary and slot:
                        continue
                    slot.append(statistics.median(vals))
                # Nutrition comes from the food's plain USDA record: SR Legacy first, FNDDS where SR is silent.
                if key in ('sr', 'fn') and not st and rec.nut and nut_from != 'nut' and (nut is None or (key == 'sr' and nut_from == 'fn')):
                    nut, nut_from = rec.nut, key
                if key == 'sr':
                    usda_primary = True
                    if water is None and not st:
                        water = rec.water
                if key in ('sr', 'fn') and not st and e.opt('noeach') is None:
                    for k, g in rec.each.items():
                        each.setdefault(k, g)
            elif key == 'ref':
                for chunk in val.split(';'):
                    g, label = measure(chunk, e.lineno)
                    fam.setdefault(st, {}).setdefault(label or 'reference', []).append(g)
            elif key == 'pin':
                pins[st] = measure(val, e.lineno)
            elif key == 'alt':
                alts[st] = measure(val, e.lineno)[0]
            elif key == 'each':
                for kv in val.split():
                    k, v = kv.split('=')
                    each[k] = float(v)
                    if not float(v):
                        del each[k]
            elif key == 'water':
                water = float(val)
            elif key == 'nut':
                # Nutrition from another record when the food's weights come from elsewhere: a USDA description,
                # or "like <id>" (optionally "like <id> x 0.5") to share another food's.
                lm = re.match(r'^like\s+([a-z0-9-]+)(?:\s*[x×]\s*([\d.]+))?$', val)
                if lm:
                    likes.append((len(foods), lm.group(1), float(lm.group(2) or 1), e))
                else:
                    rec = sr.get(val) or fn.get(val)
                    if rec is None or not rec.nut:
                        resolve(sr, val, e, 'nut')
                    else:
                        nut, nut_from = rec.nut, 'nut'
            else:
                sys.exit(f'foods.txt:{e.lineno}: unknown field "{key}"')

        chosen: dict[str, float] = {}
        basis: list[list] = []
        conf = 'A'
        note = ''
        for state in sorted(set(fam) | set(pins)):
            by_family = {k: statistics.median(v) for k, v in fam.get(state, {}).items()}
            vals = list(by_family.values())
            spread = (max(vals) - min(vals)) / statistics.median(vals) if len(vals) > 1 else 0
            if state in pins:
                value = pins[state][0]
                if pins[state][1]:
                    by_family = {pins[state][1]: value, **{k: v for k, v in by_family.items() if k != pins[state][1]}}
            elif len(vals) >= 3:
                value = statistics.median(vals)
            elif len(vals) == 2 and spread <= 0.10:
                value = sum(vals) / 2
            elif 'USDA' in by_family:
                value = by_family['USDA']
            else:
                value = vals[0]
            chosen[state] = value
            if state == (e.opt('def') or ''):
                basis = [[k, round(v, 4)] for k, v in by_family.items()]
                if state not in pins:
                    if spread > 0.10:
                        conf, note = 'B', f'sources differ by {spread:.0%}'
                        flagged += 1
                    elif len(vals) == 1 and e.fields and e.fields[0][0] != 'sr':
                        conf = 'B'
        default_state = e.opt('def') or ''
        default = chosen.get(default_state)
        if default is None and chosen:
            default_state = next((s for s in STATE_ORDER if s in chosen), sorted(chosen)[0])
            default = chosen[default_state]
            basis = [[k, round(statistics.median(v), 4)] for k, v in fam.get(default_state, {}).items()]
        if e.opt('est') is not None:
            conf = 'B'
        states = {s: round(v, 4) for s, v in chosen.items() if s and abs(v - (default or 0)) / (default or 1) > 0.03}
        if e.opt('nodefault') is not None:
            states, default, default_state = {s: round(v, 4) for s, v in chosen.items() if s}, None, ''
        if default is None and not each and not states and e.opt('skip') is None:
            ERRORS.append(f'foods.txt:{e.lineno}: {e.id} has neither a density nor a count weight')

        for a in [e.name] + e.aliases:
            a = a.split('@')[0]
            if a in seen_alias:
                ERRORS.append(f'foods.txt:{e.lineno}: alias "{a}" already belongs to {seen_alias[a]}')
            seen_alias.setdefault(a, e.id)

        out: dict = {'id': e.id, 'n': e.name}
        if e.aliases:
            out['a'] = e.aliases
        out['c'] = e.cat
        if default is not None:
            out['d'] = round(default, 4)
            if default_state:
                out['ds'] = default_state
        if states:
            out['s'] = states
        if each:
            out['e'] = {k: round(v, 1) if v < 100 else round(v) for k, v in each.items()}
        if alts:
            out['x'] = {k or 'dip': round(v, 4) for k, v in alts.items()}
        if water is not None:
            out['w'] = round(water, 3)
        if nut is not None and e.opt('skip') is None and e.opt('nonut') is None:
            out['nu'] = [round(v) if i in (0, 7) else round(v, 1) for i, v in enumerate(nut)]
        for flag, key in (('flour', 'f'), ('loose', 'l'), ('any', 'y'), ('count', 'q'), ('skip', 'z')):
            if e.opt(flag) is not None:
                out[key] = 1
        if e.opt('salt') is not None:
            out['t'] = e.opt('salt')
        if e.opt('head') is not None:
            out['h'] = e.opt('head')
        if conf != 'A':
            out['k'] = conf
        if basis:
            out['b'] = basis
        foods.append(out)

        cup = lambda v: f'{v * CUP:.0f}' if v is not None else ''
        fams = '; '.join(f'{k} {cup(v)}' for k, v in basis)
        extra = ', '.join(f'{s} {cup(v)}' for s, v in sorted(states.items()))
        report.append(f'| {e.id} | {cup(default)}{" (" + default_state + ")" if default_state else ""} | {fams} | {extra} | {" ".join(f"{k}={v:g}" for k, v in out.get("e", {}).items())} | {conf}{" " + note if note else ""} |')

    by_id = {f['id']: f for f in foods}
    for at, other, factor, e in likes:
        src = by_id.get(other, {}).get('nu')
        if src is None:
            ERRORS.append(f'foods.txt:{e.lineno}: nut: like {other}, but {other} has no nutrition')
        elif 'nu' not in foods[at]:
            foods[at]['nu'] = [round(v * factor) if i in (0, 7) else round(v * factor, 1) for i, v in enumerate(src)]
    if ERRORS:
        sys.exit('\n'.join(ERRORS) + f'\n{len(ERRORS)} problems in foods.txt')
    os.makedirs(os.path.dirname(OUT), exist_ok=True)
    with open(OUT, 'w', encoding='utf-8') as f:
        json.dump({'version': VERSION, 'foods': foods}, f, ensure_ascii=False, separators=(',', ':'))
        f.write('\n')
    with open(REPORT, 'w', encoding='utf-8') as f:
        f.write('# Ingredient dictionary report\n\n')
        f.write(f'Generated by `scripts/foods/build.py` from {VERSION}.\n\n')
        f.write(f'{len(foods)} foods, {sum(len(x.get("a", [])) + 1 for x in foods)} names. '
                f'{flagged} foods have sources that disagree by more than 10 % and are marked B (shown with ≈).\n\n')
        f.write('Densities are grams per US cup (236.6 ml). "Sources" lists what each family says for the default state.\n\n')
        f.write('| Food | g per cup | Sources | Other states | Each (g) | Confidence |\n| --- | --- | --- | --- | --- | --- |\n')
        f.write('\n'.join(report) + '\n')
    size = os.path.getsize(OUT)
    print(f'{len(foods)} foods → {os.path.relpath(OUT, ROOT)} ({size / 1024:.0f} KB); {flagged} flagged; report in {os.path.relpath(REPORT, ROOT)}')


def find(pattern: str) -> None:
    download()
    rx = re.compile(pattern, re.I)
    for label, db in (('sr', load_sr()), ('fn', load_fndds()), ('au', load_ausnut())):
        for name, food in sorted(db.items()):
            if rx.search(name) and food.raw:
                print(f'{label}: {name}\n      {"; ".join(food.raw[:12])}')


if __name__ == '__main__':
    if len(sys.argv) > 2 and sys.argv[1] == 'find':
        find(' '.join(sys.argv[2:]))
    else:
        build()
