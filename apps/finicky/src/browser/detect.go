package browser

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>
#include <stdlib.h>
#include <string.h>

// isLikelyBrowser returns YES if the app at appURL registers for http or https
// with LSHandlerRank "Default" or "Alternate". Apps that set LSHandlerRank "None"
// are using deep-link / download-interception tricks, not acting as browsers.
// Absent LSHandlerRank defaults to "Default" per Apple docs.
static BOOL isLikelyBrowser(NSURL *appURL) {
	NSBundle *bundle = [NSBundle bundleWithURL:appURL];
	NSDictionary *info = bundle.infoDictionary;
	if (!info) return NO;

	NSArray *urlTypes = info[@"CFBundleURLTypes"];
	if (!urlTypes) return NO;

	for (NSDictionary *urlType in urlTypes) {
		NSArray *schemes = urlType[@"CFBundleURLSchemes"] ?: @[];
		if (![schemes containsObject:@"http"] && ![schemes containsObject:@"https"]) continue;

		NSString *rank = urlType[@"LSHandlerRank"] ?: @"Default";
		if ([rank isEqualToString:@"Default"] || [rank isEqualToString:@"Alternate"]) {
			return YES;
		}
	}
	return NO;
}

static char **getAllHttpsHandlerNames(int *count) {
	@autoreleasepool {
		NSURL *url = [NSURL URLWithString:@"https://example.com"];
		NSArray<NSURL *> *appURLs = [[NSWorkspace sharedWorkspace] URLsForApplicationsToOpenURL:url];
		if (!appURLs || appURLs.count == 0) {
			*count = 0;
			return NULL;
		}

		NSMutableSet<NSString *> *seen = [NSMutableSet set];
		NSMutableArray<NSString *> *names = [NSMutableArray array];
		NSSet *excludedBundleIDs = [NSSet setWithObjects:
			@"se.johnste.finicky",
			@"net.kassett.finicky",
			@"com.kevinlee.finicky",
			@"com.kevinlee.finicky.preview",
			nil];

		for (NSURL *appURL in appURLs) {
			NSBundle *bundle = [NSBundle bundleWithURL:appURL];
			if ([excludedBundleIDs containsObject:bundle.bundleIdentifier]) continue;
			if (!isLikelyBrowser(appURL)) continue;

			NSString *name = [[NSFileManager defaultManager] displayNameAtPath:[appURL path]];
			if ([name hasSuffix:@".app"]) {
				name = [name substringToIndex:[name length] - 4];
			}
			if (![seen containsObject:name]) {
				[seen addObject:name];
				[names addObject:name];
			}
		}

		*count = (int)names.count;
		char **result = (char **)malloc(names.count * sizeof(char *));
		for (NSInteger i = 0; i < (NSInteger)names.count; i++) {
			result[i] = strdup([names[i] UTF8String]);
		}
		return result;
	}
}

static void freeNames(char **names, int count) {
	for (int i = 0; i < count; i++) {
		free(names[i]);
	}
	free(names);
}

// Use int at the cgo boundary: Objective-C BOOL differs on Intel and arm64.
static int appExists(const char *identifier) {
	@autoreleasepool {
		NSString *value = [NSString stringWithUTF8String:identifier];
		if ([value hasPrefix:@"/"]) {
			return [[NSFileManager defaultManager] fileExistsAtPath:value];
		}
		NSURL *url = [[NSWorkspace sharedWorkspace] URLForApplicationWithBundleIdentifier:value];
		if (url) return YES;
		return [[NSWorkspace sharedWorkspace] fullPathForApplication:value] != nil;
	}
}
*/
import "C"
import (
	"sort"
	"unsafe"
)

// AppExists checks Launch Services without opening or activating an app.
func AppExists(identifier string) bool {
	value := C.CString(identifier)
	defer C.free(unsafe.Pointer(value))
	return C.appExists(value) != 0
}

// GetInstalledBrowsers returns the display names of all apps registered to
// handle https:// URLs, as reported by the macOS Launch Services framework.
func GetInstalledBrowsers() []string {
	var count C.int
	names := C.getAllHttpsHandlerNames(&count)
	if names == nil {
		return []string{}
	}
	defer C.freeNames(names, count)

	n := int(count)
	nameSlice := unsafe.Slice(names, n)
	result := make([]string, n)
	for i, s := range nameSlice {
		result[i] = C.GoString(s)
	}
	sort.Strings(result)
	return result
}
