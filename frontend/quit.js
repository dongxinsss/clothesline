(function(root){
 'use strict';
 // A failed or stuck settings save must never disable the exit button.
 async function quitAfterSettings(queue,quit,waitMs=250){
  let timer;
  try{await Promise.race([Promise.resolve(queue).catch(()=>{}),new Promise(resolve=>{timer=setTimeout(resolve,waitMs)})])}
  finally{clearTimeout(timer)}
  return quit();
 }
 if(typeof module!=='undefined'&&module.exports)module.exports=quitAfterSettings;else root.quitAfterSettings=quitAfterSettings;
})(typeof window!=='undefined'?window:globalThis);
