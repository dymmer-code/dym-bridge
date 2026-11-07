//go:build darwin

#import <Cocoa/Cocoa.h>
#include "urlhandler_darwin.h"

extern void dymBridgeOpenedURL(char *url);

@interface DymBridgeURLHandler : NSObject
- (void)handleGetURL:(NSAppleEventDescriptor *)event withReplyEvent:(NSAppleEventDescriptor *)reply;
@end

@implementation DymBridgeURLHandler
- (void)handleGetURL:(NSAppleEventDescriptor *)event withReplyEvent:(NSAppleEventDescriptor *)reply {
	NSString *url = [[event paramDescriptorForKeyword:keyDirectObject] stringValue];
	if (url != nil) {
		dymBridgeOpenedURL((char *)[url UTF8String]);
	}
}
@end

static DymBridgeURLHandler *dymBridgeHandler = nil;

void dymBridgeRegisterURLHandler(void) {
	dymBridgeHandler = [[DymBridgeURLHandler alloc] init];
	[[NSAppleEventManager sharedAppleEventManager]
		setEventHandler:dymBridgeHandler
		andSelector:@selector(handleGetURL:withReplyEvent:)
		forEventClass:kInternetEventClass
		andEventID:kAEGetURL];
}
