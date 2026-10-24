#include "main.h"

// Standalone share requests have no clicked view to anchor to. Keep a small
// window alive and let its real Share button present AppKit's popover.
@interface FinickyShareController : NSObject <NSWindowDelegate, NSSharingServicePickerDelegate, NSSharingServiceDelegate>
@property(nonatomic, retain) NSPanel *panel;
@property(nonatomic, retain) NSURL *link;
@property(nonatomic, retain) NSSharingServicePicker *picker;
@property(nonatomic, retain) NSSharingService *service;
@property(nonatomic) BOOL finished;
- (void)present;
- (void)finish;
@end

static NSMutableSet *shareControllers;

@implementation FinickyShareController
- (void)present {
    NSPanel *panel = [[NSPanel alloc] initWithContentRect:NSMakeRect(0, 0, 440, 126)
        styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskClosable backing:NSBackingStoreBuffered defer:NO];
    self.panel = panel;
    [panel release];
    self.panel.title = @"Share link";
    self.panel.releasedWhenClosed = NO;
    self.panel.hidesOnDeactivate = NO;
    self.panel.delegate = self;
    self.panel.collectionBehavior = NSWindowCollectionBehaviorMoveToActiveSpace | NSWindowCollectionBehaviorFullScreenAuxiliary;
    NSTextField *label = [NSTextField wrappingLabelWithString:self.link.absoluteString];
    label.frame = NSMakeRect(20, 58, 400, 46);
    label.selectable = YES;
    label.maximumNumberOfLines = 2;
    label.lineBreakMode = NSLineBreakByTruncatingMiddle;
    [self.panel.contentView addSubview:label];
    NSButton *button = [NSButton buttonWithTitle:@"Share…" target:self action:@selector(share:)];
    button.frame = NSMakeRect(324, 18, 96, 30);
    button.bezelStyle = NSBezelStyleRounded;
    [button sendActionOn:NSEventMaskLeftMouseDown];
    [self.panel.contentView addSubview:button];
    [self.panel center];
    [NSApp activateIgnoringOtherApps:YES];
    [self.panel makeKeyAndOrderFront:nil];
}

- (void)share:(NSButton *)sender {
    if (self.finished || self.picker || self.service) return;
    NSSharingServicePicker *picker = [[NSSharingServicePicker alloc] initWithItems:@[self.link]];
    self.picker = picker;
    [picker release];
    self.picker.delegate = self;
    [self.picker showRelativeToRect:sender.bounds ofView:sender preferredEdge:NSRectEdgeMaxY];
}

- (id<NSSharingServiceDelegate>)sharingServicePicker:(NSSharingServicePicker *)picker delegateForSharingService:(NSSharingService *)service {
    self.service = service;
    return self;
}

- (void)sharingServicePicker:(NSSharingServicePicker *)picker didChooseSharingService:(NSSharingService *)service {
    if (!service) {
        self.picker.delegate = nil;
        self.picker = nil;
        self.service = nil;
    }
}

- (void)sharingService:(NSSharingService *)service didShareItems:(NSArray *)items { [self finish]; }
- (void)sharingService:(NSSharingService *)service didFailToShareItems:(NSArray *)items error:(NSError *)error {
    self.picker.delegate = nil;
    self.picker = nil;
    self.service.delegate = nil;
    self.service = nil;
    if ([error.domain isEqualToString:NSCocoaErrorDomain] && error.code == NSUserCancelledError) return;
    [[NSAlert alertWithError:error] beginSheetModalForWindow:self.panel completionHandler:nil];
}
- (void)windowWillClose:(NSNotification *)notification { [self finish]; }

- (void)finish {
    if (self.finished) return;
    self.finished = YES;
    [[self retain] autorelease];
    self.picker.delegate = nil;
    [self.picker close];
    self.picker = nil;
    self.service.delegate = nil;
    self.service = nil;
    self.panel.delegate = nil;
    [self.panel close];
    [shareControllers removeObject:self];
}

- (void)dealloc {
    [_panel release];
    [_link release];
    [_picker release];
    [_service release];
    [super dealloc];
}
@end

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
        if (!shareControllers) shareControllers = [[NSMutableSet alloc] init];
        FinickyShareController *controller = [[FinickyShareController alloc] init];
        controller.link = link;
        [shareControllers addObject:controller];
        [controller present];
        [controller release];
    });
}
