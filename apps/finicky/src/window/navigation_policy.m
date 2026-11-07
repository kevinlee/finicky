#import "navigation_policy.h"

BOOL FinickyIsBundledAssetURL(NSURL *url) {
    return url && [url.scheme.lowercaseString isEqualToString:@"finicky-assets"] &&
        [url.host.lowercaseString isEqualToString:@"local"] &&
        !url.user && !url.password && !url.port;
}

BOOL FinickyIsSettingsDocumentURL(NSURL *url) {
    return FinickyIsBundledAssetURL(url) && [url.path isEqualToString:@"/index.html"] && !url.query;
}

BOOL FinickyIsSettingsFrame(BOOL isMainFrame, NSURL *url, NSString *originProtocol, NSString *originHost, NSInteger originPort) {
    return isMainFrame && FinickyIsSettingsDocumentURL(url) &&
        [originProtocol.lowercaseString isEqualToString:@"finicky-assets"] &&
        [originHost.lowercaseString isEqualToString:@"local"] && originPort == 0;
}

FinickySettingsNavigation FinickySettingsNavigationPolicy(NSURL *url, BOOL targetsMainFrame, BOOL isLinkClick, BOOL fromSettingsFrame) {
    // The initial native load has no source document yet. Only this exact
    // bundled document may load or reload inside the settings web view.
    if (targetsMainFrame && FinickyIsSettingsDocumentURL(url)) {
        return FinickySettingsNavigationAllow;
    }
    NSString *scheme = url.scheme.lowercaseString;
    BOOL isWebLink = [scheme isEqualToString:@"https"] || [scheme isEqualToString:@"http"];
    if (isLinkClick && fromSettingsFrame && isWebLink && url.host.length && !url.user && !url.password) {
        return FinickySettingsNavigationOpenExternal;
    }
    return FinickySettingsNavigationCancel;
}
