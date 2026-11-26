// Cookbook covers are bound in leather the server draws from a seed. Every seed is a different hide,
// and a seed always draws the same one, so a book keeps its cover until someone picks another.

/** A fresh seed: any whole number from 1 to 2³² − 1. */
export function newLeather(): number {
  return crypto.getRandomValues(new Uint32Array(1))[0] || 1;
}

/** Where the hide for a seed and cover colour is served. The small size suits a shelf; the large one a cover shown big or printed. */
export const leatherUrl = (seed: number, color: string, large = false) => `/leather/${seed}-${color}-${large ? 'l' : 's'}.jpg`;

export const COVER_COLORS = ['bay', 'tomato', 'chestnut', 'butter', 'ocean', 'plum', 'charcoal', 'paper'];
