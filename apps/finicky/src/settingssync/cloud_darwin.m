#import <Foundation/Foundation.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <unistd.h>
#include <errno.h>

static char *FSSString(NSString *value) { return strdup([(value ?: @"Cloud storage is unavailable") UTF8String]); }

// Download requests are limited to the exact settings paths passed to this
// bridge. Repeated polls do not continuously resubmit the same request.
static BOOL FSSRequestDownload(NSURL *url) {
    static NSMutableDictionary<NSString *, NSDate *> *requested;
    @synchronized ([NSFileCoordinator class]) {
        if (!requested) requested=[NSMutableDictionary dictionary];
        NSDate *last=requested[url.path];
        if (last && -last.timeIntervalSinceNow < 30) return NO;
        if (requested.count>=1024) [requested removeAllObjects];
        requested[url.path]=NSDate.date;
    }
    [[NSFileManager defaultManager] startDownloadingUbiquitousItemAtURL:url error:nil];
    return YES;
}

static BOOL FSSPlaceholder(NSURL *url) {
    NSString *name=[[@"." stringByAppendingString:url.lastPathComponent] stringByAppendingString:@".icloud"];
    NSURL *placeholder=[[url URLByDeletingLastPathComponent] URLByAppendingPathComponent:name];
    if ([[NSFileManager defaultManager] fileExistsAtPath:placeholder.path]) { FSSRequestDownload(url);return YES; }
    return NO;
}

static BOOL FSSReady(NSURL *url, int *code, NSString **message) {
    struct stat info;
    if (lstat(url.path.fileSystemRepresentation, &info) != 0) {
        *code = errno == ENOENT ? 3 : 2; *message = @"Cloud item is not available locally"; return NO;
    }
    if (S_ISLNK(info.st_mode)) { *code=2; *message=@"Cloud symlinks are not supported"; return NO; }
    NSNumber *conflicts=nil;
    [url getResourceValue:&conflicts forKey:NSURLUbiquitousItemHasUnresolvedConflictsKey error:nil];
    if (conflicts.boolValue) { *code=4;*message=@"iCloud reports conflicting versions of a settings item; resolve the file conflict before syncing";return NO; }
    NSString *status = nil;
    [url getResourceValue:&status forKey:NSURLUbiquitousItemDownloadingStatusKey error:nil];
    if ([status isEqualToString:NSURLUbiquitousItemDownloadingStatusNotDownloaded]) {
        FSSRequestDownload(url);
        *code=1; *message=@"Waiting for iCloud to download settings"; return NO;
    }
    return YES;
}

char *FSSList(const char *path, int *code) {
    @autoreleasepool {
        NSURL *url=[NSURL fileURLWithPath:[NSString stringWithUTF8String:path] isDirectory:YES];
        if (FSSPlaceholder(url)||FSSPlaceholder([url URLByDeletingLastPathComponent])) { *code=1;return FSSString(@"Waiting for iCloud to download the settings directory"); }
        __block NSString *message=nil; __block NSData *result=nil; __block int resultCode=0;
        NSError *error=nil;
        [[[NSFileCoordinator alloc] initWithFilePresenter:nil] coordinateReadingItemAtURL:url options:0 error:&error byAccessor:^(NSURL *coordinated) {
            if (!FSSReady(coordinated,&resultCode,&message)) return;
            NSError *readError=nil;
            NSArray *names=[[NSFileManager defaultManager] contentsOfDirectoryAtPath:coordinated.path error:&readError];
            if (!names) { resultCode=2;message=readError.localizedDescription;return; }
            NSUInteger downloads=0; BOOL placeholders=NO;
            NSRegularExpression *revisionPlaceholder=[NSRegularExpression regularExpressionWithPattern:@"^\\.[a-f0-9]{32}\\.json\\.icloud$" options:0 error:nil];
            for (NSString *name in names) {
                if ([name hasSuffix:@".icloud"]) {
                    if ([revisionPlaceholder numberOfMatchesInString:name options:0 range:NSMakeRange(0,name.length)]!=1) {resultCode=4;message=@"Unexpected placeholder or duplicate item in settings history; no content was chosen automatically";return;}
                    placeholders=YES;
                    if (downloads<8) {
                        NSString *target=[name substringWithRange:NSMakeRange(1,name.length-1-7)];
                        if (FSSRequestDownload([coordinated URLByAppendingPathComponent:target])) downloads++;
                    }
                }
            }
            if (placeholders) {resultCode=1;message=@"Waiting for iCloud to download settings revisions";return;}
            result=[NSJSONSerialization dataWithJSONObject:names options:0 error:&readError];
            if (!result) {resultCode=2;message=readError.localizedDescription;}
        }];
        if (error) {resultCode=2;message=error.localizedDescription;}
        *code=resultCode;
        return resultCode==0 ? FSSString([[NSString alloc] initWithData:result encoding:NSUTF8StringEncoding]) : FSSString(message);
    }
}

char *FSSRead(const char *path,int *code) {
    @autoreleasepool {
        NSURL *url=[NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
        if (FSSPlaceholder(url)) { *code=1;return FSSString(@"Waiting for iCloud to download a settings revision"); }
        __block NSString *message=nil; __block NSData *result=nil; __block int resultCode=0;
        NSError *error=nil;
        [[[NSFileCoordinator alloc] initWithFilePresenter:nil] coordinateReadingItemAtURL:url options:0 error:&error byAccessor:^(NSURL *coordinated) {
            if (!FSSReady(coordinated,&resultCode,&message)) return;
            struct stat info;
            if (lstat(coordinated.path.fileSystemRepresentation,&info)!=0 || !S_ISREG(info.st_mode) || info.st_size>1048576) {resultCode=2;message=@"Invalid or oversized settings revision";return;}
            NSError *readError=nil;
            result=[NSData dataWithContentsOfURL:coordinated options:NSDataReadingUncached error:&readError];
            if (!result) {resultCode=2;message=readError.localizedDescription;return;}
            if (result.length>1048576 || memchr(result.bytes,0,result.length)) {resultCode=2;message=@"Invalid settings revision data";return;}
        }];
        if (error) {resultCode=2;message=error.localizedDescription;}
        *code=resultCode;
        return resultCode==0 ? FSSString([[NSString alloc] initWithData:result encoding:NSUTF8StringEncoding]) : FSSString(message);
    }
}

char *FSSMkdir(const char *path,int *code) {
    @autoreleasepool {
        NSURL *url=[NSURL fileURLWithPath:[NSString stringWithUTF8String:path] isDirectory:YES];
        __block NSString *message=@""; __block int resultCode=0;
        NSError *error=nil;
        [[[NSFileCoordinator alloc] initWithFilePresenter:nil] coordinateWritingItemAtURL:url options:0 error:&error byAccessor:^(NSURL *coordinated) {
            struct stat info;
            if (lstat(coordinated.path.fileSystemRepresentation,&info)==0) {
                if (!S_ISDIR(info.st_mode)||S_ISLNK(info.st_mode)){resultCode=2;message=@"Cloud directory is not a regular directory";}
                else FSSReady(coordinated,&resultCode,&message);
                return;
            }
            if (FSSPlaceholder(coordinated)) {resultCode=1;message=@"Waiting for iCloud to download the settings directory";return;}
            NSError *writeError=nil;
            if (![[NSFileManager defaultManager] createDirectoryAtURL:coordinated withIntermediateDirectories:NO attributes:@{NSFilePosixPermissions:@0700} error:&writeError]) {resultCode=2;message=writeError.localizedDescription;}
        }];
        if (error) {resultCode=2;message=error.localizedDescription;}
        *code=resultCode;return FSSString(message);
    }
}

char *FSSWrite(const char *path,const void *bytes,int length,int *code) {
    @autoreleasepool {
        NSURL *url=[NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
        NSData *data=[NSData dataWithBytes:bytes length:length];
        __block NSString *message=@""; __block int resultCode=0;
        NSError *error=nil;
        [[[NSFileCoordinator alloc] initWithFilePresenter:nil] coordinateWritingItemAtURL:url options:0 error:&error byAccessor:^(NSURL *coordinated) {
            struct stat info;
            if (lstat(coordinated.path.fileSystemRepresentation,&info)==0) {resultCode=2;message=@"Revision already exists; immutable revisions cannot be replaced";return;}
            NSURL *temporary=[[coordinated URLByDeletingLastPathComponent] URLByAppendingPathComponent:[@".settingssync-" stringByAppendingString:NSUUID.UUID.UUIDString]];
            NSError *writeError=nil;
            if (![data writeToURL:temporary options:NSDataWritingWithoutOverwriting error:&writeError]) {resultCode=2;message=writeError.localizedDescription;return;}
            chmod(temporary.path.fileSystemRepresentation,0600);
            int descriptor=open(temporary.path.fileSystemRepresentation,O_RDONLY);
            if (descriptor>=0){fsync(descriptor);close(descriptor);}
            // moveItem refuses an existing destination, preserving immutability.
            if (![[NSFileManager defaultManager] moveItemAtURL:temporary toURL:coordinated error:&writeError]) {resultCode=2;message=writeError.localizedDescription;}
            [[NSFileManager defaultManager] removeItemAtURL:temporary error:nil];
        }];
        if (error) {resultCode=2;message=error.localizedDescription;}
        *code=resultCode;return FSSString(message);
    }
}
