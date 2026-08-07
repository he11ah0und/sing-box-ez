/**
 * Portal action — moves the element to document.body so it renders above
 * everything else (e.g. nested inside scrollable containers).
 */
export function portal(node: HTMLElement) {
  document.body.appendChild(node);

  return {
    destroy() {
      if (node.parentNode) {
        node.parentNode.removeChild(node);
      }
    }
  };
}
