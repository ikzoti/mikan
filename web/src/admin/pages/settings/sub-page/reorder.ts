// How the lists of the subscription page's editor move a row a place up or down: the blocks, the
// guides, a platform's apps and an app's download buttons.

/** list with the item at i moved by step. */
export function moved<T>(list: readonly T[], i: number, step: -1 | 1): T[] {
  const next = [...list];
  const [item] = next.splice(i, 1);
  next.splice(i + step, 0, item!);
  return next;
}

/** The focus after a row moved by step: the button pressed may be gone at the list's end, so the other one of the pair, up and down by their ids, takes it. */
export function focusMoved(step: -1 | 1, up: string, down: string) {
  const [pressed, other] = step < 0 ? [up, down] : [down, up];
  window.requestAnimationFrame(() => {
    const want = document.getElementById(pressed) as HTMLButtonElement | null;
    (want && !want.disabled ? want : document.getElementById(other))?.focus();
  });
}
