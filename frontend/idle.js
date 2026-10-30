(function(root){
 'use strict';
 class IdleController {
  constructor(onIdle,clock={now:()=>Date.now(),set:(fn,ms)=>setTimeout(fn,ms),clear:id=>clearTimeout(id)}){this.onIdle=onIdle;this.clock=clock;this.seconds=0;this.paused=false;this.timer=null;this.last=clock.now()}
  configure(seconds){if(seconds===this.seconds)return;this.seconds=seconds;this.activity()}
  pause(paused){if(paused===this.paused)return;this.paused=paused;this.activity()}
  activity(){this.last=this.clock.now();this.arm()}
  arm(){if(this.timer!==null)this.clock.clear(this.timer);this.timer=null;if(!this.seconds||this.paused)return;const wait=Math.max(1,this.seconds*1000-(this.clock.now()-this.last));this.timer=this.clock.set(()=>{this.timer=null;if(this.paused||!this.seconds)return;if(this.clock.now()-this.last>=this.seconds*1000)this.onIdle();else this.arm()},wait)}
  destroy(){if(this.timer!==null)this.clock.clear(this.timer);this.timer=null}
 }
 if(typeof module!=='undefined'&&module.exports)module.exports=IdleController;else root.IdleController=IdleController;
})(typeof window!=='undefined'?window:globalThis);
