export { default as ArrowDown } from '@lucide/svelte/icons/arrow-down';
export { default as ArrowLeft } from '@lucide/svelte/icons/arrow-left';
export { default as ArrowUpDown } from '@lucide/svelte/icons/arrow-up-down';
export { default as ArrowUp } from '@lucide/svelte/icons/arrow-up';
export { default as Bell } from '@lucide/svelte/icons/bell-ring';
export { default as BookOpen } from '@lucide/svelte/icons/book-open';
export { default as BookPlus } from '@lucide/svelte/icons/book-plus';
export { default as Bot } from '@lucide/svelte/icons/bot';
export { default as Camera } from '@lucide/svelte/icons/camera';
export { default as Check } from '@lucide/svelte/icons/check';
export { default as ChefHat } from '@lucide/svelte/icons/chef-hat';
export { default as ChevronDown } from '@lucide/svelte/icons/chevron-down';
export { default as ChevronRight } from '@lucide/svelte/icons/chevron-right';
export { default as Clock } from '@lucide/svelte/icons/clock';
export { default as ClipboardPaste } from '@lucide/svelte/icons/clipboard-paste';
export { default as CookingPot } from '@lucide/svelte/icons/cooking-pot';
export { default as Copy } from '@lucide/svelte/icons/copy';
export { default as Dices } from '@lucide/svelte/icons/dices';
export { default as Download } from '@lucide/svelte/icons/download';
export { default as Ellipsis } from '@lucide/svelte/icons/ellipsis';
export { default as ExternalLink } from '@lucide/svelte/icons/external-link';
export { default as Eye } from '@lucide/svelte/icons/eye';
export { default as EyeOff } from '@lucide/svelte/icons/eye-off';
export { default as FileText } from '@lucide/svelte/icons/file-text';
export { default as Globe } from '@lucide/svelte/icons/globe';
export { default as Heart } from '@lucide/svelte/icons/heart';
export { default as ImagePlus } from '@lucide/svelte/icons/image-plus';
export { default as KeyRound } from '@lucide/svelte/icons/key-round';
export { default as Link } from '@lucide/svelte/icons/link';
export { default as ListPlus } from '@lucide/svelte/icons/list-plus';
export { default as Loader } from '@lucide/svelte/icons/loader-circle';
export { default as LogOut } from '@lucide/svelte/icons/log-out';
export { default as Minus } from '@lucide/svelte/icons/minus';
export { default as Pause } from '@lucide/svelte/icons/pause';
export { default as Pencil } from '@lucide/svelte/icons/pencil';
export { default as Percent } from '@lucide/svelte/icons/percent';
export { default as Play } from '@lucide/svelte/icons/play';
export { default as Plus } from '@lucide/svelte/icons/plus';
export { default as Printer } from '@lucide/svelte/icons/printer';
export { default as RotateCcw } from '@lucide/svelte/icons/rotate-ccw';
export { default as Search } from '@lucide/svelte/icons/search';
export { default as Settings } from '@lucide/svelte/icons/settings';
export { default as Share } from '@lucide/svelte/icons/share';
export { default as ShieldCheck } from '@lucide/svelte/icons/shield-check';
export { default as ShoppingBasket } from '@lucide/svelte/icons/shopping-basket';
export { default as Smartphone } from '@lucide/svelte/icons/smartphone';
export { default as Sparkles } from '@lucide/svelte/icons/sparkles';
export { default as Star } from '@lucide/svelte/icons/star';
export { default as Timer } from '@lucide/svelte/icons/timer';
export { default as Trash } from '@lucide/svelte/icons/trash';
export { default as Upload } from '@lucide/svelte/icons/upload';
export { default as Users } from '@lucide/svelte/icons/users';
export { default as Utensils } from '@lucide/svelte/icons/utensils';
export { default as Wheat } from '@lucide/svelte/icons/wheat';
export { default as X } from '@lucide/svelte/icons/x';

// Food glyphs, used when a recipe has no photo.
import Apple from '@lucide/svelte/icons/apple';
import Beef from '@lucide/svelte/icons/beef';
import Beer from '@lucide/svelte/icons/beer';
import CakeSlice from '@lucide/svelte/icons/cake-slice';
import Carrot from '@lucide/svelte/icons/carrot';
import Citrus from '@lucide/svelte/icons/citrus';
import Coffee from '@lucide/svelte/icons/coffee';
import Cookie from '@lucide/svelte/icons/cookie';
import Croissant from '@lucide/svelte/icons/croissant';
import Dessert from '@lucide/svelte/icons/dessert';
import Donut from '@lucide/svelte/icons/donut';
import Drumstick from '@lucide/svelte/icons/drumstick';
import EggFried from '@lucide/svelte/icons/egg-fried';
import Fish from '@lucide/svelte/icons/fish';
import Ham from '@lucide/svelte/icons/ham';
import IceCream from '@lucide/svelte/icons/ice-cream-cone';
import LeafyGreen from '@lucide/svelte/icons/leafy-green';
import Martini from '@lucide/svelte/icons/martini';
import Pizza from '@lucide/svelte/icons/pizza';
import Salad from '@lucide/svelte/icons/salad';
import Sandwich from '@lucide/svelte/icons/sandwich';
import Shrimp from '@lucide/svelte/icons/shrimp';
import Soup from '@lucide/svelte/icons/soup';
import UtensilsIcon from '@lucide/svelte/icons/utensils';
import WheatIcon from '@lucide/svelte/icons/wheat';
import Bean from '@lucide/svelte/icons/bean';

const FOOD: [RegExp, any][] = [
  [/pizza|flatbread|focaccia/, Pizza], [/cookie|biscotti|shortbread|snickerdoodle/, Cookie], [/donut|doughnut|bagel/, Donut],
  [/croissant|pastry|danish|scone|brioche/, Croissant], [/bread|loaf|sourdough|baguette|roll|bun|dough|tortilla|naan|pita/, WheatIcon],
  [/ice cream|gelato|sorbet|popsicle/, IceCream], [/cake|brownie|cupcake|muffin|cheesecake|torte|blondie/, CakeSlice],
  [/pudding|custard|mousse|tiramisu|panna cotta|trifle|dessert|pie|tart|crumble|cobbler/, Dessert],
  [/cocktail|margarita|martini|negroni|spritz|sangria|mojito/, Martini], [/beer|ale\b/, Beer], [/coffee|latte|espresso|tea\b|chai|cocoa|hot chocolate/, Coffee],
  [/smoothie|lemonade|juice|lemon|lime|orange|citrus/, Citrus], [/soup|stew|chili|chowder|broth|ramen|pho|curry|dal|bisque|gumbo|risotto/, Soup],
  [/salad|slaw|bowl/, Salad], [/sandwich|burger|wrap|taco|burrito|quesadilla|panini|toast|slider|hot dog/, Sandwich],
  [/shrimp|prawn|scampi|lobster|crab|scallop|clam|mussel/, Shrimp], [/fish|salmon|tuna|cod|trout|halibut|tilapia|seafood|sushi/, Fish],
  [/chicken|turkey|wings|drumstick|duck|poultry/, Drumstick], [/beef|steak|brisket|meatball|meatloaf|lamb|ribs|roast|short rib/, Beef],
  [/pork|ham|bacon|sausage|carnitas|prosciutto/, Ham], [/egg|omelet|frittata|quiche|scramble|benedict|shakshuka|pancake|waffle|french toast|crepe/, EggFried],
  [/bean|lentil|chickpea|hummus|falafel|tofu|tempeh/, Bean], [/apple|pear|peach|berry|fruit|plum|cherry|banana|mango|jam|compote/, Apple],
  [/carrot|squash|pumpkin|potato|beet|root|parsnip|sweet potato|fries/, Carrot],
  [/kale|spinach|broccoli|greens|vegetable|veggie|zucchini|asparagus|cabbage|cauliflower|brussels|pesto|herb/, LeafyGreen],
];

export function foodIcon(title: string, tags: string[] = []) {
  const hay = (title + ' ' + tags.join(' ')).toLowerCase();
  for (const [re, icon] of FOOD) if (re.test(hay)) return icon;
  return UtensilsIcon;
}

const TINTS = ['#e7eee6', '#fbf2cb', '#f6e3d7', '#e3ecf1', '#efe6f0', '#e9eedb', '#f8e9c9', '#e0efe9'];
const TINTS_DARK = ['#1f2c25', '#3a3418', '#3a2a20', '#1f2b33', '#2e2433', '#2a2f1c', '#3a2f17', '#1b302a'];
export function tint(seed: string, dark: boolean) {
  let h = 0;
  for (let i = 0; i < seed.length; i++) h = (h * 31 + seed.charCodeAt(i)) >>> 0;
  return (dark ? TINTS_DARK : TINTS)[h % TINTS.length];
}
