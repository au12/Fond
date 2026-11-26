// Grows a textarea with its text, never below its rows.
//
// The height is measured on a hidden twin rather than by setting the textarea itself to height:auto
// and back. That collapse shortens the page for a moment, the browser pulls the scroll position up to
// fit the shorter page, and typing far down a long box kept throwing the page toward the top of it.

const COPIED = [
  'box-sizing', 'width',
  'padding-top', 'padding-right', 'padding-bottom', 'padding-left',
  'border-top-width', 'border-right-width', 'border-bottom-width', 'border-left-width',
  'font-family', 'font-size', 'font-style', 'font-weight', 'font-stretch', 'font-kerning',
  'font-variant-numeric', 'font-variant-ligatures', 'font-feature-settings', 'font-variation-settings',
  'letter-spacing', 'word-spacing', 'line-height', 'text-indent', 'text-transform', 'text-rendering',
  'tab-size', 'white-space', 'word-break', 'overflow-wrap',
];

let twin: HTMLTextAreaElement | undefined;

function measure(node: HTMLTextAreaElement): number {
  if (!twin) {
    twin = document.createElement('textarea');
    twin.setAttribute('aria-hidden', 'true');
    twin.tabIndex = -1;
    twin.spellcheck = false;
    // Fixed, so however tall it gets it never adds to the page's height.
    twin.style.cssText =
      'position:fixed;top:0;left:0;z-index:-1;visibility:hidden;pointer-events:none;overflow:hidden;height:auto;min-height:0;max-height:none;margin:0;border-style:solid';
  }
  if (!twin.isConnected) document.body.append(twin);
  const cs = getComputedStyle(node);
  for (const p of COPIED) twin.style.setProperty(p, cs.getPropertyValue(p));
  twin.rows = node.rows;
  twin.value = node.value;
  // scrollHeight is the taller of the text and the rows, padding included.
  const edge = cs.boxSizing === 'border-box'
    ? parseFloat(cs.borderTopWidth) + parseFloat(cs.borderBottomWidth)
    : -(parseFloat(cs.paddingTop) + parseFloat(cs.paddingBottom));
  return Math.ceil(twin.scrollHeight + edge);
}

export function grow(node: HTMLTextAreaElement, _value?: unknown) {
  // It always fits its text, so it has nothing to scroll, and a drag handle would only be undone.
  node.style.overflowY = 'hidden';
  node.style.resize = 'none';
  let width = -1;
  let seen = '';
  let frame = 0;
  const fit = (force = false) => {
    if (!node.isConnected) return;
    const key = width + '|' + node.value;
    if (key === seen && !force) return;
    seen = key;
    const h = measure(node) + 'px';
    if (node.style.height !== h) node.style.height = h;
  };
  // A new width rewraps the text (rotating a phone, crossing a layout breakpoint).
  const ro = new ResizeObserver(([e]) => {
    if (e.contentRect.width === width) return;
    width = e.contentRect.width;
    cancelAnimationFrame(frame);
    frame = requestAnimationFrame(() => fit());
  });
  const onInput = () => fit();
  ro.observe(node);
  node.addEventListener('input', onInput);
  queueMicrotask(onInput);
  document.fonts?.ready.then(() => fit(true));
  return {
    update: onInput,
    destroy() {
      ro.disconnect();
      cancelAnimationFrame(frame);
      node.removeEventListener('input', onInput);
    },
  };
}
