#include "main.h"

char *ReadClipboard(void) {
    __block char *result = NULL;
    void (^read)(void) = ^{
        NSString *value = [[NSPasteboard generalPasteboard] stringForType:NSPasteboardTypeString];
        if (value) result = strdup(value.UTF8String);
    };
    if ([NSThread isMainThread]) read(); else dispatch_sync(dispatch_get_main_queue(), read);
    return result;
}

void CopyText(char *text) {
    NSString *value = [NSString stringWithUTF8String:text ?: ""];
    dispatch_async(dispatch_get_main_queue(), ^{
        [[NSPasteboard generalPasteboard] clearContents];
        [[NSPasteboard generalPasteboard] setString:value forType:NSPasteboardTypeString];
    });
}

void ShareURL(char *url) {
    NSString *value = [NSString stringWithUTF8String:url ?: ""];
    dispatch_async(dispatch_get_main_queue(), ^{
        NSURL *link = [NSURL URLWithString:value];
        if (!link) return;
        NSWindow *window = NSApp.keyWindow;
        if (!window) {
            window = [[NSPanel alloc] initWithContentRect:NSMakeRect(0,0,320,80) styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskClosable backing:NSBackingStoreBuffered defer:NO];
            window.title = @"Share link";
            [window center];
            [window makeKeyAndOrderFront:nil];
        }
        NSSharingServicePicker *picker = [[NSSharingServicePicker alloc] initWithItems:@[link]];
        [picker showRelativeToRect:window.contentView.bounds ofView:window.contentView preferredEdge:NSRectEdgeMinY];
    });
}
