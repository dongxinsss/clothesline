const assert=require('node:assert/strict');
const quitAfterSettings=require('../frontend/quit.js');
(async()=>{
 let quits=0;
 await quitAfterSettings(Promise.resolve(),()=>quits++);
 await quitAfterSettings(Promise.reject(new Error('save failed')),()=>quits++);
 await quitAfterSettings(new Promise(()=>{}),()=>quits++,5);
 assert.equal(quits,3);
 let saved=false;
 await quitAfterSettings(Promise.resolve().then(()=>saved=true),()=>assert.ok(saved));
 await assert.rejects(quitAfterSettings(Promise.resolve(),()=>Promise.reject(new Error('exit failed'))),/exit failed/);
 console.log('Quit works after successful, failed, and stalled settings saves.');
})().catch(e=>{console.error(e);process.exitCode=1});
