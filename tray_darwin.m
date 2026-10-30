#import <Cocoa/Cocoa.h>
extern void clTrayAction(int action);
extern void clDragEnded(void);
extern void clDragStarted(void);
@interface CLDragSource : NSObject <NSDraggingSource>
@end
@implementation CLDragSource
- (NSDragOperation)draggingSession:(NSDraggingSession *)session sourceOperationMaskForDraggingContext:(NSDraggingContext)context {return NSDragOperationCopy;}
- (void)draggingSession:(NSDraggingSession *)session endedAtPoint:(NSPoint)point operation:(NSDragOperation)operation {clDragEnded();}
@end
static CLDragSource *dragSource;
static id dragMonitor;
@interface CLTray : NSObject
@property(nonatomic,strong) NSStatusItem *item;
- (void)perform:(NSMenuItem *)sender;
@end
@implementation CLTray
- (void)perform:(NSMenuItem *)sender { clTrayAction((int)sender.tag); }
@end
static CLTray *tray;
static NSImage *applicationIcon;
void clSetApplicationIcon(const void *bytes,int len){
  NSData *data=[NSData dataWithBytes:bytes length:len];
  dispatch_async(dispatch_get_main_queue(),^{
    NSImage *icon=[[NSImage alloc]initWithData:data];
    if(icon){applicationIcon=icon;[NSApp setApplicationIconImage:applicationIcon];}
  });
}
void clSetDockVisible(int visible){dispatch_async(dispatch_get_main_queue(),^{[NSApp setActivationPolicy:visible?NSApplicationActivationPolicyRegular:NSApplicationActivationPolicyAccessory];if(applicationIcon)[NSApp setApplicationIconImage:applicationIcon];});}
void clTrayStart(const void *bytes,int len) {
  NSData *data=[NSData dataWithBytes:bytes length:len];
  dispatch_async(dispatch_get_main_queue(), ^{
    tray=[CLTray new];
    tray.item=[[NSStatusBar systemStatusBar] statusItemWithLength:NSSquareStatusItemLength];
    NSImage *icon=[[NSImage alloc] initWithData:data];icon.size=NSMakeSize(18,18);icon.template=YES;
    tray.item.button.image=icon;tray.item.button.toolTip=@"拾光绳 · 随手挂起一点灵感";
    NSMenu *menu=[NSMenu new];
    NSArray *titles=@[@"展开拾光绳",@"截取屏幕…",@"挂上剪贴板内容",@"新建文字便签",@"退出拾光绳"];
    for(int n=0;n<5;n++){if(n==4)[menu addItem:[NSMenuItem separatorItem]];NSMenuItem *item=[[NSMenuItem alloc]initWithTitle:titles[n] action:@selector(perform:) keyEquivalent:@""];item.target=tray;item.tag=n+1;[menu addItem:item];}
    tray.item.menu=menu;
  });
}
void clTrayStop(void){dispatch_async(dispatch_get_main_queue(),^{if(tray){[[NSStatusBar systemStatusBar]removeStatusItem:tray.item];tray=nil;}});}
void clBeginDrag(const char *path,const void *text,int textLen,const void *thumb,int thumbLen){
  NSString *filePath=[NSString stringWithUTF8String:path];
  NSString *value=[[NSString alloc]initWithBytes:text length:textLen encoding:NSUTF8StringEncoding];
  NSData *thumbnail=[NSData dataWithBytes:thumb length:thumbLen];
  void (^begin)(void)=^{
    NSWindow *window=NSApp.keyWindow;
    if(!window||!([NSEvent pressedMouseButtons]&1)){clDragEnded();return;}
    id<NSPasteboardWriting> writer=filePath.length?[NSURL fileURLWithPath:filePath]:value;
    if(!writer){clDragEnded();return;}
    NSPoint origin=[window mouseLocationOutsideOfEventStream];
    if(dragMonitor){[NSEvent removeMonitor:dragMonitor];dragMonitor=nil;}
    dragMonitor=[NSEvent addLocalMonitorForEventsMatchingMask:(NSEventMaskLeftMouseDragged|NSEventMaskLeftMouseUp) handler:^NSEvent *(NSEvent *event){
    if(event.type==NSEventTypeLeftMouseUp){[NSEvent removeMonitor:dragMonitor];dragMonitor=nil;clDragEnded();return event;}
    NSPoint point=[window mouseLocationOutsideOfEventStream];if(hypot(point.x-origin.x,point.y-origin.y)<6)return event;
    [NSEvent removeMonitor:dragMonitor];dragMonitor=nil;clDragStarted();
    NSImage *image=[[NSImage alloc]initWithData:thumbnail];
    if(!image){
      image=[[NSImage alloc]initWithSize:NSMakeSize(164,44)];
      [image lockFocus];[[NSColor controlBackgroundColor]setFill];[[NSBezierPath bezierPathWithRoundedRect:NSMakeRect(0,0,164,44) xRadius:8 yRadius:8]fill];
      NSString *label=value.length?[value substringWithRange:[value rangeOfComposedCharacterSequencesForRange:NSMakeRange(0,MIN(16,value.length))]]:@"图片";
      [label drawInRect:NSMakeRect(10,12,144,22) withAttributes:@{NSFontAttributeName:[NSFont systemFontOfSize:13],NSForegroundColorAttributeName:NSColor.labelColor}];[image unlockFocus];
    }
    NSSize size=image.size;CGFloat scale=MIN(164.0/MAX(size.width,1),110.0/MAX(size.height,1));size=NSMakeSize(size.width*scale,size.height*scale);
    NSView *view=window.contentView;NSPoint local=[view convertPoint:point fromView:nil];
    NSDraggingItem *item=[[NSDraggingItem alloc]initWithPasteboardWriter:writer];[item setDraggingFrame:NSMakeRect(local.x-12,local.y-size.height+12,size.width,size.height) contents:image];
    if(!dragSource)dragSource=[CLDragSource new];[view beginDraggingSessionWithItems:@[item] event:event source:dragSource];
    return nil;
    }];
  };
  if(NSThread.isMainThread)begin();else dispatch_sync(dispatch_get_main_queue(),begin);
}
