import type { System } from './ingredients';

export interface Line { text: string; h?: boolean }

export interface Recipe {
  id: number; slug: string; title: string; description: string; image: string; imageColor: string;
  sourceUrl: string; sourceName: string; author: string; yield: string; servings: number;
  prepMin: number; cookMin: number; totalMin: number; ingredients: Line[]; steps: Line[]; notes: string; tags: string[];
  nutrition?: Record<string, string>; extra?: { flour?: Record<string, boolean>; baker?: boolean } | null;
  rating: number; favorite: boolean; cookedCount: number; lastCooked: number; shareToken?: string; cookbooks: number[];
  createdAt: number; updatedAt: number;
}

export interface Card {
  id: number; slug: string; title: string; image: string; color: string; tags: string[]; total: number; fav: boolean;
  rating: number; cooked: number; lastCooked: number; source: string; created: number; updated: number;
}

export interface Cookbook {
  id: number; title: string; subtitle: string; description: string; color: string; style: string; chapters: string[];
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
}

export const DEFAULT_PREFS: Prefs = { system: 'original', weigh: false, fractions: true, temp: 'original', theme: 'auto', awake: true, baker: true, docStyle: 'classic', paper: 'letter' };
