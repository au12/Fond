import type { Fix, System } from './ingredients';

export interface Line { text: string; h?: boolean }

export interface Recipe {
  id: number; slug: string; title: string; description: string; image: string; imageColor: string;
  sourceUrl: string; sourceName: string; author: string; yield: string; servings: number;
  prepMin: number; cookMin: number; totalMin: number; ingredients: Line[]; steps: Line[]; notes: string; tags: string[];
  nutrition?: Record<string, string>; extra?: Extra | null;
  rating: number; favorite: boolean; cookedCount: number; lastCooked: number; shareToken?: string; cookbooks: number[];
  createdAt: number; updatedAt: number;
  /** Counts the saves that really changed the ingredients or the steps. */
  version: number;
  /** Sent with a save to say what it did; kept in the history. */
  versionNote?: string;
}

/** Things Fond remembers about one recipe beyond its text. */
export interface Extra {
  /** Lines counted (or not counted) as flour in baker's percentages, by the line as written. */
  flour?: Record<string, boolean>;
  baker?: boolean;
  /** Corrections to how single lines are weighed, by the line as written. */
  fixes?: Record<string, Fix>;
  /** Which kosher salt this recipe means, when the cook has said. */
  salt?: 'dc' | 'morton';
  /** Hydration of the starter or preferment, in percent. */
  starter?: number;
}

/** One saved state of a recipe's ingredients and steps. */
export interface Version {
  version: number; title: string; yield: string; servings: number; ingredients: Line[]; steps: Line[]; note: string; user: string; at: number;
}

export interface Card {
  id: number; slug: string; title: string; image: string; color: string; tags: string[]; total: number; fav: boolean;
  rating: number; cooked: number; lastCooked: number; source: string; created: number; updated: number;
}

export interface Cookbook {
  id: number; title: string; subtitle: string; description: string; color: string; style: string; chapters: string[];
  /** The seed the cover's leather is drawn from. It stays the same until someone picks another. */
  leather: number;
  shareToken?: string; count: number; covers: string[]; entries?: { recipeId: number; chapter: string }[];
}

export interface User { id: number; username: string; name: string; role: string; prefs: Partial<Prefs>; createdAt: number }

export interface Prefs {
  system: System;
  weigh: boolean;
  fractions: boolean;
  temp: 'original' | 'C' | 'F';
  theme: 'auto' | 'light' | 'dark';
  awake: boolean;
  baker: boolean;
  docStyle: 'classic' | 'modern' | 'card';
  paper: 'letter' | 'a4';
  /** How a cup of flour is filled: spooned and levelled (120 g) or dipped and swept (142 g). */
  flour: 'spoon' | 'dip';
  /** One-time notes this person has already read. */
  seen: { smallWeights?: boolean };
}

export const DEFAULT_PREFS: Prefs = {
  system: 'original', weigh: false, fractions: true, temp: 'original', theme: 'auto', awake: true, baker: true, docStyle: 'classic', paper: 'letter',
  flour: 'spoon', seen: {},
};
