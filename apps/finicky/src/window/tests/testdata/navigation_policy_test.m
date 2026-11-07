#import "../../navigation_policy.h"
#include <stdio.h>

static int failures;

static void Check(BOOL success, NSString *label) {
    if (!success) {
        fprintf(stderr, "FAIL: %s\n", label.UTF8String);
        failures++;
    }
}

int main(void) {
    @autoreleasepool {
        NSURL *document = [NSURL URLWithString:@"finicky-assets://local/index.html"];
        Check(FinickyIsSettingsDocumentURL(document), @"bundled document");
        Check(FinickyIsSettingsDocumentURL([NSURL URLWithString:@"finicky-assets://local/index.html#routing"]), @"document fragment");
        Check(FinickyIsBundledAssetURL([NSURL URLWithString:@"finicky-assets://local/index.js"]), @"bundled script");
        Check(!FinickyIsSettingsDocumentURL([NSURL URLWithString:@"finicky-assets://local/index.js"]), @"script is not a document");
        Check(!FinickyIsSettingsDocumentURL(nil), @"missing document");

        NSArray<NSString *> *untrustedDocuments = @[
            @"about:blank", @"https://local/index.html", @"file:///index.html",
            @"data:text/html,hello", @"finicky-assets://remote/index.html",
            @"finicky-assets://local.evil.example/index.html", @"finicky-assets://local@evil.example/index.html",
            @"finicky-assets://user@local/index.html", @"finicky-assets://local:443/index.html",
            @"finicky-assets://local:0/index.html", @"finicky-assets://local/",
            @"finicky-assets://local/index.html?remote=true", @"finicky-assets://local/other.html",
        ];
        for (NSString *raw in untrustedDocuments) {
            NSURL *url = [NSURL URLWithString:raw];
            Check(!FinickyIsSettingsDocumentURL(url), [@"reject document " stringByAppendingString:raw]);
            Check(FinickySettingsNavigationPolicy(url, YES, NO, YES) == FinickySettingsNavigationCancel,
                [@"reject non-link navigation " stringByAppendingString:raw]);
            Check(!FinickyIsSettingsFrame(YES, url, @"finicky-assets", @"local", 0),
                [@"reject bridge URL " stringByAppendingString:raw]);
        }

        Check(FinickyIsSettingsFrame(YES, document, @"finicky-assets", @"local", 0), @"trusted main frame bridge");
        Check(!FinickyIsSettingsFrame(NO, document, @"finicky-assets", @"local", 0), @"no subframe bridge");
        Check(!FinickyIsSettingsFrame(YES, document, @"https", @"local", 0), @"no different protocol bridge");
        Check(!FinickyIsSettingsFrame(YES, document, @"finicky-assets", @"other", 0), @"no different host bridge");
        Check(!FinickyIsSettingsFrame(YES, document, @"finicky-assets", @"local", 443), @"no different port bridge");
        Check(!FinickyIsSettingsFrame(YES, document, nil, nil, 0), @"no opaque origin bridge");

        Check(FinickySettingsNavigationPolicy(document, YES, NO, NO) == FinickySettingsNavigationAllow, @"native first load without a source frame");
        Check(FinickySettingsNavigationPolicy(document, YES, NO, YES) == FinickySettingsNavigationAllow, @"bundled reload");
        Check(FinickySettingsNavigationPolicy(document, NO, NO, YES) == FinickySettingsNavigationCancel, @"no bundled iframe or popup");
        for (NSString *raw in @[@"https://example.com/path", @"http://example.com/", @"https://example.com:8443/path"]) {
            NSURL *url = [NSURL URLWithString:raw];
            Check(FinickySettingsNavigationPolicy(url, YES, YES, YES) == FinickySettingsNavigationOpenExternal, @"trusted explicit web link");
            Check(FinickySettingsNavigationPolicy(url, NO, YES, YES) == FinickySettingsNavigationOpenExternal, @"target blank explicit web link");
            Check(FinickySettingsNavigationPolicy(url, YES, NO, YES) == FinickySettingsNavigationCancel, @"no script redirect");
            Check(FinickySettingsNavigationPolicy(url, YES, YES, NO) == FinickySettingsNavigationCancel, @"no external link from foreign frame");
        }
        for (NSString *raw in @[@"javascript:alert(1)", @"data:text/html,hello", @"file:///tmp/example.html", @"finicky-plus://set-default?browser=Safari", @"mailto:example@example.com", @"https://user:password@example.com/", @"https:relative"]) {
            Check(FinickySettingsNavigationPolicy([NSURL URLWithString:raw], YES, YES, YES) == FinickySettingsNavigationCancel,
                [@"no privileged or malformed external link " stringByAppendingString:raw]);
        }
    }
    return failures ? 1 : 0;
}
