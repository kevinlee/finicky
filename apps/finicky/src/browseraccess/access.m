#import <Cocoa/Cocoa.h>
#include <string.h>
#import "access.h"

static NSString *const BookmarkDefaultsKey = @"FinickyPlusBrowserProfileBookmarks";
static NSObject *accessLock;
static NSMutableDictionary<NSString *, NSURL *> *activeFolders;
static NSOpenPanel *accessPanel;
static id terminationObserver;

static void InitializeAccess(void) {
    static dispatch_once_t once;
    dispatch_once(&once, ^{
        accessLock = [[NSObject alloc] init];
        activeFolders = [NSMutableDictionary dictionary];
        terminationObserver = [NSNotificationCenter.defaultCenter addObserverForName:NSApplicationWillTerminateNotification
                                                                              object:nil queue:nil usingBlock:^(NSNotification *note) {
            (void)note;
            @synchronized (accessLock) {
                for (NSURL *url in activeFolders.allValues) [url stopAccessingSecurityScopedResource];
                [activeFolders removeAllObjects];
            }
        }];
    });
}

static NSError *AccessError(NSString *message) {
    return [NSError errorWithDomain:@"FinickyPlusBrowserAccess" code:1
                          userInfo:@{NSLocalizedDescriptionKey: message}];
}

static NSString *FolderKey(NSURL *url) {
    return url.URLByStandardizingPath.path;
}

// Must be called with accessLock held. Successful scopes are retained once per
// folder and balanced on normal app termination. Discovery needs them all day.
static BOOL StartFolder(NSURL *url, NSError **error) {
    NSString *key = FolderKey(url);
    if (!key || !url.isFileURL) {
        if (error) *error = AccessError(@"The selected browser folder is not a local folder.");
        return NO;
    }
    if (activeFolders[key]) return YES;
    BOOL scoped = [url startAccessingSecurityScopedResource];
    NSNumber *isDirectory = nil;
    NSError *resourceError = nil;
    BOOL exists = [url getResourceValue:&isDirectory forKey:NSURLIsDirectoryKey error:&resourceError];
    BOOL readable = [NSFileManager.defaultManager isReadableFileAtPath:url.path];
    if (!exists || !isDirectory.boolValue || !readable) {
        if (scoped) [url stopAccessingSecurityScopedResource];
        if (error) *error = resourceError ?: AccessError(@"This browser folder is not readable. Choose the folder again to grant access.");
        return NO;
    }
    // Unsandboxed builds may already have direct access. A false scope result
    // alone is not a failure when the chosen directory is verifiably readable.
    if (scoped) activeFolders[key] = url;
    return YES;
}

static NSData *ReadOnlyBookmark(NSURL *url, NSError **error) {
    return [url bookmarkDataWithOptions:NSURLBookmarkCreationWithSecurityScope | NSURLBookmarkCreationSecurityScopeAllowOnlyReadAccess
        includingResourceValuesForKeys:@[NSURLIsDirectoryKey] relativeToURL:nil error:error];
}

char *FinickyRestoreBrowserAccess(void) {
    @autoreleasepool {
        InitializeAccess();
        @synchronized (accessLock) {
            id saved = [NSUserDefaults.standardUserDefaults objectForKey:BookmarkDefaultsKey];
            if (!saved) return NULL;
            if (![saved isKindOfClass:NSDictionary.class]) {
                return strdup("Saved browser-folder access is invalid. Choose the browser folder again in Settings.");
            }
            NSMutableDictionary *updated = [saved mutableCopy];
            NSMutableArray<NSString *> *failures = [NSMutableArray array];
            for (NSString *key in [saved allKeys]) {
                id data = saved[key];
                if (![data isKindOfClass:NSData.class]) {
                    [failures addObject:@"A saved folder bookmark is invalid."];
                    continue;
                }
                BOOL stale = NO;
                NSError *error = nil;
                NSURL *url = [NSURL URLByResolvingBookmarkData:data
                    options:NSURLBookmarkResolutionWithSecurityScope | NSURLBookmarkResolutionWithoutUI
                    relativeToURL:nil bookmarkDataIsStale:&stale error:&error];
                if (!url || !StartFolder(url, &error)) {
                    [failures addObject:error.localizedDescription ?: @"A saved browser folder could not be reopened."];
                    continue;
                }
                if (stale || ![key isEqualToString:FolderKey(url)]) {
                    NSData *fresh = ReadOnlyBookmark(url, &error);
                    if (fresh) {
                        [updated removeObjectForKey:key];
                        updated[FolderKey(url)] = fresh;
                    } else {
                        [failures addObject:error.localizedDescription ?: @"A browser folder bookmark could not be refreshed."];
                    }
                }
            }
            if (![updated isEqual:saved]) [NSUserDefaults.standardUserDefaults setObject:updated forKey:BookmarkDefaultsKey];
            if (failures.count) {
                NSString *message = [NSString stringWithFormat:@"Some saved browser folders need access again. Choose them in Settings. %@", [failures componentsJoinedByString:@" "]];
                return strdup(message.UTF8String);
            }
            return NULL;
        }
    }
}

void FinickyRequestBrowserAccess(unsigned long long requestID) {
    InitializeAccess();
    dispatch_async(dispatch_get_main_queue(), ^{
        if (accessPanel) {
            FinickyBrowserAccessFinished(requestID, "The browser folder chooser is already open.");
            [accessPanel makeKeyAndOrderFront:nil];
            return;
        }
        accessPanel = [NSOpenPanel openPanel];
        accessPanel.title = @"Allow Browser Profile Access";
        accessPanel.message = @"Choose your browser’s data folder, such as Google/Chrome or Firefox. Waystile will read profile names from this folder.";
        accessPanel.prompt = @"Allow Access";
        accessPanel.canChooseDirectories = YES;
        accessPanel.canChooseFiles = NO;
        accessPanel.canCreateDirectories = NO;
        accessPanel.allowsMultipleSelection = NO;
        accessPanel.resolvesAliases = YES;
        accessPanel.directoryURL = [NSFileManager.defaultManager URLsForDirectory:NSApplicationSupportDirectory inDomains:NSUserDomainMask].firstObject;
        [NSApp activateIgnoringOtherApps:YES];
        [accessPanel beginWithCompletionHandler:^(NSModalResponse response) {
            NSURL *chosen = accessPanel.URL;
            accessPanel = nil;
            if (response != NSModalResponseOK || !chosen) {
                FinickyBrowserAccessFinished(requestID, "Browser profile folder selection was cancelled.");
                return;
            }
            NSError *error = nil;
            @synchronized (accessLock) {
                if (!StartFolder(chosen, &error)) {
                    NSString *message = error.localizedDescription ?: @"Could not access the selected browser folder.";
                    FinickyBrowserAccessFinished(requestID, (char *)message.UTF8String);
                    return;
                }
                NSData *bookmark = ReadOnlyBookmark(chosen, &error);
                if (!bookmark) {
                    NSString *message = [NSString stringWithFormat:@"The folder is readable now, but access could not be saved for next time. %@", error.localizedDescription ?: @"Please try again."];
                    FinickyBrowserAccessFinished(requestID, (char *)message.UTF8String);
                    return;
                }
                id saved = [NSUserDefaults.standardUserDefaults objectForKey:BookmarkDefaultsKey];
                NSMutableDictionary *bookmarks = [saved isKindOfClass:NSDictionary.class] ? [saved mutableCopy] : [NSMutableDictionary dictionary];
                bookmarks[FolderKey(chosen)] = bookmark;
                [NSUserDefaults.standardUserDefaults setObject:bookmarks forKey:BookmarkDefaultsKey];
            }
            FinickyBrowserAccessFinished(requestID, NULL);
        }];
    });
}
