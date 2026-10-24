// browse.h
#ifndef MAIN_H
#define MAIN_H

#ifdef __OBJC__
#import <Cocoa/Cocoa.h>
#endif

#include <syslog.h>
#include <stdbool.h>

extern void HandleURL(char *url, char *name, char *bundleId, char *path, char *windowTitle, bool openInBackground);
extern void QueueWindowDisplay(int launchedByUser);
extern void ShowConfigWindow();
extern char* GetCurrentConfigPath();

#ifdef __OBJC__
@interface BrowseAppDelegate: NSObject<NSApplicationDelegate>
    @property (nonatomic) bool forceOpenWindow;
    @property (nonatomic) bool receivedURL;
    @property (nonatomic) bool keepRunning;
    @property (nonatomic) bool showMenuItem;
    - (instancetype)initWithForceOpenWindow:(bool)forceOpenWindow initShow:(bool)showMenuItem keepRunning:(bool)keepRunning;
    - (void)handleGetURLEvent:(NSAppleEventDescriptor *) event withReplyEvent:(NSAppleEventDescriptor *)replyEvent;
    - (bool)application:(NSApplication *)sender openFile:(NSString *)filename;
@end
#endif

void RunApp(bool forceOpenWindow, bool showStatusItem, bool keepRunning);
void SetPreviewMode(bool enabled);
void SetStatusItemError(bool hasError);
void SetMenuIcon(char *style, char *browser);
char *ReadClipboard(void);
void CopyText(char *text);
void ShareURL(char *url);
extern void HandleSelectedText(char *text, bool prompt);
extern char *GetMenuDestinations(void);
extern void ChooseMenuDestination(char *identifier, bool launch);

#endif /* MAIN_H */
