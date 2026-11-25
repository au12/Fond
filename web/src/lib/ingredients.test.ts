import { describe, expect, it } from 'vitest';
import foods from '../data/foods.json';
import { loadFoods } from './foods';
import { bakers, parseIngredient as p, show, type DisplayOptions } from './ingredients';
import { tokenize } from './steps';

loadFoods(foods as any);

const base: DisplayOptions = { scale: 1, system: 'original', weigh: false, fractions: true };
const s = (text: string, o: Partial<DisplayOptions> = {}) => {
  const r = show(p(text), { ...base, ...o });
  return (r.measure + ' ' + r.rest).trim();
};

describe('parsing', () => {
  it('reads quantities, units and names', () => {
    const a = p('2 1/2 cups all-purpose flour, sifted');
    expect([a.qty, a.unit?.key, a.rest]).toEqual([2.5, 'cup', 'all-purpose flour, sifted']);
    const b = p('1½ tsp. kosher salt');
    expect([b.qty, b.unit?.key, b.rest]).toEqual([1.5, 'tsp', 'kosher salt']);
    const c = p('250g bread flour');
    expect([c.qty, c.unit?.key, c.rest]).toEqual([250, 'g', 'bread flour']);
    const d = p('3 large eggs');
    expect([d.qty, d.unit, d.rest]).toEqual([3, null, 'large eggs']);
    const e = p('Salt and pepper, to taste');
    expect([e.qty, e.rest]).toEqual([null, 'Salt and pepper, to taste']);
    expect(p('2 cloves garlic, minced').unit?.key).toBe('clove');
    expect(p('1 T olive oil').unit?.key).toBe('tbsp');
    expect(p('1 t vanilla').unit?.key).toBe('tsp');
    expect(p('2 tablespoons of butter').rest).toBe('butter');
    expect(p('1 lb. ground beef').unit?.key).toBe('lb');
  });
  it('reads ranges, alternates and container sizes', () => {
    const a = p('2-3 tablespoons olive oil');
    expect([a.qty, a.qtyMax, a.unit?.key]).toEqual([2, 3, 'tbsp']);
    expect(p('2 to 3 tbsp oil').qtyMax).toBe(3);
    const b = p('2 cups (240 g) flour');
    expect([b.alt?.qty, b.alt?.unit.key, b.rest]).toEqual([240, 'g', 'flour']);
    const c = p('1 (14.5-ounce) can diced tomatoes');
    expect([c.qty, c.unit?.key, c.size, c.rest]).toEqual([1, 'can', '(14.5-ounce)', 'diced tomatoes']);
    const d = p('1 stick (113 g) butter');
    expect([d.unit?.key, d.alt?.qty, d.rest]).toEqual(['stick', 113, 'butter']);
    const e = p('a pinch of salt');
    expect([e.qty, e.unit?.key, e.rest]).toEqual([1, 'pinch', 'salt']);
    expect(p('200g/7oz dark chocolate').alt?.unit.key).toBe('oz');
    expect(p('2 x 400g tins chopped tomatoes').unit?.key).toBe('tin');
    expect(p('about 4 cups water').qty).toBe(4);
    expect(p('4 chicken thighs').unit).toBe(null);
  });
});

describe('display', () => {
  it('keeps the author’s wording when nothing changes', () => {
    expect(s('2 1/2 cups all-purpose flour, sifted')).toBe('2½ cups all-purpose flour, sifted');
    expect(s('1/2 tsp salt', { fractions: false })).toBe('1/2 tsp salt');
    expect(s('a pinch of salt')).toBe('a pinch salt');
  });
  it('scales', () => {
    expect(s('2 1/2 cups flour', { scale: 2 })).toBe('5 cups flour');
    expect(s('1 cup sugar', { scale: 0.5 })).toBe('½ cup sugar');
    expect(s('1 tsp salt', { scale: 3 })).toBe('1 tbsp salt');
    expect(s('3 large eggs', { scale: 1.5 })).toBe('4½ large eggs');
    expect(s('1 clove garlic', { scale: 3 })).toBe('3 cloves garlic');
    expect(s('1 egg', { scale: 2 })).toBe('2 eggs');
    expect(s('1 large egg', { scale: 2 })).toBe('2 large eggs');
    expect(s('2 large eggs', { scale: 0.5 })).toBe('1 large egg');
    expect(s('2 eggs, beaten', { scale: 1.5 })).toBe('3 eggs, beaten');
    expect(s('113 g butter', { scale: 2 })).toBe('226 g butter');
    expect(s('1/2 cup (117 g) starter', { system: 'metric', weigh: true })).toBe('117 g starter');
    expect(s('500 g flour', { scale: 1.5 })).toBe('750 g flour');
    expect(s('3 tbsp butter', { scale: 1.5 })).toBe('4½ tbsp butter');
    expect(s('1/4 cup oil', { scale: 2 })).toBe('½ cup oil');
    expect(s('1 tbsp sugar', { scale: 0.5 })).toBe('1½ tsp sugar');
    expect(s('1 (14.5-ounce) can tomatoes', { scale: 2 })).toBe('2 (14.5-ounce) cans tomatoes');
    expect(s('2-3 tbsp oil', { scale: 2 })).toBe('4–6 tbsp oil');
    expect(s('2 cups (240 g) flour', { scale: 2 })).toBe('4 cups (480 g) flour');
  });
  it('converts to metric', () => {
    expect(s('1 lb ground beef', { system: 'metric' })).toBe('455 g ground beef');
    expect(s('2 cups milk', { system: 'metric' })).toBe('475 ml milk');
    expect(s('1 tbsp soy sauce', { system: 'metric' })).toBe('1 tbsp soy sauce');
    expect(s('2 cups (240 g) flour', { system: 'metric', weigh: true })).toBe('240 g flour');
    expect(s('1 cup all-purpose flour', { system: 'metric', weigh: true })).toBe('120 g all-purpose flour');
    expect(s('8 oz cream cheese', { system: 'metric' })).toBe('225 g cream cheese');
  });
  it('converts to US', () => {
    expect(s('500 g beef mince', { system: 'us' })).toBe('1 lb beef mince');
    expect(s('250 ml milk', { system: 'us' })).toBe('1 cup milk');
    expect(s('120 g flour', { system: 'us' })).toBe('1 cup flour');
    expect(s('120 g flour', { system: 'us', weigh: true })).toBe('4¼ oz flour');
    expect(s('15 ml lemon juice', { system: 'us' })).toBe('1 tbsp lemon juice');
    expect(s('200g/7oz dark chocolate', { system: 'us' })).toBe('7 oz dark chocolate');
  });
});

describe('baker’s percentages', () => {
  it('computes against total flour', () => {
    const r = bakers(['500 g bread flour', '375 g water', '10 g salt', '100 g sourdough starter'].map(p));
    expect(Math.round(r.flour)).toBe(550);
    expect(Math.round(r.rows[1].percent!)).toBe(68);
    expect(Math.round(r.hydration!)).toBe(77);
  });
});

describe('steps', () => {
  it('finds timers', () => {
    const t = tokenize('Bake for 25 minutes, then rest 1 hour 30 minutes. Stir for a minute.').filter((x) => x.t === 'timer') as any[];
    expect(t.map((x) => x.seconds)).toEqual([1500, 5400, 60]);
    expect((tokenize('Simmer 10-15 min').find((x) => x.t === 'timer') as any).seconds).toBe(600);
    expect((tokenize('Chill for 1½ hours').find((x) => x.t === 'timer') as any).seconds).toBe(5400);
    expect(tokenize('Add a second bowl').some((x) => x.t === 'timer')).toBe(false);
  });
  it('converts temperatures', () => {
    const t = tokenize('Heat the oven to 350°F.', 'C').find((x) => x.t === 'temp') as any;
    expect(t.shown).toBe('175°C');
    const u = tokenize('Bake at 200°C until golden', 'F').find((x) => x.t === 'temp') as any;
    expect(u.shown).toBe('400°F');
    const v = tokenize('Heat to 350°F (175°C).', 'C').filter((x) => x.t === 'temp') as any[];
    expect(v[0].converted).toBe(false);
  });
});
