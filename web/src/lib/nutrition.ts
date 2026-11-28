// Nutrition, worked out from the ingredients. Every line Fond can weigh is matched to a food in the
// dictionary, which carries USDA FoodData Central figures per 100 g. The result is an estimate, and it
// says so: which lines were counted, which could not be, and how much of the recipe that covers.

import { foodById } from './foods';
import { lookup, parseIngredient, weigh, type WeighOptions } from './ingredients';
import type { Line } from './types';

export const NUTRIENTS = [
  { key: 'kcal', label: 'Calories', unit: '' },
  { key: 'protein', label: 'Protein', unit: 'g' },
  { key: 'fat', label: 'Fat', unit: 'g' },
  { key: 'satfat', label: 'Saturated fat', unit: 'g' },
  { key: 'carbs', label: 'Carbohydrate', unit: 'g' },
  { key: 'fiber', label: 'Fibre', unit: 'g' },
  { key: 'sugar', label: 'Sugars', unit: 'g' },
  { key: 'sodium', label: 'Sodium', unit: 'mg' },
] as const;

export type NutrientKey = (typeof NUTRIENTS)[number]['key'];

export interface Nutrition {
  /** For the whole recipe as written, in the order of NUTRIENTS. */
  total: number[];
  /** For one serving, when the recipe says how many it serves. */
  each: number[] | null;
  servings: number;
  /** Lines that were counted. */
  counted: number;
  /** Lines with an amount that could not be counted, as written. */
  missed: string[];
  /** Lines with no amount ("salt to taste"), which are left out. */
  unmeasured: number;
  /** Grams counted, against grams that were weighed at all: how much of the recipe the estimate covers. */
  coverage: number;
  /** True when any weight in the sum was itself an estimate. */
  approx: boolean;
}

// Fat for frying, flour for dusting: mostly not eaten, so not counted.
const NOT_EATEN = /\bfor (deep[- ]?)?frying\b|\bfor (greasing|dusting|brushing|the pan|the tin|the work surface|rolling|oiling)\b|\bto grease\b|\bto dust\b/i;

export function nutritionOf(r: { ingredients: Line[]; servings: number }, o: WeighOptions = {}): Nutrition {
  const total = NUTRIENTS.map(() => 0);
  let counted = 0, unmeasured = 0, gCounted = 0, gAll = 0, approx = false;
  const missed: string[] = [];
  for (const line of r.ingredients) {
    if (line.h || !line.text.trim()) continue;
    if (NOT_EATEN.test(line.text)) continue;
    const p = parseIngredient(line.text);
    if (p.qty == null) {
      unmeasured++;
      continue;
    }
    const m = lookup(p, o);
    if (m.skip) continue; // bay leaves, cinnamon sticks: never eaten, never weighed
    const w = weigh(p, o);
    const food = w?.match?.food ?? m.food;
    const nu = food?.nu ?? (m.rule?.foodId ? foodById(m.rule.foodId)?.nu : undefined);
    if (w) gAll += w.g;
    if (!w || !nu) {
      missed.push(line.text);
      continue;
    }
    for (let i = 0; i < total.length; i++) total[i] += (w.g * (nu[i] ?? 0)) / 100;
    gCounted += w.g;
    counted++;
    approx ||= w.approx;
  }
  const servings = r.servings > 0 ? r.servings : 0;
  return {
    total, servings, counted, missed, unmeasured, approx,
    each: servings ? total.map((v) => v / servings) : null,
    coverage: gAll > 0 ? gCounted / gAll : counted ? 1 : 0,
  };
}

/** A figure as a label shows it: whole calories and milligrams, grams to one place under ten. */
export function fmt(v: number, key: NutrientKey): string {
  if (key === 'kcal' || key === 'sodium') return String(Math.round(v));
  if (v < 0.05) return '0';
  return v < 10 ? (Math.round(v * 10) / 10).toString() : String(Math.round(v));
}
