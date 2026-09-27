/** WKWebView offers Reload and image downloads by default; Wails v3 beta.23 hides that menu only on Windows. */
export function limitContextMenu(): void {
  document.addEventListener("contextmenu", (event) => {
    const target = event.target;
    const editable =
      target instanceof HTMLInputElement ||
      target instanceof HTMLTextAreaElement ||
      (target instanceof HTMLElement && target.isContentEditable);
    const selected = document.getSelection()?.isCollapsed === false;
    if (!editable && !selected) {
      event.preventDefault();
    }
  });
}
