#import "window.h"
#import "navigation_policy.h"

static WindowController* windowController = nil;
static NSString* htmlContent = nil;
static NSMutableDictionary* fileContents = nil;

static BOOL IsSettingsFrame(WKFrameInfo *frame) {
    return frame && FinickyIsSettingsFrame(frame.isMainFrame, frame.request.URL,
        frame.securityOrigin.protocol, frame.securityOrigin.host, frame.securityOrigin.port);
}

void SetHTMLContent(const char* content) {
    if (content) {
        htmlContent = [NSString stringWithUTF8String:content];
    }
}

void SetFileContent(const char* path, const char* content) {
    if (!fileContents) {
        fileContents = [[NSMutableDictionary alloc] init];
    }
    if (path && content) {
        NSString* pathStr = [NSString stringWithUTF8String:path];
        if ([pathStr hasSuffix:@".png"]) {
            // For PNG files, use SetFileContentWithLength instead
            SetFileContentWithLength(path, content, strlen(content));
        } else {
            // For text files, store as string
            NSString* contentStr = [NSString stringWithUTF8String:content];
            fileContents[pathStr] = contentStr;
        }
    }
}

void SetFileContentWithLength(const char* path, const char* content, size_t length) {
    if (!fileContents) {
        fileContents = [[NSMutableDictionary alloc] init];
    }
    if (path && content) {
        NSString* pathStr = [NSString stringWithUTF8String:path];
        NSData* data = [[NSData alloc] initWithBytes:content length:length];
        fileContents[pathStr] = data;
    }
}

@implementation WindowController {
    NSWindow* window;
    WKWebView* webView;
}

- (id)init {
    self = [super init];
    NSLog(@"Initialize window controller");
    if (self) {
        // Always setup window on main thread
        if ([NSThread isMainThread]) {
            [self setupWindow];
            [self setupMenu];
        } else {
            dispatch_sync(dispatch_get_main_queue(), ^{
                [self setupWindow];
                [self setupMenu];
            });
        }
    }
    return self;
}

- (void)setupWindow {
    // Create window
    window = [[NSWindow alloc] initWithContentRect:NSMakeRect(0, 0, 1120, 780)
                                       styleMask:NSWindowStyleMaskTitled |
                                                NSWindowStyleMaskClosable |
                                                NSWindowStyleMaskMiniaturizable |
                                                NSWindowStyleMaskResizable
                                         backing:NSBackingStoreBuffered
                                           defer:NO];
    [window setTitle:@"Waystile"];
    [window center];
    [window setReleasedWhenClosed:NO];
    [window setBackgroundColor:[NSColor colorWithCalibratedWhite:0.1 alpha:1.0]];

    // Set minimum window size
    [window setMinSize:NSMakeSize(760, 560)];
    [window setMaxSize:NSMakeSize(1600, 1100)];

    // Configure WKWebView
    WKWebViewConfiguration *config = [[WKWebViewConfiguration alloc] init];
    [config.userContentController addScriptMessageHandler:self name:@"finicky"];
    [config setURLSchemeHandler:self forURLScheme:@"finicky-assets"];

    // Inject a stub window.finicky at document start so the native side can
    // safely call finicky.receiveMessage() before the Svelte app has mounted.
    // Messages are buffered in _queue and drained by App.svelte at module scope.
    NSString *stubScript = @""
        "window.finicky = {"
        "  _queue: [],"
        "  receiveMessage: function(msg) { this._queue.push(msg); },"
        "  sendMessage: function(msg) {"
        "    window.webkit?.messageHandlers?.finicky?.postMessage(JSON.stringify(msg));"
        "  }"
        "};";
    WKUserScript *stubUserScript = [[WKUserScript alloc]
        initWithSource:stubScript
         injectionTime:WKUserScriptInjectionTimeAtDocumentStart
      forMainFrameOnly:YES];
    [config.userContentController addUserScript:stubUserScript];

    // Create WKWebView
    webView = [[WKWebView alloc] initWithFrame:window.contentView.bounds configuration:config];
    webView.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    webView.navigationDelegate = self;
    webView.wantsLayer = true;
    webView.layer.backgroundColor = [NSColor colorWithCalibratedWhite:0.1 alpha:1.0].CGColor;

    // Use a real bundled document URL so navigation and bridge checks have one
    // explicit origin, including during startup. No about:blank exception is
    // needed for loadHTMLString's synthetic document.
    if (htmlContent) {
        [webView loadRequest:[NSURLRequest requestWithURL:[NSURL URLWithString:@"finicky-assets://local/index.html"]]];
    } else {
        NSLog(@"Warning: HTML content not set");
    }

    // Add window close notification observer
    [[NSNotificationCenter defaultCenter] addObserver:self
                                         selector:@selector(windowWillClose:)
                                             name:NSWindowWillCloseNotification
                                           object:window];

    // Set webView as content view
    window.contentView = webView;
}

- (void)showWindow {
    if ([NSThread isMainThread]) {
        [window makeKeyAndOrderFront:nil];
        [NSApp activateIgnoringOtherApps:true];
    } else {
        dispatch_async(dispatch_get_main_queue(), ^{
            [window makeKeyAndOrderFront:nil];
            [NSApp activateIgnoringOtherApps:true];
        });
    }
}

- (void)closeWindow {
    if ([NSThread isMainThread]) {
        [window close];
    } else {
        dispatch_async(dispatch_get_main_queue(), ^{
            [window close];
        });
    }
}

- (void)sendMessageToWebView:(NSString *)message {
    // The message is already JSON encoded from Go, pass it as a string literal, but escape the quotes and backslashes
    NSString *escapedMessage = [[message stringByReplacingOccurrencesOfString:@"\\" withString:@"\\\\"]
                                        stringByReplacingOccurrencesOfString:@"\"" withString:@"\\\""];
    NSString *js = [NSString stringWithFormat:@"finicky.receiveMessage(\"%@\")", escapedMessage];

    if ([NSThread isMainThread]) {
        if (webView && !webView.loading && FinickyIsSettingsDocumentURL(webView.URL)) {
            [webView evaluateJavaScript:js completionHandler:nil];
        }
    } else {
        dispatch_async(dispatch_get_main_queue(), ^{
            if (webView && !webView.loading && FinickyIsSettingsDocumentURL(webView.URL)) {
                [webView evaluateJavaScript:js completionHandler:nil];
            }
        });
    }
}

- (void)userContentController:(WKUserContentController *)userContentController
      didReceiveScriptMessage:(WKScriptMessage *)message {
    // Never accept settings commands from a subframe or another origin, and
    // never log the body: it can contain URLs, routing rules and local settings.
    if (message.webView != webView || userContentController != webView.configuration.userContentController ||
        ![message.name isEqualToString:@"finicky"] || !IsSettingsFrame(message.frameInfo) ||
        !FinickyIsSettingsDocumentURL(webView.URL)) {
        return;
    }

    // Convert the message body to JSON string and forward to Go
    if ([message.body isKindOfClass:[NSString class]]) {
        extern void HandleWebViewMessage(const char* message);
        NSString *messageString = (NSString *)message.body;
        HandleWebViewMessage([messageString UTF8String]);
    } else if ([message.body isKindOfClass:[NSDictionary class]]) {
        NSError *error;
        NSData *jsonData = [NSJSONSerialization dataWithJSONObject:message.body options:0 error:&error];
        if (jsonData && !error) {
            NSString *jsonString = [[NSString alloc] initWithData:jsonData encoding:NSUTF8StringEncoding];
            extern void HandleWebViewMessage(const char* message);
            HandleWebViewMessage([jsonString UTF8String]);
        }
    }
}

#pragma mark - WKURLSchemeHandler

- (void)webView:(WKWebView *)webView startURLSchemeTask:(id<WKURLSchemeTask>)urlSchemeTask {
    NSURLRequest *request = urlSchemeTask.request;
    if (!FinickyIsBundledAssetURL(request.URL)) {
        [urlSchemeTask didFailWithError:[NSError errorWithDomain:NSURLErrorDomain code:NSURLErrorUnsupportedURL userInfo:nil]];
        return;
    }
    NSString *path = request.URL.path;
    if ([path hasPrefix:@"/"]) {
        path = [path substringFromIndex:1];
    }

    id content = [path isEqualToString:@"index.html"] ? htmlContent : fileContents[path];
    if (content) {
        NSData *data;
        if ([content isKindOfClass:[NSData class]]) {
            data = (NSData *)content;
        } else {
            data = [(NSString *)content dataUsingEncoding:NSUTF8StringEncoding];
        }

        NSString *mimeType = @"text/plain";
        if ([path hasSuffix:@".html"]) {
            mimeType = @"text/html";
        } else if ([path hasSuffix:@".css"]) {
            mimeType = @"text/css";
        } else if ([path hasSuffix:@".js"]) {
            mimeType = @"application/javascript";
        } else if ([path hasSuffix:@".png"]) {
            mimeType = @"image/png";
        }

        NSURLResponse *response = [[NSURLResponse alloc] initWithURL:request.URL
                                                          MIMEType:mimeType
                                             expectedContentLength:data.length
                                                  textEncodingName:[content isKindOfClass:[NSString class]] ? @"utf-8" : nil];

        [urlSchemeTask didReceiveResponse:response];
        [urlSchemeTask didReceiveData:data];
        [urlSchemeTask didFinish];
    } else {
        [urlSchemeTask didFailWithError:[NSError errorWithDomain:NSURLErrorDomain
                                                          code:NSURLErrorResourceUnavailable
                                                      userInfo:nil]];
    }
}

- (void)webView:(WKWebView *)webView stopURLSchemeTask:(id<WKURLSchemeTask>)urlSchemeTask {
    // Nothing to do here
}

#pragma mark - WKNavigationDelegate

- (void)webView:(WKWebView *)finishedWebView didFinishNavigation:(WKNavigation *)navigation {
    if (finishedWebView != webView || !FinickyIsSettingsDocumentURL(finishedWebView.URL)) return;
    // Notify Go only after the bundled settings document is ready.
    extern void WindowIsReady(void);
    WindowIsReady();
}

- (void)webView:(WKWebView *)webView decidePolicyForNavigationAction:(WKNavigationAction *)navigationAction decisionHandler:(void (^)(WKNavigationActionPolicy))decisionHandler {
    NSURL *url = navigationAction.request.URL;

    FinickySettingsNavigation policy = FinickySettingsNavigationPolicy(url,
        navigationAction.targetFrame.isMainFrame,
        navigationAction.navigationType == WKNavigationTypeLinkActivated,
        IsSettingsFrame(navigationAction.sourceFrame));
    decisionHandler(policy == FinickySettingsNavigationAllow ? WKNavigationActionPolicyAllow : WKNavigationActionPolicyCancel);
    if (policy == FinickySettingsNavigationOpenExternal) {
        // Explicit HTTP(S) clicks, including target=_blank links, leave this
        // privileged web view. Router deep links are handled by the app's
        // existing Apple event path, never by loading them as settings pages.
        [[NSWorkspace sharedWorkspace] openURL:url];
    }
}

// Add new method to handle window close
- (void)windowWillClose:(NSNotification *)notification {
    extern void WindowDidClose(void);
    WindowDidClose();
}

- (void)setupMenu {
    NSMenu *mainMenu = [[NSMenu alloc] init];
    [NSApp setMainMenu:mainMenu];

    // Application menu
    NSMenuItem *appMenuItem = [[NSMenuItem alloc] init];
    [mainMenu addItem:appMenuItem];
    NSMenu *appMenu = [[NSMenu alloc] init];
    [appMenuItem setSubmenu:appMenu];

    // Quit menu item (⌘Q)
    NSMenuItem *quitMenuItem = [[NSMenuItem alloc] initWithTitle:@"Quit"
                                                        action:@selector(terminate:)
                                                 keyEquivalent:@"q"];
    [quitMenuItem setTarget:NSApp];
    [appMenu addItem:quitMenuItem];

    // File menu
    NSMenuItem *fileMenuItem = [[NSMenuItem alloc] init];
    [mainMenu addItem:fileMenuItem];
    NSMenu *fileMenu = [[NSMenu alloc] initWithTitle:@"File"];
    [fileMenuItem setSubmenu:fileMenu];

    // Close window menu item (⌘W)
    NSMenuItem *closeMenuItem = [[NSMenuItem alloc] initWithTitle:@"Close Window"
                                                         action:@selector(performClose:)
                                                  keyEquivalent:@"w"];
    [closeMenuItem setTarget:window];
    [fileMenu addItem:closeMenuItem];

    // Edit menu
    NSMenuItem *editMenuItem = [[NSMenuItem alloc] init];
    [mainMenu addItem:editMenuItem];
    NSMenu *editMenu = [[NSMenu alloc] initWithTitle:@"Edit"];
    [editMenuItem setSubmenu:editMenu];

    // Add Cut menu item (⌘X)
    NSMenuItem *cutMenuItem = [[NSMenuItem alloc] initWithTitle:@"Cut"
                                                       action:@selector(cut:)
                                                keyEquivalent:@"x"];
    [editMenu addItem:cutMenuItem];

    // Add Copy menu item (⌘C)
    NSMenuItem *copyMenuItem = [[NSMenuItem alloc] initWithTitle:@"Copy"
                                                        action:@selector(copy:)
                                                 keyEquivalent:@"c"];
    [editMenu addItem:copyMenuItem];

    // Add Paste menu item (⌘V)
    NSMenuItem *pasteMenuItem = [[NSMenuItem alloc] initWithTitle:@"Paste"
                                                         action:@selector(paste:)
                                                  keyEquivalent:@"v"];
    [editMenu addItem:pasteMenuItem];

    // Add separator
    [editMenu addItem:[NSMenuItem separatorItem]];

    // Add Undo menu item (⌘Z)
    NSMenuItem *undoMenuItem = [[NSMenuItem alloc] initWithTitle:@"Undo"
                                                       action:@selector(undo:)
                                                keyEquivalent:@"z"];
    [editMenu addItem:undoMenuItem];

    // Add Redo menu item (⌘⇧Z)
    NSMenuItem *redoMenuItem = [[NSMenuItem alloc] initWithTitle:@"Redo"
                                                       action:@selector(redo:)
                                                keyEquivalent:@"Z"];
    [editMenu addItem:redoMenuItem];

    // Add separator
    [editMenu addItem:[NSMenuItem separatorItem]];

    // Add Select All menu item (⌘A)
    NSMenuItem *selectAllMenuItem = [[NSMenuItem alloc] initWithTitle:@"Select All"
                                                            action:@selector(selectAll:)
                                                     keyEquivalent:@"a"];
    [editMenu addItem:selectAllMenuItem];
}

@end

void ShowWindow(void) {
    if (!windowController) {
        windowController = [[WindowController alloc] init];
    }
    [windowController showWindow];
}

void CloseWindow(void) {
    if (windowController) {
        [windowController closeWindow];
    }
}

void SendMessageToWebView(const char* message) {
    if (windowController) {
        NSString *nsMessage = [NSString stringWithUTF8String:message];
        [windowController sendMessageToWebView:nsMessage];
    }
}
