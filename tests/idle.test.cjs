const assert=require('node:assert/strict');
const IdleController=require('../frontend/idle.js');
let now=0,next=0,jobs=new Map(),fired=0;
const clock={now:()=>now,set:(fn,ms)=>{const id=++next;jobs.set(id,{fn,at:now+ms});return id},clear:id=>jobs.delete(id)};
function advance(ms){now+=ms;for(const [id,job] of [...jobs])if(job.at<=now){jobs.delete(id);job.fn()}}
const idle=new IdleController(()=>fired++,clock);
idle.configure(10);advance(9000);assert.equal(fired,0);idle.activity();advance(9000);assert.equal(fired,0);advance(1000);assert.equal(fired,1);
idle.activity();idle.pause(true);advance(100000);assert.equal(fired,1);idle.pause(false);advance(9999);assert.equal(fired,1);advance(1);assert.equal(fired,2);
idle.configure(0);advance(1000000);assert.equal(fired,2);assert.equal(jobs.size,0);
idle.configure(3);advance(2000);idle.configure(8);advance(7000);assert.equal(fired,2);advance(1000);assert.equal(fired,3);
idle.destroy();assert.equal(jobs.size,0);console.log('Idle activity reset, editing/capture pause, custom timeout and never-hide verified.');
