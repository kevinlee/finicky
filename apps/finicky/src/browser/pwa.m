#import <Foundation/Foundation.h>
#include <stdlib.h>
#include <string.h>

// Parse XML and binary plists through Foundation without loading the bundle's
// executable or trusting arbitrary object types in its metadata.
char *finickyPWAInfoJSON(const void *bytes, size_t length) {
  @autoreleasepool {
    NSData *data = [NSData dataWithBytes:bytes length:length];
    id plist = [NSPropertyListSerialization propertyListWithData:data
        options:NSPropertyListImmutable format:NULL error:NULL];
    if (![plist isKindOfClass:[NSDictionary class]]) return NULL;
    NSArray *keys = @[@"CrBundleIdentifier", @"CrAppModeShortcutID",
                      @"CrAppModeProfileDir", @"CrAppModeUserDataDir"];
    if (!plist[@"CrAppModeShortcutID"] && !plist[@"CrAppModeUserDataDir"])
      return NULL;
    NSMutableDictionary *values = [NSMutableDictionary dictionary];
    for (NSString *key in keys) {
      id value = plist[key];
      if (value && ![value isKindOfClass:[NSString class]]) {
        values[@"error"] = @"Web app metadata contains a non-text field";
        break;
      }
      if (value) values[key] = value;
    }
    NSData *json = [NSJSONSerialization dataWithJSONObject:values options:0 error:NULL];
    if (!json) return NULL;
    char *result = malloc(json.length + 1);
    if (!result) return NULL;
    memcpy(result, json.bytes, json.length);
    result[json.length] = 0;
    return result;
  }
}
