#import <Cocoa/Cocoa.h>
#import <Carbon/Carbon.h>
#import <WebKit/WebKit.h>
#import <objc/runtime.h>

extern void clHotkeyPressed(int id);

static NSWindow *clWindow(void) {
  for (NSWindow *w in NSApp.windows) {
    NSMutableArray<NSView *> *views = [NSMutableArray arrayWithObject:w.contentView ?: [NSView new]];
    while (views.count) {
      NSView *v = views.lastObject;
      [views removeLastObject];
      if ([v isKindOfClass:WKWebView.class]) return w;
      [views addObjectsFromArray:v.subviews];
    }
  }
  return nil;
}
static void clOnMain(void (^work)(void)) {
  if (NSThread.isMainThread) work(); else dispatch_sync(dispatch_get_main_queue(), work);
}
static NSScreen *clMainScreen(void) { return NSScreen.screens.firstObject ?: NSScreen.mainScreen; }

// With the app no longer activated on collapse, the first click on the basket
// or a card must act instead of only bringing the app forward.
static BOOL clAcceptsFirstMouse(id self, SEL _cmd, NSEvent *event) { return YES; }
void clWindowSetup(void) {
  dispatch_async(dispatch_get_main_queue(), ^{
    Class c = NSClassFromString(@"WailsWebView") ?: WKWebView.class;
    class_replaceMethod(c, @selector(acceptsFirstMouse:), (IMP)clAcceptsFirstMouse, "c@:@");
  });
}

// Orders the window in without activating the app, so a collapse never takes
// keyboard focus from whatever the user is typing in.
void clShowQuietly(void) {
  dispatch_async(dispatch_get_main_queue(), ^{ [clWindow() orderFrontRegardless]; });
}

// Size of the usable area (without menu bar and Dock) of the window's screen,
// in the same coordinates Wails uses for window positions.
void clWorkArea(int *width, int *height) {
  __block NSSize size;
  clOnMain(^{ size = (clWindow().screen ?: NSScreen.mainScreen).visibleFrame.size; });
  *width = (int)size.width;
  *height = (int)size.height;
}
void clMainScreenSize(int *width, int *height) {
  __block NSSize size;
  clOnMain(^{ size = clMainScreen().frame.size; });
  *width = (int)size.width;
  *height = (int)size.height;
}

// Places the window in main-screen points with a top-left origin, the same
// space screencapture -R uses. Wails positions are relative to the visible
// frame instead, which cannot cover the menu bar.
void clPlaceWindow(int x, int y, int width, int height, int aboveMenuBar) {
  dispatch_async(dispatch_get_main_queue(), ^{
    NSWindow *w = clWindow();
    NSRect screen = clMainScreen().frame;
    [w setFrame:NSMakeRect(screen.origin.x + x, NSMaxY(screen) - y - height, width, height) display:YES animate:NO];
    if (aboveMenuBar) w.level = NSMainMenuWindowLevel + 1;
  });
}

// Only the shelf can be resized from its edges; the basket, panels and the
// capture overlay keep the size the app gives them.
void clSetResizable(int on) {
  dispatch_async(dispatch_get_main_queue(), ^{
    NSWindow *w = clWindow();
    if (on) w.styleMask |= NSWindowStyleMaskResizable;
    else w.styleMask &= ~NSWindowStyleMaskResizable;
  });
}

static EventHotKeyRef hotkeys[2];
static EventHandlerRef hotkeyHandler;
static OSStatus clHotkeyEvent(EventHandlerCallRef next, EventRef event, void *data) {
  EventHotKeyID key;
  if (GetEventParameter(event, kEventParamDirectObject, typeEventHotKeyID, NULL, sizeof(key), NULL, &key) == noErr) clHotkeyPressed((int)key.id);
  return noErr;
}

// Registers ⌃⌥A (capture) and ⌃⌥S (scroll capture). Carbon hotkeys need no
// Accessibility permission. Returns a bit per shortcut another app holds.
int clSetHotkeys(int enabled) {
  __block int failed = 0;
  clOnMain(^{
    for (int i = 0; i < 2; i++) if (hotkeys[i]) { UnregisterEventHotKey(hotkeys[i]); hotkeys[i] = NULL; }
    if (!enabled) return;
    if (!hotkeyHandler) {
      EventTypeSpec spec = {kEventClassKeyboard, kEventHotKeyPressed};
      InstallApplicationEventHandler(&clHotkeyEvent, 1, &spec, NULL, &hotkeyHandler);
    }
    UInt32 keys[2] = {kVK_ANSI_A, kVK_ANSI_S};
    for (int i = 0; i < 2; i++) {
      EventHotKeyID key = {'CLth', (UInt32)i + 1};
      if (RegisterEventHotKey(keys[i], controlKey | optionKey, key, GetApplicationEventTarget(), 0, &hotkeys[i]) != noErr) {
        hotkeys[i] = NULL;
        failed |= 1 << i;
      }
    }
  });
  return failed;
}
