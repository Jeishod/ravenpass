/** Fires after nodes are added or removed or a hiding attribute changes; returns the unsubscribe. */
export type PageChanges = (onChange: () => void) => () => void;

const observed: MutationObserverInit = {
  childList: true,
  subtree: true,
  attributes: true,
  attributeFilter: ["type", "class", "style", "hidden"],
};

/** Observes every shadow root the node sits in: a document's observer does not see into them. */
export function changesAround(node: Node): PageChanges {
  return (onChange) => {
    const observer = new MutationObserver(() => onChange());
    let root = node.getRootNode();
    observer.observe(root, observed);
    while (root instanceof ShadowRoot) {
      root = root.host.getRootNode();
      observer.observe(root, observed);
    }
    return () => observer.disconnect();
  };
}
