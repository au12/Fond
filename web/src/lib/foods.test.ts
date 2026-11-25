import { afterEach, describe, expect, it } from 'vitest';
import foods from '../data/foods.json';
import { allFoods, loadFoods, matchFood, readName, searchFoods, setRules } from './foods';
import { parseIngredient as p, show, weigh, withWeight, type DisplayOptions, type WeighOptions } from './ingredients';

loadFoods(foods as any);

const id = (name: string) => matchFood(name).food?.id ?? null;
const g = (line: string, o: WeighOptions = {}) => {
  const w = weigh(p(line), o);
  return w ? Math.round(w.g * 10) / 10 : null;
};
const approx = (line: string, o: WeighOptions = {}) => !!weigh(p(line), o)?.approx;
const base: DisplayOptions = { scale: 1, system: 'original', weigh: true, fractions: true };
const s = (line: string, o: Partial<DisplayOptions> = {}) => {
  const r = show(p(line), { ...base, ...o });
  return `${r.measure} ${r.rest}`.trim() + (r.also ? ` [${r.approx ? '≈' : ''}${r.also}]` : '');
};

afterEach(() => setRules([]));

describe('the dictionary', () => {
  it('is well formed', () => {
    const all = allFoods();
    expect(all.length).toBeGreaterThan(200);
    const names = new Set<string>();
    for (const f of all) {
      expect(f.id).toMatch(/^[a-z0-9-]+$/);
      // Everything can be weighed some way, or is marked as never weighed.
      expect(f.d != null || !!f.s || !!f.e || !!f.z, f.id).toBe(true);
      if (f.d != null) expect(f.d, f.id).toBeGreaterThan(0.02);
      if (f.d != null) expect(f.d, f.id).toBeLessThan(1.6);
      for (const n of [f.n, ...(f.a ?? [])]) {
        const key = n.split('@')[0];
        expect(names.has(key), `${key} is listed twice`).toBe(false);
        names.add(key);
      }
    }
  });
  it('uses the agreed staples', () => {
    expect(g('1 cup all-purpose flour')).toBe(120);
    expect(g('1 cup bread flour')).toBe(120);
    expect(g('1 cup whole wheat flour')).toBeCloseTo(113, 0);
    expect(g('1 cup granulated sugar')).toBeCloseTo(198.7, 0);
    expect(g('1 cup water')).toBeCloseTo(237, 0);
    expect(g('1 cup butter')).toBeCloseTo(227, 0);
    expect(g('1 teaspoon table salt')).toBe(6);
    expect(g('1 teaspoon instant yeast')).toBe(3.1);
    expect(g('1 teaspoon baking powder')).toBe(4.6);
  });
});

describe('reading a name', () => {
  it('separates the name from its notes', () => {
    expect(readName('all-purpose flour, sifted')).toMatchObject({ name: 'all-purpose flour', notes: ', sifted' });
    expect(readName('unsalted butter (at room temperature)').name).toBe('unsalted butter');
    expect(readName('olive oil from the sun-dried tomato jar').name).toBe('olive oil');
    expect(readName('whole tomatoes with juice').name).toBe('whole tomatoes');
  });
  it('lets a name made of describing words run past the comma', () => {
    expect(id('chopped, toasted walnuts')).toBe('walnuts');
    expect(id('boneless, skinless cooked chicken')).toBe('chicken-cooked');
    expect(id('medium-sized (about 1 cup) button mushrooms')).toBe('mushroom');
  });
  it('reads states and sizes', () => {
    expect(matchFood('onion, finely chopped').state).toBe('chopped');
    expect(matchFood('extra-large eggs').size).toBe('xl');
    expect(matchFood('loosely packed brown sugar').state).toBe('loose');
  });
});

describe('matching', () => {
  it('goes by the head noun, not by words that happen to appear', () => {
    expect(id('boiling water')).toBe('water');
    expect(id('butternut squash')).toBe('winter-squash');
    expect(id('sugar snap peas')).toBe('green-beans');
    expect(id('milk chocolate chips')).toBe('chocolate-chips');
    expect(id('brown rice syrup')).toBe('rice-syrup');
    expect(id('water, for the sugar syrup')).toBe('water');
    expect(id('peanut butter')).toBe('peanut-butter');
    expect(id('coconut milk')).toBe('coconut-milk');
    expect(id('cream cheese')).toBe('cream-cheese');
    expect(id('almond flour')).toBe('flour-almond');
    expect(id('garlic powder')).toBe('garlic-powder');
    expect(id('lemon juice')).toBe('lemon-juice');
    expect(id('tomato paste')).toBe('tomato-paste');
    expect(id('rice vinegar')).toBe('vinegar');
    expect(id('corn starch')).toBe('cornstarch');
    expect(id('egg whites')).toBe('egg-white');
  });
  it('accepts unknown describing words only where they are harmless', () => {
    expect(id('vidalia onion')).toBe('onion');
    expect(id('organic free-range eggs')).toBe('egg');
    expect(id('Kerrygold unsalted butter')).toBe('butter');
    // Butter is not a loose match: an unknown word might be a different thing altogether.
    expect(id('apple butter')).not.toBe('butter');
    expect(id('cocoa butter')).toBe(null);
  });
  it('keeps cooked, dried and powdered things apart', () => {
    expect(id('rice')).toBe('rice');
    expect(id('cooked rice')).toBe('rice-cooked');
    expect(id('steamed white rice')).toBe('rice-cooked');
    expect(id('cooked quinoa')).toBe('rice-cooked');
    expect(id('quinoa')).toBe('quinoa');
    expect(id('basil')).toBe('basil');
    expect(id('dried basil')).toBe('herbs-dried');
    expect(id('milk')).toBe('milk');
    expect(id('powdered milk')).toBe('milk-powder');
    expect(id('sweetened condensed milk')).toBe('milk-condensed');
    expect(id('evaporated milk')).toBe('milk-evaporated');
    expect(id('ginger')).toBe('ginger');
    expect(id('ground ginger')).toBe('ginger-ground');
    expect(id('dried shiitake')).toBe(null);
    expect(id('fried shallots')).toBe(null);
    expect(id('hash brown potatoes')).toBe(null);
  });
  it('does not match a line that names another food as well', () => {
    expect(id('pepper corns')).toBe(null);
    expect(id('macaroni and cheese')).toBe(null);
    expect(id('garlic butter')).toBe(null);
    expect(id('salt and pepper')).toBe(null);
    // ...unless the food is a declared catch-all.
    expect(id('garlic herb seasoning')).toBe('seasoning');
    expect(id('raspberry extract')).toBe('extract');
    expect(id('cherry jam')).toBe('jam');
  });
  it('weighs the first of two choices', () => {
    expect(id('butter or margarine')).toBe('butter');
    expect(id('vegetable or canola oil')).toBe('oil');
    expect(id('sherry or apple cider vinegar')).toBe('vinegar');
    expect(id('honey or maple syrup')).toBe('honey');
    expect(id('milk/water')).toBe('milk');
    expect(id('ghee or 1 tablespoon butter')).toBe('ghee');
  });
  it('treats a second measurement as a second ingredient', () => {
    expect(g('1 tablespoon cornstarch + 1 tablespoon water')).toBe(7);
    expect(id('olive oil 1 cup water')).toBe(null);
    // A stray unit before the name is only a format quirk.
    expect(id('/ 200 g all-purpose flour')).toBe('flour-ap');
    expect(id('15 oz black beans')).toBe('beans');
  });
  it('fixes obvious typos but never invents a food', () => {
    expect(id('backing powder')).toBe('baking-powder');
    expect(id('tumeric')).toBe('turmeric');
    expect(id('pasrley')).toBe('parsley');
    expect(approx('1 tsp tumeric')).toBe(true);
    expect(id('salmon fillet')).toBe(null);
    expect(id('pita pockets')).toBe(null);
    expect(id('lady fingers')).toBe(null);
  });
  it('knows which diet labels matter', () => {
    expect(id('gluten-free rolled oats')).toBe('oats');
    expect(id('gluten-free all-purpose flour')).toBe('flour-gf');
    expect(id('low-sodium soy sauce')).toBe('soy-sauce');
    expect(id('no-salt-added tomato sauce')).toBe('tomato-sauce');
    expect(id('dairy-free milk')).toBe('milk');
    expect(id('sugar-free maple syrup')).toBe(null);
  });
  it('marks things nobody weighs', () => {
    expect(matchFood('bay leaves').skip).toBe(true);
    expect(matchFood('cinnamon stick').skip).toBe(true);
    expect(matchFood('flour, for dusting').skip).toBe(true);
    expect(matchFood('vegetable oil, for frying').skip).toBe(true);
    expect(matchFood('flour, plus more for dusting').skip).toBe(false);
    expect(matchFood('salt, or to taste').skip).toBe(false);
  });
});

describe('weighing', () => {
  it('weighs eggs by size', () => {
    expect(g('1 egg')).toBe(50);
    expect(g('3 large eggs')).toBe(150);
    expect(g('2 medium eggs')).toBe(88);
    expect(g('1 extra-large egg')).toBe(56);
    expect(g('2 jumbo eggs')).toBe(126);
    expect(g('3 egg yolks')).toBe(51);
    expect(g('2 egg whites')).toBe(66);
    expect(approx('3 large eggs')).toBe(false);
  });
  it('weighs sticks, cloves, packets and cans', () => {
    expect(g('1 stick butter')).toBe(113);
    expect(g('1/2 stick unsalted butter, softened')).toBe(56.5);
    expect(g('2 sticks (226 g) butter')).toBe(226);
    expect(g('2 cloves garlic, minced')).toBe(6);
    expect(g('3 garlic cloves')).toBe(9);
    expect(g('1 packet active dry yeast')).toBe(7);
    expect(g('1 (14.5-ounce) can diced tomatoes')).toBeCloseTo(411.1, 1);
    expect(g('2 x 400g tins chopped tomatoes')).toBe(800);
    expect(g('2 stalks celery')).toBe(80);
  });
  it('adds and subtracts compound amounts', () => {
    expect(g('1 cup plus 2 tablespoons water')).toBeCloseTo(266.4, 1);
    expect(g('1 cup minus 1 tablespoon milk')).toBeCloseTo(228.7, 0);
    expect(g('1 cup + 2 tbsp all-purpose flour')).toBe(135);
  });
  it('honours how the cup was filled', () => {
    expect(g('1 cup sliced almonds')!).toBeLessThan(g('1 cup whole almonds')!);
    expect(g('1 cup chopped walnuts')!).toBeGreaterThan(g('1 cup walnut halves')!);
    expect(g('1 cup packed brown sugar')).toBe(g('1 cup brown sugar'));
    expect(g('1 packed cup brown sugar')).toBe(g('1 cup brown sugar'));
    expect(g('1 cup shredded cheddar')).toBeCloseTo(113, 0);
    expect(g('1 cup mashed banana')!).toBeGreaterThan(g('1 cup sliced banana')!);
  });
  it('offers dip-and-sweep flour', () => {
    expect(g('1 cup all-purpose flour', { flour: 'dip' })).toBe(142);
    expect(g('1 cup sugar', { flour: 'dip' })).toBe(g('1 cup sugar'));
  });
  it('treats kosher salt as two salts', () => {
    expect(g('1 teaspoon kosher salt')).toBe(3.8);
    expect(approx('1 teaspoon kosher salt')).toBe(true);
    expect(g('1 teaspoon kosher salt', { salt: { brand: 'dc', sure: true } })).toBe(2.8);
    expect(approx('1 teaspoon kosher salt', { salt: { brand: 'dc', sure: true } })).toBe(false);
    expect(g('1 teaspoon kosher salt', { salt: { brand: 'morton', sure: false } })).toBe(4.8);
    expect(approx('1 teaspoon kosher salt', { salt: { brand: 'morton', sure: false } })).toBe(true);
    expect(g('1 teaspoon Diamond Crystal kosher salt')).toBe(2.8);
    expect(g('1 teaspoon Morton kosher salt')).toBe(4.8);
  });
  it('reads citrus either way round', () => {
    expect(g('juice of 1 lemon')).toBe(48);
    expect(g('1 lemon, juiced')).toBe(48);
    expect(g('Juice from 1/2 lime')).toBe(22);
    expect(g('1 tablespoon lemon juice')).toBeCloseTo(15.2, 1);
  });
  it('does not guess at miniature produce', () => {
    expect(g('2 carrots')).toBe(122);
    expect(g('2 baby carrots')).toBe(null);
    expect(g('5 mini cucumbers')).toBe(null);
    expect(g('1 key lime')).toBe(null);
  });
  it('calls a guess a guess', () => {
    expect(approx('1 onion')).toBe(true);
    expect(approx('1 cup all-purpose flour')).toBe(false);
    expect(approx('1 tablespoon taco seasoning')).toBe(true);
    expect(approx('1/4 cup fresh basil')).toBe(true);
  });
  it('trusts the author over arithmetic', () => {
    expect(g('2 cups (250 g) all-purpose flour')).toBe(250);
    expect(weigh(p('2 cups (250 g) all-purpose flour'))?.how).toBe('author');
    expect(g('1 cup/128 grams flour')).toBe(128);
  });
});

describe('corrections', () => {
  it('applies household rules before the dictionary', () => {
    expect(id('tiger nut flour')).toBe(null);
    setRules([{ phrase: 'tiger nut flour', action: 'map', foodId: 'flour-almond' }]);
    expect(id('tiger nut flour')).toBe('flour-almond');
    expect(matchFood('Tiger Nut Flour, sifted').via).toBe('rule');
  });
  it('files a rule under the ingredient, however it was written', () => {
    setRules([{ phrase: 'Toffee Bits', action: 'map', foodId: 'chocolate-chips' }]);
    expect(id('toffee bits')).toBe('chocolate-chips');
    expect(id('toffee bit')).toBe('chocolate-chips');
    expect(id('chilled toffee bits, roughly chopped')).toBe('chocolate-chips');
    expect(id('toffee sauce')).not.toBe('chocolate-chips');
  });
  it('takes the cook’s own measurements', () => {
    setRules([{ phrase: 'house granola', action: 'custom', gPerMl: 0.5, gEach: null }]);
    expect(g('1 cup house granola')).toBeCloseTo(118.3, 1);
    expect(approx('1 cup house granola')).toBe(false);
  });
  it('can leave an ingredient alone', () => {
    setRules([{ phrase: 'water', action: 'skip' }]);
    expect(g('1 cup water')).toBe(null);
    expect(matchFood('water').skip).toBe(true);
  });
  it('lets one recipe overrule', () => {
    const line = '1 cup mystery flour';
    expect(g(line)).toBe(null);
    expect(g(line, { fixes: { [line]: { foodId: 'flour-rye' } } })).toBe(106);
    expect(g(line, { fixes: { [line]: { gPerMl: 0.6 } } })).toBeCloseTo(142, 0);
    expect(g('1 cup water', { fixes: { '1 cup water': { skip: true } } })).toBe(null);
  });
});

describe('search', () => {
  it('finds foods by any of their names', () => {
    expect(searchFoods('flour')[0].id).toBe('flour-ap');
    expect(searchFoods('icing sugar')[0].id).toBe('sugar-powdered');
    expect(searchFoods('scallion')[0].id).toBe('green-onion');
    expect(searchFoods('zzzz')).toEqual([]);
  });
});

describe('weigh mode', () => {
  it('turns cups and spoons into grams', () => {
    expect(s('1 cup all-purpose flour')).toBe('120 g all-purpose flour');
    expect(s('1 1/2 cups granulated sugar')).toBe('298 g granulated sugar');
    expect(s('1 teaspoon baking powder')).toBe('4.6 g baking powder');
    expect(s('1/8 teaspoon instant yeast')).toBe('0.39 g instant yeast');
    expect(s('2 cups (250 g) flour')).toBe('250 g flour');
    expect(s('1 stick butter')).toBe('113 g butter');
    expect(s('1 cup flour', { scale: 2 })).toBe('240 g flour');
  });
  it('keeps spoons in US units, where ounces are too coarse', () => {
    expect(s('1 teaspoon baking powder', { system: 'us' })).toBe('1 teaspoon baking powder');
    expect(s('1 cup all-purpose flour', { system: 'us' })).toBe('4¼ oz all-purpose flour');
  });
  it('annotates counts instead of replacing them', () => {
    expect(s('3 large eggs')).toBe('3 large eggs [150 g]');
    expect(s('1 onion, diced')).toBe('1 onion, diced [≈110 g]');
    expect(s('2 cloves garlic')).toBe('2 cloves garlic [≈6 g]');
    expect(s('4 chicken thighs')).toBe('4 chicken thighs');
  });
  it('leaves what it cannot weigh as written and says so', () => {
    const r = show(p('1 cup mystery flakes'), base);
    expect([r.measure, r.measurable, r.weighed]).toEqual(['1 cup', true, false]);
    const k = show(p('1 cup flour'), base);
    expect([k.measurable, k.weighed, k.approx]).toEqual([true, true, false]);
    const b = show(p('2 bay leaves'), base);
    expect(b.measurable).toBe(false);
  });
  it('explains where a weight comes from', () => {
    expect(show(p('1 cup all-purpose flour'), base).basis).toContain('King Arthur');
    expect(show(p('2 cups (250 g) flour'), base).basis).toContain('author');
  });
  it('writes weights into a line', () => {
    expect(withWeight(p('1 cup all-purpose flour'))).toBe('1 cup (120 g) all-purpose flour');
    expect(withWeight(p('2 1/2 cups bread flour, sifted'))).toBe('2 1/2 cups (300 g) bread flour, sifted');
    expect(withWeight(p('1 cup (120 g) flour'))).toBe(null);
    expect(withWeight(p('1 tablespoon taco seasoning'))).toBe(null);
    expect(withWeight(p('3 eggs'))).toBe(null);
  });
});
