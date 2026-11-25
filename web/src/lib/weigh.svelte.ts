// The ingredient dictionary and the household's corrections, as shared state.
// The dictionary is a separate chunk, fetched once and cached with the app.

import { del, get, put } from './api';
import { allRules, loadFoods, ruleKey, setRules, type Rule } from './foods';
import type { WeighOptions } from './ingredients';
import { saltFor } from './salt';
import type { Prefs, Recipe } from './types';

/** `rev` changes whenever a conversion could come out differently; read it to stay current. */
export const dict = $state({ ready: false, rev: 0, version: '', rules: [] as Rule[] });

let loading: Promise<void> | null = null;

export function ensureFoods(): Promise<void> {
  loading ??= import('../data/foods.json')
    .then((m) => {
      const data = (m.default ?? m) as any;
      loadFoods(data);
      // Loading the dictionary clears nothing the household has taught Fond.
      setRules(dict.rules);
      dict.version = data.version;
      dict.ready = true;
      dict.rev++;
    })
    .catch(() => {
      loading = null;
    });
  return loading;
}

export function useRules(rules: Rule[] | null | undefined) {
  dict.rules = rules ?? [];
  setRules(dict.rules);
  dict.rev++;
}

export async function loadRules() {
  try {
    useRules(await get<Rule[]>('/food-rules'));
  } catch {
    /* keep what we have */
  }
}

/** Adds or replaces the household's rule for a phrase. */
export async function saveRule(rule: Rule): Promise<Rule> {
  const key = ruleKey(rule.phrase);
  const saved = await put<Rule>('/food-rules', rule);
  // "toffee bit" and "toffee bits" are one ingredient: the new rule replaces the old.
  for (const old of allRules()) {
    if (old.id && old.id !== saved.id && ruleKey(old.phrase) === key) await del(`/food-rules/${old.id}`).catch(() => {});
  }
  useRules([...allRules().filter((r) => ruleKey(r.phrase) !== key), saved].sort((a, b) => a.phrase.localeCompare(b.phrase)));
  return saved;
}

export async function removeRule(rule: Rule) {
  if (rule.id) await del(`/food-rules/${rule.id}`);
  useRules(allRules().filter((r) => r.phrase !== rule.phrase));
}

/** Everything that decides how one recipe is weighed for one reader. */
export function weighOptions(r: Pick<Recipe, 'title' | 'tags' | 'ingredients' | 'extra'> | null, pf: Pick<Prefs, 'flour'>): WeighOptions {
  if (!r) return { flour: pf.flour };
  const base: WeighOptions = { flour: pf.flour, fixes: r.extra?.fixes };
  const said = r.extra?.salt;
  if (said) return { ...base, salt: { brand: said, sure: true } };
  const guess = dict.ready ? saltFor(r, base) : null;
  return { ...base, salt: guess?.brand ? { brand: guess.brand, sure: false } : null };
}
