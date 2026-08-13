// Native write access to the general pasteboard.
//
// The previous implementation piped through pbcopy, which interprets its
// stdin according to LC_CTYPE — and a GUI app launched from Dock or Finder
// inherits no locale variables, so pbcopy read UTF-8 bytes as MacRoman and
// stored mojibake ("ö" became "√∂", VOX-15). NSPasteboard takes the string
// as-is; there is no external binary and no locale involved.

#import <Cocoa/Cocoa.h>

// voxWriteGeneralPasteboard replaces the pasteboard contents with the given
// UTF-8 bytes. Returns false if the bytes are not valid UTF-8 or the
// pasteboard rejects the write; the caller reports that as an error instead
// of storing garbage.
_Bool voxWriteGeneralPasteboard(const void *bytes, size_t len) {
	@autoreleasepool {
		NSString *s;
		if (len == 0) {
			s = @"";
		} else {
			s = [[NSString alloc] initWithBytes:bytes
			                             length:len
			                           encoding:NSUTF8StringEncoding];
		}
		if (s == nil) {
			return false;
		}
		NSPasteboard *pb = [NSPasteboard generalPasteboard];
		[pb clearContents];
		return [pb setString:s forType:NSPasteboardTypeString];
	}
}
