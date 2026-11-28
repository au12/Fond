import { describe, expect, it } from 'vitest';
import foods from '../data/foods.json';
import { loadFoods } from './foods';
import { nutritionOf } from './nutrition';

loadFoods(foods as any);

const lines = (...t: string[]) => t.map((text) => (text.startsWith('# ') ? { text: text.slice(2), h: true } : { text }));

describe('nutrition', () => {
  it('adds up what each ingredient carries', () => {
    // 100 g of all-purpose flour is 364 kcal and 100 g of sugar 387 in USDA SR Legacy.
    const n = nutritionOf({ ingredients: lines('100 g flour', '100 g sugar'), servings: 2 });
    expect(Math.round(n.total[0])).toBe(751);
    expect(Math.round(n.each![0])).toBe(376);
    expect(n.counted).toBe(2);
    expect(n.coverage).toBe(1);
  });

  it('weighs cups and counts first', () => {
    const n = nutritionOf({ ingredients: lines('1 cup all-purpose flour', '2 large eggs'), servings: 0 });
    // 120 g flour (437 kcal) and 100 g egg (143 kcal).
    expect(Math.round(n.total[0])).toBe(580);
    expect(n.each).toBeNull();
  });

  it('says what it could not count, and leaves out what is not eaten', () => {
    const n = nutritionOf({
      ingredients: lines('# For the dough', '500 g bread flour', '2 tsp xanthan gum', 'salt to taste', '4 cups oil, for deep frying', '1 bay leaf', '3 dragon scales'),
      servings: 4,
    });
    expect(n.counted).toBe(1);
    expect(n.missed).toEqual(['2 tsp xanthan gum', '3 dragon scales']);
    expect(n.unmeasured).toBe(1);
    expect(n.coverage).toBeGreaterThan(0.95);
    // Sodium comes through for salt.
    const salty = nutritionOf({ ingredients: lines('1 tsp table salt'), servings: 1 });
    expect(Math.round(salty.total[7])).toBeGreaterThan(2000);
  });
});
