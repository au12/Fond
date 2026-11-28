// Lays a page out of sight and says whether its content stays inside the page. Shared by the book
// viewer and the print page, which measure at different sizes and settings.
import { flushSync, mount, unmount } from 'svelte';
import BookPage from './BookPage.svelte';
import type { Fits, Page } from '../../lib/book';

export function fitsIn(host: HTMLElement, props: Record<string, unknown>): Fits {
  return (r, part, chapter, n) => {
    const page: Page = { kind: 'recipe', r, n, chapter, part };
    const c = mount(BookPage, { target: host, props: { ...props, page, index: 9, side: 'right' } as any });
    flushSync();
    const inner = host.querySelector('.in') as HTMLElement | null;
    const rp = host.querySelector('.rp') as HTMLElement | null;
    let ok = true;
    if (inner && rp) {
      const box = inner.getBoundingClientRect();
      const bottom = box.bottom - parseFloat(getComputedStyle(inner).paddingBottom) - 4;
      ok = rp.getBoundingClientRect().bottom <= bottom;
    }
    unmount(c);
    return ok;
  };
}
