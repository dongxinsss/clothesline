#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import <QuartzCore/QuartzCore.h>

// Shelf <-> basket transition drawn the way the Dock minimizes windows. The
// real window is hidden (alpha 0) while an overlay animates a snapshot of the
// web view in thin horizontal slices along the genie funnel, or as a plain
// scale when the user picked "Scale effect" in Desktop & Dock.

static const int kSlices = 96;
static const CFTimeInterval kDuration = 0.5;

static NSWindow *animWindow;     // the app window being animated
static NSWindow *overlay;        // borderless, click-through
static CALayer *sliceRoot, *iconLayer;
static NSMutableArray<CALayer *> *slices;
static NSTimer *timer;
static CGImageRef sourceImage;   // snapshot of the large (shelf) state
static NSRect fromFrame;         // window frame before the layout change
static NSInteger generation;

static WKWebView *clFindWebView(NSView *v) {
  if ([v isKindOfClass:WKWebView.class]) return (WKWebView *)v;
  for (NSView *s in v.subviews) { WKWebView *w = clFindWebView(s); if (w) return w; }
  return nil;
}
static NSWindow *clAppWindow(void) {
  for (NSWindow *w in NSApp.windows) if (w.isVisible && clFindWebView(w.contentView)) return w;
  return nil;
}
static void clSetSource(CGImageRef img) {
  if (sourceImage) CGImageRelease(sourceImage);
  sourceImage = img ? CGImageRetain(img) : NULL;
}
static void clFinish(void) {
  [timer invalidate]; timer = nil;
  if (overlay) { [overlay orderOut:nil]; overlay = nil; }
  sliceRoot = nil; iconLayer = nil; slices = nil;
  clSetSource(NULL);
  if (animWindow) { animWindow.alphaValue = 1; animWindow = nil; }
}
static CGImageRef clImage(NSImage *img) { return [img CGImageForProposedRect:NULL context:nil hints:nil]; }

static void clShowOverlay(NSWindow *w, CGImageRef still) {
  NSRect screen = (w.screen ?: NSScreen.mainScreen).frame;
  overlay = [[NSWindow alloc] initWithContentRect:screen styleMask:NSWindowStyleMaskBorderless backing:NSBackingStoreBuffered defer:NO];
  overlay.releasedWhenClosed = NO;
  overlay.animationBehavior = NSWindowAnimationBehaviorNone;
  overlay.opaque = NO;
  overlay.backgroundColor = NSColor.clearColor;
  overlay.hasShadow = NO;
  overlay.ignoresMouseEvents = YES;
  overlay.level = MAX(w.level, NSFloatingWindowLevel);
  overlay.collectionBehavior = NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorStationary | NSWindowCollectionBehaviorIgnoresCycle | NSWindowCollectionBehaviorFullScreenAuxiliary;
  NSView *view = [[NSView alloc] initWithFrame:NSMakeRect(0, 0, screen.size.width, screen.size.height)];
  CALayer *root = [CALayer layer];
  view.layer = root; view.wantsLayer = YES;
  overlay.contentView = view;
  sliceRoot = [CALayer layer]; sliceRoot.frame = root.bounds; [root addSublayer:sliceRoot];
  // The still frame keeps the old state on screen while the window re-lays out.
  iconLayer = [CALayer layer];
  iconLayer.contents = (__bridge id)still;
  iconLayer.frame = NSMakeRect(w.frame.origin.x - screen.origin.x, w.frame.origin.y - screen.origin.y, w.frame.size.width, w.frame.size.height);
  [root addSublayer:iconLayer];
  [overlay orderFront:nil];
}

static double clClamp(double v) { return v < 0 ? 0 : v > 1 ? 1 : v; }
static double clSmooth(double v) { v = clClamp(v); return v * v * (3 - 2 * v); }
static double clLerp(double a, double b, double t) { return a + (b - a) * t; }

// Lays the slices out for progress p: 0 = full window at big, 1 = inside small.
// Coordinates are top-down within the overlay.
static void clLayout(double p, NSRect big, NSRect small, BOOL genie) {
  CGFloat H = sliceRoot.bounds.size.height;
  double bt = H - NSMaxY(big), bb = H - NSMinY(big), bl = NSMinX(big), br = NSMaxX(big);
  double tt = H - NSMaxY(small), tb = H - NSMinY(small), tl = NSMinX(small), tr = NSMaxX(small);
  double curve, top, bottom;
  if (genie) {
    curve = clSmooth(p / 0.45);
    bottom = clLerp(bb, tb, clSmooth((p - 0.2) / 0.6));
    top = clLerp(bt, tt, clSmooth((p - 0.35) / 0.65));
  } else {
    curve = clSmooth(p);
    bottom = clLerp(bb, tb, curve);
    top = clLerp(bt, tt, curve);
  }
  double y0 = bt, y1 = MAX(tt, bt + 1);
  double W = br - bl;
  size_t rows = CGImageGetHeight(sourceImage);
  for (int i = 0; i < kSlices; i++) {
    double f0 = (double)(rows * i / kSlices) / rows, f1 = (double)(rows * (i + 1) / kSlices) / rows;
    double ya = clLerp(top, bottom, f0), yb = clLerp(top, bottom, f1);
    double ka = genie ? clSmooth((ya - y0) / (y1 - y0)) * curve : curve;
    double kb = genie ? clSmooth((yb - y0) / (y1 - y0)) * curve : curve;
    double la = clLerp(bl, tl, ka), ra = clLerp(br, tr, ka), lb = clLerp(bl, tl, kb), rb = clLerp(br, tr, kb);
    double h = MAX(yb - ya, 0.01) + 0.75, sh = (f1 - f0) * big.size.height;
    double ca = (la + ra) / 2, cb = (lb + rb) / 2, w = ((ra - la) + (rb - lb)) / 2;
    double midY = (ya + yb) / 2;
    CALayer *s = slices[i];
    s.bounds = CGRectMake(0, 0, W, MAX(sh, 0.01));
    s.position = CGPointMake((ca + cb) / 2, H - midY);
    // Shear keeps neighbouring slices joined along the curved edges.
    CGAffineTransform m = CGAffineTransformMake(w / W, 0, (ca - cb) / MAX(sh, 0.01), h / MAX(sh, 0.01), 0, 0);
    s.affineTransform = m;
  }
  sliceRoot.opacity = 1 - clSmooth((p - 0.86) / 0.14);
}

static void clAnimate(BOOL expand, NSRect big, NSRect small) {
  NSString *effect = [[[NSUserDefaults alloc] initWithSuiteName:@"com.apple.dock"] stringForKey:@"mineffect"];
  // The funnel only bends downwards, like the Dock at the bottom of the screen.
  BOOL genie = ![effect isEqualToString:@"scale"] && NSMidY(small) < NSMidY(big);
  NSRect screen = overlay.frame;
  big = NSOffsetRect(big, -screen.origin.x, -screen.origin.y);
  small = NSOffsetRect(small, -screen.origin.x, -screen.origin.y);
  slices = [NSMutableArray arrayWithCapacity:kSlices];
  size_t rows = CGImageGetHeight(sourceImage), cols = CGImageGetWidth(sourceImage);
  for (int i = 0; i < kSlices; i++) {
    size_t r0 = rows * i / kSlices, r1 = rows * (i + 1) / kSlices;
    CALayer *s = [CALayer layer];
    CGImageRef part = CGImageCreateWithImageInRect(sourceImage, CGRectMake(0, r0, cols, MAX(r1 - r0, 1)));
    s.contents = (__bridge_transfer id)part;
    s.anchorPoint = CGPointMake(0.5, 0.5);
    // Antialiased top/bottom edges would show as hairline seams between slices.
    s.edgeAntialiasingMask = kCALayerLeftEdge | kCALayerRightEdge;
    [sliceRoot addSublayer:s];
    [slices addObject:s];
  }
  NSInteger gen = generation;
  CFTimeInterval start = CACurrentMediaTime();
  void (^tick)(void) = ^{
    double t = clClamp((CACurrentMediaTime() - start) / kDuration);
    double p = expand ? 1 - t : t;
    [CATransaction begin];
    [CATransaction setDisableActions:YES];
    clLayout(p, big, small, genie);
    // Cross-fade with the small state: the live basket when minimizing, its
    // still frame when restoring (the live window is already the shelf).
    double fade = clSmooth((p - 0.78) / 0.22);
    if (expand) iconLayer.opacity = fade; else iconLayer.hidden = YES;
    [CATransaction commit];
    if (!expand) animWindow.alphaValue = fade;
    if (t >= 1 && gen == generation) clFinish();
  };
  tick();
  timer = [NSTimer timerWithTimeInterval:1.0 / 120 repeats:YES block:^(NSTimer *t) { tick(); }];
  [NSRunLoop.mainRunLoop addTimer:timer forMode:NSRunLoopCommonModes];
}

// Called off the main thread before the layout change. Freezes the current
// look on an overlay and hides the real window. Returns 0 when the caller
// should switch instantly instead.
int clMinimizePrepare(void) {
  if (NSThread.isMainThread) return 0;
  dispatch_semaphore_t done = dispatch_semaphore_create(0);
  __block int result = 0;
  __block BOOL abandoned = NO;
  NSObject *lock = [NSObject new];
  dispatch_async(dispatch_get_main_queue(), ^{
    BOOL running = animWindow != nil;
    clFinish();
    NSWindow *w = clAppWindow();
    WKWebView *web = w ? clFindWebView(w.contentView) : nil;
    if (running || !web || w.alphaValue < 1 || NSWorkspace.sharedWorkspace.accessibilityDisplayShouldReduceMotion) { dispatch_semaphore_signal(done); return; }
    NSInteger gen = ++generation;
    [web takeSnapshotWithConfiguration:nil completionHandler:^(NSImage *image, NSError *error) {
      @synchronized (lock) {
        CGImageRef still = image ? clImage(image) : NULL;
        if (!abandoned && still && gen == generation) {
          animWindow = w;
          fromFrame = w.frame;
          clSetSource(still);
          clShowOverlay(w, still);
          w.alphaValue = 0;
          result = 1;
          // Never leave the window invisible if no run follows.
          dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 3 * NSEC_PER_SEC), dispatch_get_main_queue(), ^{ if (gen == generation && !timer) clFinish(); });
        }
      }
      dispatch_semaphore_signal(done);
    }];
  });
  if (dispatch_semaphore_wait(done, dispatch_time(DISPATCH_TIME_NOW, 400 * NSEC_PER_MSEC))) {
    @synchronized (lock) { abandoned = YES; }
  }
  @synchronized (lock) { return result; }
}

// Called after the window was resized for the new state.
void clMinimizeRun(int expand) {
  NSInteger gen = generation;
  int64_t wait = (expand ? 140 : 50) * NSEC_PER_MSEC;
  dispatch_after(dispatch_time(DISPATCH_TIME_NOW, wait), dispatch_get_main_queue(), ^{
    if (gen != generation || !animWindow || !overlay) return;
    NSRect to = animWindow.frame;
    BOOL shrinking = to.size.width * to.size.height < fromFrame.size.width * fromFrame.size.height;
    if (shrinking == (BOOL)expand) { clFinish(); return; }
    if (!expand) { clAnimate(NO, fromFrame, to); return; }
    WKWebView *web = clFindWebView(animWindow.contentView);
    [web takeSnapshotWithConfiguration:nil completionHandler:^(NSImage *image, NSError *error) {
      CGImageRef big = image ? clImage(image) : NULL;
      if (gen != generation || !overlay) return;
      if (!big) { clFinish(); return; }
      clSetSource(big);
      clAnimate(YES, animWindow.frame, fromFrame);
    }];
  });
}
