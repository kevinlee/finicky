#pragma once
#import <Foundation/Foundation.h>

typedef NS_ENUM(NSInteger, FinickySettingsNavigation) {
    FinickySettingsNavigationCancel,
    FinickySettingsNavigationAllow,
    FinickySettingsNavigationOpenExternal,
};

BOOL FinickyIsBundledAssetURL(NSURL *url);
BOOL FinickyIsSettingsDocumentURL(NSURL *url);
BOOL FinickyIsSettingsFrame(BOOL isMainFrame, NSURL *url, NSString *originProtocol, NSString *originHost, NSInteger originPort);
FinickySettingsNavigation FinickySettingsNavigationPolicy(NSURL *url, BOOL targetsMainFrame, BOOL isLinkClick, BOOL fromSettingsFrame);
