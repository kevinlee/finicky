#import <Cocoa/Cocoa.h>
#import "picker.h"

enum {
    PickerCancelled = 1 << 0,
    PickerRemember = 1 << 1,
    PickerCopy = 1 << 2,
    PickerShare = 1 << 3,
    PickerPrivate = 1 << 4,
    PickerNewWindow = 1 << 5,
    PickerBackground = 1 << 6,
    PickerBackgroundSpecified = 1 << 7
};

@interface FinickyPickerPanel : NSPanel
@end
@implementation FinickyPickerPanel
- (BOOL)canBecomeKeyWindow { return YES; }
- (BOOL)canBecomeMainWindow { return YES; }
@end

@interface FinickyPickerController : NSObject <NSWindowDelegate, NSTableViewDataSource, NSTableViewDelegate, NSMenuDelegate, NSSharingServicePickerDelegate, NSSharingServiceDelegate>
@property(nonatomic) unsigned long long requestID;
@property(nonatomic, strong) NSDictionary *request;
@property(nonatomic, strong) NSArray<NSDictionary *> *destinations;
@property(nonatomic, strong) FinickyPickerPanel *panel;
@property(nonatomic, strong) NSTableView *table;
@property(nonatomic, strong) NSButton *rememberButton;
@property(nonatomic, strong) NSButton *privateButton;
@property(nonatomic, strong) NSButton *separateWindowButton;
@property(nonatomic, strong) NSButton *backgroundButton;
@property(nonatomic, strong) NSButton *openButton;
@property(nonatomic, strong) id keyboardMonitor;
@property(nonatomic, strong) NSSharingServicePicker *sharingPicker;
@property(nonatomic, strong) NSSharingService *sharingService;
@property(nonatomic) BOOL finished;
@property(nonatomic) BOOL contextMenuOpen;
- (void)present;
- (void)finish:(NSInteger)index flags:(unsigned int)flags;
@end

static NSMutableArray<NSDictionary *> *pendingRequests;
static FinickyPickerController *currentPicker;
static void PresentNextPicker(void);

static NSString *StringValue(id value) {
    return [value isKindOfClass:NSString.class] ? value : @"";
}

static NSButton *ActionButton(NSString *title, id target, SEL action) {
    NSButton *button = [NSButton buttonWithTitle:title target:target action:action];
    button.bezelStyle = NSBezelStyleRounded;
    button.translatesAutoresizingMaskIntoConstraints = NO;
    return button;
}

static NSImage *DestinationIcon(NSDictionary *destination) {
    NSDictionary *browser = destination[@"browser"];
    NSString *name = StringValue(browser[@"name"]);
    NSString *path = nil;
    if ([name hasPrefix:@"/"]) {
        path = name;
    } else {
        NSURL *applicationURL = [NSWorkspace.sharedWorkspace URLForApplicationWithBundleIdentifier:name];
        path = applicationURL.path;
        if (!path) {
            // App names remain accepted by the routing engine, alongside bundle IDs.
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
            path = [NSWorkspace.sharedWorkspace fullPathForApplication:name];
#pragma clang diagnostic pop
        }
    }
    if (path) return [NSWorkspace.sharedWorkspace iconForFile:path];
    return [NSImage imageWithSystemSymbolName:@"globe" accessibilityDescription:@"Browser"];
}

@implementation FinickyPickerController

- (void)present {
    self.destinations = [self.request[@"destinations"] isKindOfClass:NSArray.class] ? self.request[@"destinations"] : @[];
    CGFloat listHeight = MIN(MAX(self.destinations.count, 1) * 62.0, 372.0);
    self.panel = [[FinickyPickerPanel alloc] initWithContentRect:NSMakeRect(0, 0, 560, listHeight + 238)
                                                     styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskClosable
                                                       backing:NSBackingStoreBuffered defer:NO];
    self.panel.title = @"Open Link — Waystile";
    self.panel.delegate = self;
    self.panel.releasedWhenClosed = NO;
    self.panel.hidesOnDeactivate = NO;
    self.panel.level = NSFloatingWindowLevel;
    self.panel.collectionBehavior = NSWindowCollectionBehaviorMoveToActiveSpace | NSWindowCollectionBehaviorFullScreenAuxiliary;

    NSStackView *stack = [[NSStackView alloc] init];
    stack.orientation = NSUserInterfaceLayoutOrientationVertical;
    stack.alignment = NSLayoutAttributeLeading;
    stack.spacing = 10;
    stack.translatesAutoresizingMaskIntoConstraints = NO;
    [self.panel.contentView addSubview:stack];
    [NSLayoutConstraint activateConstraints:@[
        [stack.leadingAnchor constraintEqualToAnchor:self.panel.contentView.leadingAnchor constant:20],
        [stack.trailingAnchor constraintEqualToAnchor:self.panel.contentView.trailingAnchor constant:-20],
        [stack.topAnchor constraintEqualToAnchor:self.panel.contentView.topAnchor constant:16],
        [stack.bottomAnchor constraintLessThanOrEqualToAnchor:self.panel.contentView.bottomAnchor constant:-16]
    ]];

    NSTextField *heading = [NSTextField labelWithString:@"Choose where to open this link"];
    heading.font = [NSFont systemFontOfSize:20 weight:NSFontWeightSemibold];
    [stack addArrangedSubview:heading];

    NSString *URLString = StringValue(self.request[@"url"]);
    NSTextField *URLLabel = [NSTextField wrappingLabelWithString:URLString];
    URLLabel.font = [NSFont systemFontOfSize:12];
    URLLabel.textColor = NSColor.secondaryLabelColor;
    URLLabel.maximumNumberOfLines = 2;
    URLLabel.lineBreakMode = NSLineBreakByTruncatingMiddle;
    URLLabel.selectable = YES;
    URLLabel.toolTip = URLString;
    URLLabel.accessibilityLabel = [@"Link: " stringByAppendingString:URLString];
    [stack addArrangedSubview:URLLabel];
    [URLLabel.widthAnchor constraintEqualToAnchor:stack.widthAnchor].active = YES;
    [URLLabel.heightAnchor constraintEqualToConstant:34].active = YES;

    NSScrollView *scroll = [[NSScrollView alloc] init];
    scroll.translatesAutoresizingMaskIntoConstraints = NO;
    scroll.hasVerticalScroller = YES;
    scroll.autohidesScrollers = YES;
    scroll.borderType = NSBezelBorder;
    scroll.drawsBackground = NO;
    self.table = [[NSTableView alloc] initWithFrame:NSMakeRect(0, 0, 520, listHeight)];
    NSTableColumn *column = [[NSTableColumn alloc] initWithIdentifier:@"destination"];
    column.width = 500;
    [self.table addTableColumn:column];
    self.table.headerView = nil;
    self.table.rowHeight = 62;
    self.table.intercellSpacing = NSMakeSize(0, 0);
    self.table.columnAutoresizingStyle = NSTableViewLastColumnOnlyAutoresizingStyle;
    self.table.allowsMultipleSelection = NO;
    self.table.allowsEmptySelection = NO;
    self.table.dataSource = self;
    self.table.delegate = self;
    self.table.target = self;
    self.table.action = @selector(choose:);
    self.table.doubleAction = @selector(choose:);
    NSMenu *destinationMenu = [[NSMenu alloc] initWithTitle:@"Open Link"];
    destinationMenu.delegate = self;
    destinationMenu.autoenablesItems = NO;
    self.table.menu = destinationMenu;
    self.table.accessibilityLabel = @"Browser and profile destinations";
    self.table.accessibilityHelp = @"Choose a destination. Use arrow keys or Tab to move, and Return to open. Number and letter shortcuts open immediately.";
    scroll.documentView = self.table;
    [stack addArrangedSubview:scroll];
    [scroll.widthAnchor constraintEqualToAnchor:stack.widthAnchor].active = YES;
    [scroll.heightAnchor constraintEqualToConstant:listHeight].active = YES;

    self.privateButton = [NSButton checkboxWithTitle:@"Private window" target:nil action:nil];
    self.privateButton.toolTip = @"Request private browsing in a supported browser.";
    self.separateWindowButton = [NSButton checkboxWithTitle:@"New window" target:nil action:nil];
    self.separateWindowButton.toolTip = @"Request a separate window in a supported browser.";
    self.backgroundButton = [NSButton checkboxWithTitle:@"Background" target:nil action:nil];
    self.backgroundButton.toolTip = @"Ask the browser to open without becoming active. Some browsers may still activate.";
    if ([self.request[@"private"] boolValue]) {
        self.privateButton.state = NSControlStateValueOn;
        self.privateButton.enabled = NO;
        self.privateButton.toolTip = @"This link's rule or request requires private browsing.";
    }
    if ([self.request[@"newWindow"] boolValue]) {
        self.separateWindowButton.state = NSControlStateValueOn;
        self.separateWindowButton.enabled = NO;
        self.separateWindowButton.toolTip = @"This link's rule or request requires a new window.";
    }
    self.backgroundButton.state = [self.request[@"background"] boolValue] ? NSControlStateValueOn : NSControlStateValueOff;
    NSStackView *options = [NSStackView stackViewWithViews:@[self.privateButton, self.separateWindowButton, self.backgroundButton]];
    options.spacing = 18;
    [stack addArrangedSubview:options];

    self.rememberButton = [NSButton checkboxWithTitle:@"Always use this destination for this website" target:nil action:nil];
    self.rememberButton.toolTip = @"Create a rule for this link’s website after choosing a destination.";
    [stack addArrangedSubview:self.rememberButton];

    NSButton *copy = ActionButton(@"Copy Link", self, @selector(copyLink:));
    copy.toolTip = @"Copy this link (Command-C).";
    NSButton *share = ActionButton(@"Share…", self, @selector(shareLink:));
    // AppKit requires showing its sharing picker during mouseDown, while the
    // clicked button remains in a visible window.
    [share sendActionOn:NSEventMaskLeftMouseDown];
    NSButton *cancel = ActionButton(@"Cancel", self, @selector(cancel:));
    self.openButton = ActionButton(@"Open", self, @selector(choose:));
    self.openButton.keyEquivalent = @"\r";
    self.openButton.enabled = self.destinations.count > 0;
    NSView *spacer = [[NSView alloc] init];
    [spacer setContentHuggingPriority:NSLayoutPriorityDefaultLow forOrientation:NSLayoutConstraintOrientationHorizontal];
    NSStackView *footer = [NSStackView stackViewWithViews:@[copy, share, spacer, cancel, self.openButton]];
    footer.spacing = 8;
    footer.distribution = NSStackViewDistributionFill;
    [stack addArrangedSubview:footer];
    [footer.widthAnchor constraintEqualToAnchor:stack.widthAnchor].active = YES;
    self.table.nextKeyView = self.privateButton;
    self.privateButton.nextKeyView = self.separateWindowButton;
    self.separateWindowButton.nextKeyView = self.backgroundButton;
    self.backgroundButton.nextKeyView = self.rememberButton;
    self.rememberButton.nextKeyView = copy;
    copy.nextKeyView = share;
    share.nextKeyView = cancel;
    cancel.nextKeyView = self.openButton;
    self.openButton.nextKeyView = self.table;

    if (self.destinations.count) {
        [self.table selectRowIndexes:[NSIndexSet indexSetWithIndex:0] byExtendingSelection:NO];
    } else {
        self.table.accessibilityHelp = @"No destinations are configured. You can copy or share the link, or cancel.";
    }

    __weak FinickyPickerController *weakSelf = self;
    self.keyboardMonitor = [NSEvent addLocalMonitorForEventsMatchingMask:NSEventMaskKeyDown handler:^NSEvent *(NSEvent *event) {
        FinickyPickerController *controller = weakSelf;
        if (!controller || controller.finished || controller.contextMenuOpen || controller.sharingPicker || controller.sharingService || !controller.panel.isKeyWindow || event.window != controller.panel) return event;
        return [controller handleKey:event] ? nil : event;
    }];
    [self.panel center];
    [NSApp activateIgnoringOtherApps:YES];
    [self.panel makeKeyAndOrderFront:nil];
    [self.panel makeFirstResponder:self.table];
}

- (NSInteger)numberOfRowsInTableView:(NSTableView *)tableView {
    return self.destinations.count;
}

- (NSView *)tableView:(NSTableView *)tableView viewForTableColumn:(NSTableColumn *)tableColumn row:(NSInteger)row {
    NSDictionary *destination = self.destinations[row];
    CGFloat width = tableColumn.width;
    NSTableCellView *cell = [[NSTableCellView alloc] initWithFrame:NSMakeRect(0, 0, width, 62)];
    cell.identifier = @"destination";
    NSImageView *icon = [[NSImageView alloc] initWithFrame:NSMakeRect(12, 13, 36, 36)];
    icon.image = DestinationIcon(destination);
    icon.imageScaling = NSImageScaleProportionallyUpOrDown;
    icon.accessibilityElement = NO;
    [cell addSubview:icon];
    cell.imageView = icon;

    NSString *name = StringValue(destination[@"name"]);
    NSString *subtitle = StringValue(destination[@"subtitle"]);
    NSTextField *nameLabel = [NSTextField labelWithString:name];
    nameLabel.font = [NSFont systemFontOfSize:14 weight:NSFontWeightSemibold];
    nameLabel.frame = NSMakeRect(60, subtitle.length ? 31 : 21, MAX(width - 122, 1), 20);
    nameLabel.autoresizingMask = NSViewWidthSizable;
    nameLabel.lineBreakMode = NSLineBreakByTruncatingTail;
    [cell addSubview:nameLabel];
    cell.textField = nameLabel;
    if (subtitle.length) {
        NSTextField *detail = [NSTextField labelWithString:subtitle];
        detail.font = [NSFont systemFontOfSize:11];
        detail.textColor = NSColor.secondaryLabelColor;
        detail.frame = NSMakeRect(60, 13, MAX(width - 122, 1), 16);
        detail.autoresizingMask = NSViewWidthSizable;
        detail.lineBreakMode = NSLineBreakByTruncatingTail;
        [cell addSubview:detail];
    }
    NSString *shortcut = StringValue(destination[@"shortcut"]);
    if (!shortcut.length && row < 9) shortcut = [NSString stringWithFormat:@"%ld", (long)row + 1];
    NSTextField *key = [NSTextField labelWithString:shortcut.uppercaseString];
    key.font = [NSFont monospacedSystemFontOfSize:12 weight:NSFontWeightMedium];
    key.textColor = NSColor.secondaryLabelColor;
    key.alignment = NSTextAlignmentCenter;
    key.frame = NSMakeRect(width - 44, 21, 30, 20);
    key.autoresizingMask = NSViewMinXMargin;
    [cell addSubview:key];
    cell.accessibilityLabel = subtitle.length ? [NSString stringWithFormat:@"%@, %@", name, subtitle] : name;
    if (shortcut.length) cell.accessibilityHelp = [NSString stringWithFormat:@"Press %@ to open.", shortcut.uppercaseString];
    cell.toolTip = cell.accessibilityLabel;
    return cell;
}

- (BOOL)handleKey:(NSEvent *)event {
    NSEventModifierFlags modifiers = event.modifierFlags & NSEventModifierFlagDeviceIndependentFlagsMask;
    NSString *key = event.charactersIgnoringModifiers.lowercaseString;
    if ((modifiers & NSEventModifierFlagCommand) && !(modifiers & (NSEventModifierFlagControl | NSEventModifierFlagOption)) && [key isEqualToString:@"c"]) {
        [self copyLink:nil]; return YES;
    }
    // Leave VoiceOver and other system shortcuts intact.
    if (modifiers & (NSEventModifierFlagCommand | NSEventModifierFlagControl | NSEventModifierFlagOption)) return NO;
    if (event.keyCode == 53) { [self cancel:nil]; return YES; }
    if (event.keyCode == 36 || event.keyCode == 76) { [self choose:nil]; return YES; }
    BOOL tableHasFocus = self.panel.firstResponder == self.table;
    if (tableHasFocus && (event.keyCode == 125 || event.keyCode == 126 || event.keyCode == 48)) {
        NSInteger count = self.destinations.count;
        if (count) {
            NSInteger delta = (event.keyCode == 126 || (event.keyCode == 48 && (modifiers & NSEventModifierFlagShift))) ? -1 : 1;
            if (event.keyCode == 48 && ((delta > 0 && self.table.selectedRow == count - 1) || (delta < 0 && self.table.selectedRow == 0))) {
                // Tab visits rows, then leaves the list so every option remains
                // reachable with the keyboard. Arrow navigation wraps instead.
                return NO;
            }
            NSInteger selected = (self.table.selectedRow + delta + count) % count;
            [self.table selectRowIndexes:[NSIndexSet indexSetWithIndex:selected] byExtendingSelection:NO];
            [self.table scrollRowToVisible:selected];
        }
        return YES;
    }
    // Space remains a standard accessibility/control activation key.
    if (tableHasFocus && [key isEqualToString:@" "]) { [self choose:nil]; return YES; }
    if (key.length == 1) {
        unichar character = [key characterAtIndex:0];
        if (character >= '1' && character <= '9') {
            NSInteger index = character - '1';
            if (index < (NSInteger)self.destinations.count) { [self finish:index flags:0]; return YES; }
        }
        for (NSInteger i = 0; i < (NSInteger)self.destinations.count; i++) {
            if ([StringValue(self.destinations[i][@"shortcut"]) isEqualToString:key]) {
                [self finish:i flags:0]; return YES;
            }
        }
    }
    return NO;
}

- (void)choose:(id)sender {
    NSInteger index = self.table.selectedRow;
    if (index >= 0 && index < (NSInteger)self.destinations.count) [self finish:index flags:0];
}

- (void)menuNeedsUpdate:(NSMenu *)menu {
    [menu removeAllItems];
    NSInteger index = self.table.clickedRow;
    if (index < 0) index = self.table.selectedRow;
    if (index < 0 || index >= (NSInteger)self.destinations.count) return;
    [self.table selectRowIndexes:[NSIndexSet indexSetWithIndex:index] byExtendingSelection:NO];
    NSArray<NSString *> *titles = @[@"Open", @"Open in Background", @"Open in New Window", @"Open Private", @"Always Use for This Website"];
    const unsigned int flags[] = {0, PickerBackground, PickerNewWindow, PickerPrivate, PickerRemember};
    for (NSUInteger i = 0; i < titles.count; i++) {
        if (i == titles.count - 1) [menu addItem:NSMenuItem.separatorItem];
        NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:titles[i] action:@selector(contextAction:) keyEquivalent:@""];
        item.target = self;
        item.tag = flags[i];
        item.representedObject = @(index);
        [menu addItem:item];
    }
}

- (void)menuWillOpen:(NSMenu *)menu { self.contextMenuOpen = YES; }
- (void)menuDidClose:(NSMenu *)menu { self.contextMenuOpen = NO; }

- (void)contextAction:(NSMenuItem *)item {
    NSInteger index = [item.representedObject integerValue];
    if (index >= 0 && index < (NSInteger)self.destinations.count) {
        [self finish:index flags:(unsigned int)item.tag];
    }
}

- (void)cancel:(id)sender { [self finish:-1 flags:PickerCancelled]; }
- (void)copyLink:(id)sender { [self finish:-1 flags:PickerCopy]; }
- (void)shareLink:(NSButton *)sender {
    if (self.finished || self.sharingPicker || self.sharingService) return;
    NSURL *link = [NSURL URLWithString:StringValue(self.request[@"url"])];
    if (!link || !sender.window.isVisible) return;
    self.sharingPicker = [[NSSharingServicePicker alloc] initWithItems:@[link]];
    self.sharingPicker.delegate = self;
    [self.sharingPicker showRelativeToRect:sender.bounds ofView:sender preferredEdge:NSRectEdgeMaxY];
}

- (id<NSSharingServiceDelegate>)sharingServicePicker:(NSSharingServicePicker *)picker delegateForSharingService:(NSSharingService *)service {
    self.sharingService = service;
    return self;
}

- (void)sharingServicePicker:(NSSharingServicePicker *)picker didChooseSharingService:(NSSharingService *)service {
    if (!service) {
        self.sharingPicker.delegate = nil;
        self.sharingPicker = nil;
        self.sharingService = nil;
    }
    // A chosen service receives its items after this callback returns. Keep
    // the anchor alive until that service finishes; cancelling the popover
    // instead returns to the same destination picker.
}

- (void)sharingService:(NSSharingService *)service didShareItems:(NSArray *)items {
    // AppKit has handled the action. Complete as cancelled so Go does not open
    // a browser or present a second sharing picker.
    [self finish:-1 flags:PickerCancelled];
}

- (void)sharingService:(NSSharingService *)service didFailToShareItems:(NSArray *)items error:(NSError *)error {
    self.sharingPicker.delegate = nil;
    self.sharingPicker = nil;
    self.sharingService.delegate = nil;
    self.sharingService = nil;
    if ([error.domain isEqualToString:NSCocoaErrorDomain] && error.code == NSUserCancelledError) return;
    NSAlert *alert = [NSAlert alertWithError:error];
    [alert beginSheetModalForWindow:self.panel completionHandler:nil];
}
- (void)windowWillClose:(NSNotification *)notification { [self finish:-1 flags:PickerCancelled]; }

- (void)finish:(NSInteger)index flags:(unsigned int)flags {
    if (self.finished) return;
    self.finished = YES;
    if (index >= 0) {
        flags |= PickerBackgroundSpecified;
        NSEventModifierFlags modifiers = NSEvent.modifierFlags;
        if (self.rememberButton.state == NSControlStateValueOn || (modifiers & NSEventModifierFlagCommand)) flags |= PickerRemember;
        if (self.privateButton.state == NSControlStateValueOn) flags |= PickerPrivate;
        if (self.separateWindowButton.state == NSControlStateValueOn) flags |= PickerNewWindow;
        if (self.backgroundButton.state == NSControlStateValueOn || (modifiers & NSEventModifierFlagControl)) flags |= PickerBackground;
    }
    if (self.keyboardMonitor) {
        [NSEvent removeMonitor:self.keyboardMonitor];
        self.keyboardMonitor = nil;
    }
    self.sharingPicker.delegate = nil;
    [self.sharingPicker close];
    self.sharingPicker = nil;
    self.sharingService.delegate = nil;
    self.sharingService = nil;
    self.panel.delegate = nil;
    [self.panel orderOut:nil];
    [self.panel close];
    FinickyPickerDidSelect(self.requestID, (int)index, flags);
    dispatch_async(dispatch_get_main_queue(), ^{
        currentPicker = nil;
        PresentNextPicker();
    });
}
@end

static void PresentNextPicker(void) {
    if (currentPicker || pendingRequests.count == 0) return;
    NSDictionary *next = pendingRequests.firstObject;
    [pendingRequests removeObjectAtIndex:0];
    currentPicker = [[FinickyPickerController alloc] init];
    currentPicker.requestID = [next[@"id"] unsignedLongLongValue];
    currentPicker.request = next[@"request"];
    [currentPicker present];
}

void FinickyShowPicker(unsigned long long requestID, const char *requestJSON) {
    // Copy before returning: Go releases the input buffer immediately after this call.
    NSString *json = requestJSON ? [NSString stringWithUTF8String:requestJSON] : @"";
    dispatch_async(dispatch_get_main_queue(), ^{
        NSData *data = [json dataUsingEncoding:NSUTF8StringEncoding];
        id request = data ? [NSJSONSerialization JSONObjectWithData:data options:0 error:nil] : nil;
        if (![request isKindOfClass:NSDictionary.class]) {
            FinickyPickerDidSelect(requestID, -1, PickerCancelled);
            return;
        }
        if (!pendingRequests) pendingRequests = [NSMutableArray array];
        [pendingRequests addObject:@{@"id": @(requestID), @"request": request}];
        PresentNextPicker();
    });
}
