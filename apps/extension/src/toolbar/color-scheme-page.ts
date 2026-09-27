const pageUrl = "pages/color-scheme.html";

/** Offscreen page that reports Chrome's colour scheme; a service worker has no matchMedia. */
export class ColorSchemePage {
  private opening: Promise<void> | null = null;

  open(): Promise<void> {
    this.opening ??= this.openOnce().finally(() => {
      this.opening = null;
    });
    return this.opening;
  }

  private async openOnce(): Promise<void> {
    const pages = await chrome.runtime.getContexts({
      contextTypes: ["OFFSCREEN_DOCUMENT"],
      documentUrls: [chrome.runtime.getURL(pageUrl)],
    });
    if (pages.length > 0) return;
    await chrome.offscreen.createDocument({
      url: pageUrl,
      reasons: ["MATCH_MEDIA"],
      justification: "Matches the toolbar icon to Chrome's light or dark mode.",
    });
  }
}
