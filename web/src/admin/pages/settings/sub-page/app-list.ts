// A page's own lists of a platform's apps (config.apps, PageAppList): the order and the hidden
// ones, by the apps' ids. The editor writes them with the ids of the apps there are, so an app
// deleted since leaves them with the next change, and they keep within what the API takes
// (subpage.MaxAppList).

/** The hidden ones of hidden with app id shown or not, of the apps there are (ids) alone. */
export function hiddenWith(hidden: readonly string[], ids: readonly string[], id: string, shown: boolean): string[] {
  const others = hidden.filter((x) => x !== id && ids.includes(x));
  return shown ? others : [...others, id];
}
