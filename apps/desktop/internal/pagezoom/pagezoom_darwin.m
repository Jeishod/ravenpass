//go:build darwin && cgo

#import <AppKit/AppKit.h>
#import <WebKit/WebKit.h>

#include "pagezoom_darwin.h"

static WKWebView *ravenpass_find_web_view(NSView *view) {
    if ([view isKindOfClass:[WKWebView class]]) {
        return (WKWebView *)view;
    }
    for (NSView *child in view.subviews) {
        WKWebView *found = ravenpass_find_web_view(child);
        if (found != nil) {
            return found;
        }
    }
    return nil;
}

static WKWebView *ravenpass_web_view(void *window) {
    if (window == NULL) {
        return nil;
    }
    return ravenpass_find_web_view([(NSWindow *)window contentView]);
}

double ravenpass_page_zoom(void *window) {
    WKWebView *webView = ravenpass_web_view(window);
    return webView == nil ? 1 : webView.pageZoom;
}

void ravenpass_set_page_zoom(void *window, double level) {
    WKWebView *webView = ravenpass_web_view(window);
    if (webView == nil) {
        return;
    }
    webView.magnification = 1;
    webView.pageZoom = level;
}
