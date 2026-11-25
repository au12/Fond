import { describe, expect, it } from 'vitest';
import foods from '../data/foods.json';
import { loadFoods } from './foods';
import { SALT_RANGES, saltFor } from './salt';

loadFoods(foods as any);

const recipe = (title: string, lines: string[], tags: string[] = []) => ({ title, tags, ingredients: lines.map((text) => ({ text })), extra: null });
const guess = (title: string, lines: string[], tags: string[] = []) => saltFor(recipe(title, lines, tags));

describe('which kosher salt a recipe means', () => {
  it('has nothing to say when the recipe is clear already', () => {
    expect(guess('Loaf', ['500 g bread flour', '10 g salt'])).toBe(null);
    expect(guess('Loaf', ['500 g bread flour', '2 tsp Diamond Crystal kosher salt'])).toBe(null);
    expect(guess('Loaf', ['500 g bread flour', '2 tsp (6 g) kosher salt'])).toBe(null);
    expect(guess('Loaf', ['500 g bread flour', '1 tsp table salt'])).toBe(null);
  });

  it('reads bread by its salt percentage', () => {
    // 1 tbsp: 1.7 % as Diamond Crystal, 2.9 % as Morton. Bread is about 2 %.
    const a = guess('Country loaf', ['500 g bread flour', '375 g water', '1 tablespoon kosher salt', '1 tsp instant yeast'])!;
    expect([a.kind, a.brand]).toEqual(['bread', 'dc']);
    expect(a.percent!.dc).toBeCloseTo(1.68, 1);
    expect(a.percent!.morton).toBeCloseTo(2.88, 1);
    expect(a.facts).toContain('1.7 % of the flour as Diamond Crystal');
    expect(a.facts).toContain('usual range for bread is 1.8–2.2 %');
    expect(a.verdict).toBe('That points to Diamond Crystal.');
    // 2 tsp: 1.1 % as Diamond Crystal, 1.9 % as Morton.
    const b = guess('Sandwich bread', ['500 g bread flour', '350 g water', '2 teaspoons kosher salt', '7 g yeast'])!;
    expect([b.kind, b.brand]).toEqual(['bread', 'morton']);
  });

  it('classifies doughs by what is in them when the title does not say', () => {
    const lean = guess('Weekend bake', ['4 cups all-purpose flour', '1 1/2 cups water', '1 tbsp kosher salt', '1 tsp instant yeast'])!;
    expect(lean.kind).toBe('bread');
    const rich = guess('Weekend bake', ['4 cups all-purpose flour', '1 cup milk', '1/2 cup sugar', '1 stick butter', '2 tsp kosher salt', '1 packet active dry yeast'])!;
    expect(rich.kind).toBe('enriched');
    const sweet = guess('Grandma’s favourite', ['2 1/4 cups all-purpose flour', '1 cup butter', '1 1/2 cups sugar', '1 tsp kosher salt', '2 eggs'])!;
    expect(sweet.kind).toBe('cookies');
  });

  it('counts other salt in the recipe towards the total', () => {
    const a = guess('Bagels', ['500 g bread flour', '1 tsp kosher salt', '5 g fine sea salt'])!;
    expect(a.percent!.dc).toBeCloseTo((2.8 + 5) / 5, 1);
  });

  it('says so when the amount fits both brands', () => {
    // Half a teaspoon in a batch of cookies is 0.5 % or 0.9 %: both are normal.
    const a = guess('Chocolate chip cookies', ['2 1/4 cups all-purpose flour', '1 cup butter', '1 cup sugar', '1/2 tsp kosher salt'])!;
    expect([a.kind, a.brand]).toEqual(['cookies', null]);
    expect(a.verdict).toContain('fits either');
  });

  it('is careful outside baking, where other ingredients carry salt too', () => {
    // 1 tbsp on two pounds of potatoes: 0.9 % or 1.5 %. Morton would oversalt it.
    const a = guess('Roast potatoes', ['2 lb potatoes', '2 tbsp olive oil', '1 tablespoon kosher salt'])!;
    expect([a.kind, a.brand]).toEqual(['dish', 'dc']);
    // Low salt next to soy sauce proves nothing.
    const b = guess('Stir-fry', ['1 lb broccoli', '3 tbsp soy sauce', '1/2 tsp kosher salt', '1 cup water'])!;
    expect(b.brand).toBe(null);
    // Both brands are ordinary on two pounds of chicken.
    const c = guess('Roast chicken thighs', ['2 lb chicken thighs', '2 tsp kosher salt'])!;
    expect([c.kind, c.brand]).toEqual(['meat', null]);
  });

  it('still sees a cookie when it is mostly chocolate', () => {
    // Flour is under a fifth of this dough by weight; the title settles it.
    const a = guess('Brown butter chocolate chip cookies', ['1 cup butter', '2 1/4 cups all-purpose flour', '1 tsp kosher salt', '1 cup packed brown sugar', '1/2 cup sugar', '2 large eggs', '2 cups chocolate chips', '1 cup chopped walnuts'])!;
    expect([a.kind, a.brand]).toEqual(['cookies', 'dc']);
    // A spoonful of flour does not make a stew a bake.
    const b = guess('Beef stew', ['2 lb beef chuck', '2 tbsp all-purpose flour', '4 cups water', '2 tsp kosher salt'])!;
    expect(b.kind).not.toBe('baked');
    expect(SALT_RANGES[b.kind].of).not.toBe('flour');
  });

  it('does not guess when it cannot weigh the recipe', () => {
    const a = guess('Mystery', ['3 handfuls mystery grain', '1 tsp kosher salt'])!;
    expect(a.brand).toBe(null);
    expect(a.percent).toBe(null);
  });
});
