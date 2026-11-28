import { describe, expect, it } from 'vitest';
import foods from '../data/foods.json';
import { loadFoods } from './foods';
import { combine, type ShopItem, type SumOptions } from './shopping';

loadFoods(foods as any);

let n = 0;
const item = (text: string, extra: Partial<ShopItem> = {}): ShopItem => ({ id: ++n, text, checked: false, recipeId: null, recipeTitle: '', group: '', ...extra });
const o: SumOptions = { system: 'original', fractions: true, weigh: false };
const texts = (list: ShopItem[], opts: Partial<SumOptions> = {}) => combine(list, { ...o, ...opts }).map((l) => `${l.approx ? '≈ ' : ''}${l.text}`).sort();

describe('the shopping list', () => {
  it('keeps a single line as written', () => {
    expect(texts([item('2 1/2 cups flour, sifted')])).toEqual(['2 1/2 cups flour, sifted']);
  });

  it('adds up the same food measured the same way', () => {
    expect(texts([item('1 cup milk'), item('1/2 cup whole milk')])).toEqual(['1½ cups milk']);
    expect(texts([item('200 g butter'), item('100 g unsalted butter')])).toEqual(['300 g butter']);
    expect(texts([item('2 large eggs'), item('1 egg')])).toEqual(['3 eggs']);
    expect(texts([item('2 cloves garlic, minced'), item('3 garlic cloves')])).toEqual(['5 cloves garlic']);
  });

  it('keeps a sum of cups and spoons exact', () => {
    expect(texts([item('1 cup unsalted butter'), item('2 tbsp butter, melted')])).toEqual(['1 cup + 2 tbsp butter']);
    expect(texts([item('1/2 cup milk'), item('1/4 cup milk')])).toEqual(['¾ cup milk']);
  });

  it('weighs different measures of one food into one weight', () => {
    expect(texts([item('1 cup all-purpose flour'), item('200 g flour')])).toEqual(['320 g flour']);
    expect(texts([item('1 stick butter'), item('2 tbsp butter')])).toEqual(['141 g butter']);
  });

  it('prefers weights when the reader does', () => {
    expect(texts([item('1 cup sugar'), item('1/4 cup sugar')], { weigh: true })).toEqual(['248 g sugar']);
  });

  it('shows the reader’s measures', () => {
    expect(texts([item('1 cup milk'), item('1 cup milk')], { system: 'metric' })).toEqual(['475 ml milk']);
    expect(texts([item('500 g flour'), item('500 g flour')], { system: 'us' })).toEqual(['2¼ lb flour']);
  });

  it('keeps different foods apart', () => {
    expect(texts([item('1 cup sugar'), item('1 cup brown sugar'), item('1 tsp salt'), item('1 tsp kosher salt')])).toHaveLength(4);
  });

  it('joins measures it cannot weigh with a plus', () => {
    expect(texts([item('1 can chickpeas'), item('200 g chickpeas')])).toEqual(['200 g + 1 can chickpeas']);
  });

  it('adds lines without amounts to the ones that have them', () => {
    expect(texts([item('salt'), item('1 tsp salt')])).toEqual(['1 tsp salt']);
  });

  it('merges by hand, and keeps lines apart when asked', () => {
    expect(texts([item('1 bunch scallions', { group: 'm:a' }), item('2 green onions', { group: 'm:a' })])).toHaveLength(1);
    expect(texts([item('1 cup milk', { group: 'solo:x' }), item('1 cup milk')])).toEqual(['1 cup milk', '1 cup milk']);
  });

  it('remembers the recipes and sorts into aisles', () => {
    const [l] = combine([item('1 onion', { recipeTitle: 'Soup' }), item('2 onions', { recipeTitle: 'Stew' })], o);
    expect(l.from).toEqual(['Soup', 'Stew']);
    expect(l.aisle).toBe('Produce');
    expect(l.text).toBe('3 onions');
    expect(combine([item('2 lb chicken thighs')], o)[0].aisle).toBe('Meat and fish');
    expect(combine([item('1 cup milk')], o)[0].aisle).toBe('Dairy and eggs');
    expect(combine([item('1 bag frozen peas')], o)[0].aisle).toBe('Frozen');
  });
});
