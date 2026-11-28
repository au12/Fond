// The meal plan: dates, weeks, and turning planned meals into a shopping list.

import { get, post } from './api';
import { parseIngredient, show } from './ingredients';
import type { Recipe } from './types';
import { weighOptions } from './weigh.svelte';

export type Meal = 'breakfast' | 'lunch' | 'dinner' | 'snack';
export const MEALS: [Meal, string][] = [['breakfast', 'Breakfast'], ['lunch', 'Lunch'], ['dinner', 'Dinner'], ['snack', 'Snack']];
export const mealLabel = (m: string) => MEALS.find(([k]) => k === m)?.[1] ?? m;

export interface PlanEntry { id: number; day: string; meal: Meal; recipeId: number | null; text: string; servings: number; position: number }

/** A date as the plan keys it: local, not UTC, so a late-evening plan stays on its day. */
export function iso(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}
export function parseDay(s: string): Date {
  const [y, m, d] = s.split('-').map(Number);
  return new Date(y, m - 1, d);
}
export function addDays(d: Date, n: number): Date {
  const x = new Date(d);
  x.setDate(x.getDate() + n);
  return x;
}

/** The first day of the week where the reader lives: Sunday in the US, Monday in most of the world. */
export function firstDayOfWeek(): number {
  try {
    const loc = new Intl.Locale(navigator.language) as any;
    const info = loc.getWeekInfo?.() ?? loc.weekInfo;
    if (info?.firstDay) return info.firstDay % 7;
  } catch {
    /* older browsers */
  }
  return /^en-(US|CA)|^(ja|ko|he|pt-BR|zh-TW)/.test(navigator.language) ? 0 : 1;
}
export function weekStart(d: Date, first = firstDayOfWeek()): Date {
  const x = new Date(d.getFullYear(), d.getMonth(), d.getDate());
  return addDays(x, -((x.getDay() - first + 7) % 7));
}

export const loadPlan = (from: string, to: string) => get<PlanEntry[]>(`/plan?from=${from}&to=${to}`);

/** Adds the ingredients of planned meals to the shopping list, each scaled to the servings planned. */
export async function shopFor(entries: PlanEntry[], flour: 'spoon' | 'dip'): Promise<number> {
  const ids = [...new Set(entries.filter((e) => e.recipeId).map((e) => e.recipeId!))];
  const recipes = new Map<number, Recipe>();
  await Promise.all(ids.map((id) => get<Recipe>(`/recipes/${id}`).then((r) => recipes.set(id, r)).catch(() => {})));
  const items: { text: string; recipeId: number; recipeTitle: string }[] = [];
  for (const e of entries) {
    const r = e.recipeId ? recipes.get(e.recipeId) : null;
    if (!r) continue;
    const scale = e.servings > 0 && r.servings > 0 ? e.servings / r.servings : 1;
    const o = { scale, system: 'original' as const, weigh: false, fractions: true, ...weighOptions(r, { flour }) };
    for (const l of r.ingredients) {
      if (l.h || !l.text.trim()) continue;
      const s = show(parseIngredient(l.text), o);
      items.push({ text: scale === 1 ? l.text : (s.measure + ' ' + s.rest).trim(), recipeId: r.id, recipeTitle: r.title });
    }
  }
  for (let i = 0; i < items.length; i += 300) await post('/shopping', { items: items.slice(i, i + 300) });
  return items.length;
}
