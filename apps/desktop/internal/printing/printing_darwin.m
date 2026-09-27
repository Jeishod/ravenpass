//go:build darwin && cgo

#import <AppKit/AppKit.h>
#import <WebKit/WebKit.h>

#include "printing_darwin.h"

typedef void (^ravenpass_print_finished)(int status);

// RavenpassPrintJob loads one page offscreen and prints it as a sheet on a window. Main thread only; finished runs once.
@interface RavenpassPrintJob : NSObject <WKNavigationDelegate>
- (instancetype)initWithTitle:(NSString *)title window:(NSWindow *)window finished:(ravenpass_print_finished)finished;
- (void)load:(NSString *)page;
@end

@implementation RavenpassPrintJob {
    NSString *_title;
    NSWindow *_window;
    ravenpass_print_finished _finished;
    WKWebView *_view;
    BOOL _navigated;
    BOOL _printing;
}

- (instancetype)initWithTitle:(NSString *)title window:(NSWindow *)window finished:(ravenpass_print_finished)finished {
    self = [super init];
    if (self != nil) {
        _title = title;
        _window = window;
        _finished = finished;
    }
    return self;
}

- (void)load:(NSString *)page {
    WKWebViewConfiguration *configuration = [[WKWebViewConfiguration alloc] init];
    // A non-persistent store keeps the page's caches in memory.
    configuration.websiteDataStore = [WKWebsiteDataStore nonPersistentDataStore];
    configuration.defaultWebpagePreferences.allowsContentJavaScript = NO;
    _view = [[WKWebView alloc] initWithFrame:NSZeroRect configuration:configuration];
    _view.navigationDelegate = self;
    [_view loadHTMLString:page baseURL:nil];
}

// Only the page's own load navigates.
- (void)webView:(WKWebView *)webView decidePolicyForNavigationAction:(WKNavigationAction *)action
    decisionHandler:(void (^)(WKNavigationActionPolicy))decisionHandler {
    BOOL first = !_navigated;
    _navigated = YES;
    decisionHandler(first ? WKNavigationActionPolicyAllow : WKNavigationActionPolicyCancel);
}

- (void)webView:(WKWebView *)webView didFinishNavigation:(WKNavigation *)navigation {
    if (_printing) {
        return;
    }
    _printing = YES;
    NSPrintInfo *info = [[NSPrintInfo sharedPrintInfo] copy];
    NSPrintOperation *operation = [webView printOperationWithPrintInfo:info];
    operation.jobTitle = _title;
    operation.showsPrintPanel = YES;
    operation.showsProgressPanel = YES;
    // A WKWebView print operation lays out blank pages until its view has a frame, and prints only as a sheet.
    operation.view.frame = NSMakeRect(0, 0, info.paperSize.width, info.paperSize.height);
    [operation runOperationModalForWindow:_window
                                 delegate:self
                           didRunSelector:@selector(printOperationDidRun:success:contextInfo:)
                              contextInfo:NULL];
}

- (void)webView:(WKWebView *)webView didFailNavigation:(WKNavigation *)navigation withError:(NSError *)error {
    [self failUnlessPrinting];
}

- (void)webView:(WKWebView *)webView
    didFailProvisionalNavigation:(WKNavigation *)navigation
                       withError:(NSError *)error {
    [self failUnlessPrinting];
}

- (void)webViewWebContentProcessDidTerminate:(WKWebView *)webView {
    [self failUnlessPrinting];
}

- (void)printOperationDidRun:(NSPrintOperation *)operation success:(BOOL)success contextInfo:(void *)contextInfo {
    [self finish:0];
}

- (void)failUnlessPrinting {
    if (!_printing) {
        [self finish:-1];
    }
}

- (void)finish:(int)status {
    if (_finished == nil) {
        return;
    }
    ravenpass_print_finished finished = _finished;
    _finished = nil;
    _view.navigationDelegate = nil;
    [_view stopLoading];
    [_view loadHTMLString:@"" baseURL:nil];
    _view = nil;
    _window = nil;
    finished(status);
}

@end

// The job printing now. Main thread only.
static RavenpassPrintJob *ravenpass_current_job;

int ravenpass_print_page(const char *job, size_t jobLength, const char *page, size_t pageLength) {
    if ([NSThread isMainThread]) {
        return -1;
    }
    @autoreleasepool {
        NSString *title = [[NSString alloc] initWithBytes:job length:jobLength encoding:NSUTF8StringEncoding];
        NSString *html = [[NSString alloc] initWithBytes:page length:pageLength encoding:NSUTF8StringEncoding];
        if (title == nil || html == nil) {
            return -1;
        }
        dispatch_semaphore_t closed = dispatch_semaphore_create(0);
        __block int outcome = -1;
        dispatch_async(dispatch_get_main_queue(), ^{
            NSWindow *window = NSApp.mainWindow;
            if (ravenpass_current_job != nil || window == nil) {
                dispatch_semaphore_signal(closed);
                return;
            }
            ravenpass_current_job = [[RavenpassPrintJob alloc] initWithTitle:title
                                                                      window:window
                                                                    finished:^(int status) {
                outcome = status;
                ravenpass_current_job = nil;
                dispatch_semaphore_signal(closed);
            }];
            [ravenpass_current_job load:html];
        });
        dispatch_semaphore_wait(closed, DISPATCH_TIME_FOREVER);
        return outcome;
    }
}
