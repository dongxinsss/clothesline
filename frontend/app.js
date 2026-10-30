/* No framework, no remote assets, no telemetry. */
'use strict';
const $=s=>document.querySelector(s), $$=s=>[...document.querySelectorAll(s)];
const native=!!window.go?.main?.App, api=window.go?.main?.App;
const defaultSettings={autoCollect:true,alwaysOnTop:true,heightPercent:20,theme:'blue',fontSize:14,noteFontSize:15,metaFontSize:11,cardWidth:180,showDockIcon:true,idleSeconds:0};
const themes=[{id:'blue',name:'晴空浅蓝',bg:'#edf5fa',ink:'#64869c',note:'#ffefbf'},{id:'dark',name:'静夜深色',bg:'#233642',ink:'#b7d7e6',note:'#547180'},{id:'paper',name:'暖纸米白',bg:'#f3eee3',ink:'#897459',note:'#e7d7b2'},{id:'contrast',name:'清晰高对比',bg:'#fff',ink:'#143e59',note:'#b1d8f0'}];
let state={items:[],history:[],schedules:[],settings:{...defaultSettings}},filter='all',toastTimer,mode='shelf',panelKind='',platform='',previewFromHistory=false,historyQuery='',historyFilter='all',historyScope='all',historyLimit=40,settingsQueue=Promise.resolve(),pendingSettings=null,historyRefresh=null,scheduleRefresh=null,scheduleView='day',scheduleAnchor=ScheduleDates.iso(new Date());
let nativeDragActive=false,captureBusy=false,hotkeys=['⌃⌥A','⌃⌥S'];
const idle=new IdleController(()=>run(async()=>{if(native){if(!await api.AutoCollapse())idle.activity()}else await collapse()}));
function updateIdle(){idle.configure(state.settings.idleSeconds||0);idle.pause(mode!=='shelf'||!!panelKind||nativeDragActive||captureBusy)}
function applyAppearance(){
 state.settings={...defaultSettings,...state.settings};state.history=state.history||[];state.schedules=state.schedules||[];
 const root=document.documentElement,s=state.settings;root.dataset.theme=s.theme;root.dataset.largeFonts=String(s.fontSize>=18);
 for(const [key,name] of [['fontSize','--ui-size'],['noteFontSize','--note-size'],['metaFontSize','--meta-size'],['cardWidth','--card-width']])root.style.setProperty(name,s[key]+'px');
 document.querySelector('meta[name="color-scheme"]').content='light';
 updateIdle();
}
// Changes that arrive during a drag are drawn once it ends, so the line never
// stays stale if the end notice is missed.
function endNativeDrag(){if(!nativeDragActive)return;nativeDragActive=false;idle.activity();updateIdle();render()}
// Windows' drag loop owns the mouse, so a press reaching the page means it
// is over.
document.addEventListener('pointerdown',()=>{if(platform!=='darwin')endNativeDrag()},true);
function makeDraggable(element,id,text=null){
 if(!native)return;let down=null,started=false;
 element.title='点击查看 · 按住拖到聊天框或其他应用';element.classList.add('can-drag-out');
 if(text!==null){
  element.draggable=true;element.addEventListener('dragstart',e=>{started=true;e.dataTransfer.effectAllowed='copy';e.dataTransfer.setData('text/plain',text);nativeDragActive=true;updateIdle()});
  element.addEventListener('dragend',()=>{nativeDragActive=false;idle.activity();updateIdle();render();setTimeout(()=>started=false,250)});
  element.addEventListener('click',e=>{if(started){e.preventDefault();e.stopImmediatePropagation()}},true);return;
 }
 if(platform==='darwin'){
  element.draggable=false;element.addEventListener('pointerdown',e=>{if(e.button===0)api.DragItem(id).catch(e=>toast(e?.message||String(e)))});return;
 }
 element.draggable=true;
 element.addEventListener('dragstart',e=>{e.preventDefault();if(started||nativeDragActive)return;started=true;nativeDragActive=true;updateIdle();api.DragItem(id).catch(e=>toast(e?.message||String(e))).finally(()=>{endNativeDrag();setTimeout(()=>started=false,250)})});
 element.addEventListener('click',e=>{if(started||nativeDragActive){e.preventDefault();e.stopImmediatePropagation();started=false}},true);
}
const media=(i,thumb=false)=>i.demoImage||`/media/${i.id}.${thumb?'jpg':'png'}`;
const icon=name=>{const s=document.createElementNS('http://www.w3.org/2000/svg','svg'),u=document.createElementNS(s.namespaceURI,'use');u.setAttribute('href','#i-'+name);s.append(u);return s;};
function button(name,title,fn,cls='icon-button'){const b=document.createElement('button');b.className=cls;b.title=title;b.setAttribute('aria-label',title);b.append(icon(name));b.onclick=e=>{e.stopPropagation();run(fn,b)};return b;}
async function run(fn,b){try{if(b)b.disabled=true;await fn()}catch(e){toast(e?.message||String(e))}finally{if(b)b.disabled=false}}
function toast(message,undo){clearTimeout(toastTimer);const t=$('#toast');t.replaceChildren(document.createTextNode(message));if(undo){const b=document.createElement('button');b.textContent='撤销';b.onclick=()=>run(async()=>{if(native)await api.UndoRemove();else{state.items=JSON.parse(sessionStorage.getItem('cl-undo')||'[]');const restored=new Set(state.items.map(i=>i.id));state.history=state.history.filter(i=>!restored.has(i.id));saveDemo();render()}t.hidden=true});t.append(b)}t.hidden=false;toastTimer=setTimeout(()=>t.hidden=true,undo?6500:3300)}
function timeLabel(v){const d=new Date(v),today=new Date();return `${d.toDateString()===today.toDateString()?'今天':d.toLocaleDateString('zh-CN',{month:'numeric',day:'numeric'})} ${d.toLocaleTimeString('zh-CN',{hour:'2-digit',minute:'2-digit',hour12:false})}`}
const cardNodes=new Map();
function cardNode(i){
  let wrap=cardNodes.get(i.id);
  if(!wrap){
    wrap=document.createElement('article');wrap.className='card-wrap fresh';wrap.setAttribute('role','listitem');wrap.addEventListener('animationend',()=>wrap.classList.remove('fresh'),{once:true});
    const clip=document.createElement('span');clip.className='clip';clip.setAttribute('aria-hidden','true');
    const card=document.createElement('button');card.className='card'+(i.kind==='text'?' text-card':'');card.title='点击放大查看';
    if(i.kind==='image'){const img=document.createElement('img');img.src=media(i,true);img.alt='';img.loading='lazy';img.decoding='async';card.append(img)}else{const p=document.createElement('p');p.textContent=i.text;card.append(p)}
    card.onclick=()=>run(()=>preview(i));
    makeDraggable(card,i.id,i.kind==='text'?i.text:null);
    const meta=document.createElement('span');meta.className='card-meta';
    const tools=document.createElement('div');tools.className='card-tools';tools.append(button('copy','复制',()=>copy(i)),button('download','保存到文件',()=>exportItem(i)),button('trash','移除卡片',()=>remove(i),'icon-button remove'));
    wrap.append(card,clip,meta,tools);cardNodes.set(i.id,wrap);
  }
  // Labels say 今天 or a date, so refresh them on every render.
  wrap.firstChild.setAttribute('aria-label',i.kind==='text'?`查看便签：${i.text.slice(0,60)}`:`查看截图，${timeLabel(i.createdAt)}`);
  wrap.querySelector('.card-meta').textContent=`${timeLabel(i.createdAt)} · ${i.source}`;
  return wrap;
}
function render(scroll=false){
  applyAppearance();
  if(nativeDragActive)return;
  $('#count').textContent=state.items.length;$('#basket-count').textContent=state.items.length;$('#auto').setAttribute('aria-pressed',state.settings.autoCollect);
  const cards=$('#cards'),previous=cards.scrollLeft;const items=state.items.filter(i=>filter==='all'||i.kind===filter);
  $('#empty').hidden=items.length>0;
  if(state.items.length&&!items.length){$('#empty strong').textContent=filter==='image'?'还没有截图':'还没有小纸条';$('#empty div>span').textContent=filter==='image'?'截一张图，或拖一张图片进来':'点击上方的文字按钮，写下第一条灵感'}else{$('#empty strong').textContent='把刚刚的灵感，挂在这里';$('#empty div>span').textContent='截个图 · 拖入图片 · 或写一张小纸条'}
  // Cards are kept by id so only new ones swing in; rebuilding every card on
  // each change replayed the animation across the whole line.
  const live=new Set(state.items.map(i=>i.id));for(const id of cardNodes.keys())if(!live.has(id))cardNodes.delete(id);
  const nodes=items.map(cardNode);
  if(nodes.length!==cards.children.length||nodes.some((n,k)=>cards.children[k]!==n))cards.replaceChildren(...nodes);
  cards.scrollLeft=scroll?cards.scrollWidth:previous;
  $('#timeline').textContent=state.items.length?'从左到右，时间慢慢走 · '+state.items.length+' 段拾光':'从左到右，时间慢慢走';
}
async function updateSettings(patch){
 const next={...state.settings,...patch};state.settings=next;applyAppearance();
 if(native){pendingSettings=next;const job=settingsQueue.catch(()=>{}).then(()=>api.SaveSettings(next));settingsQueue=job;try{await job}finally{if(pendingSettings===next)pendingSettings=null}}
 else{saveDemo();render()}
 return next;
}
async function showPanel(title,kind){panelKind=kind;syncResize(false);updateIdle();$('#panel-title').textContent=title;$('#panel-content').replaceChildren();if(native)await api.SetPanel(true);else parent.postMessage({clothesline:'panel',open:true},location.origin);if(!$('#panel').open)$('#panel').showModal()}
async function closePanel(){if(!$('#panel').open)return;await settingsQueue.catch(()=>{});if(panelKind==='schedule-edit'){await showSchedules();return}if(panelKind==='preview'&&previewFromHistory){previewFromHistory=false;await showHistory();return}$('#panel').close();panelKind='';updateIdle();if(native){await api.SetPanel(false)}else{parent.postMessage({clothesline:'panel',open:false},location.origin)}syncResize()}
async function compose(){
  await showPanel('写一张小纸条','compose');const text=document.createElement('textarea');text.className='note-input';text.placeholder='留下一点想法，等会儿继续。';text.maxLength=20000;text.setAttribute('aria-label','便签内容');
  const actions=document.createElement('div');actions.className='panel-actions';const hint=document.createElement('span');hint.className='hint';hint.textContent='⌘ / Ctrl + Enter 挂上去';
  const cancel=document.createElement('button');cancel.className='secondary-button';cancel.textContent='取消';cancel.onclick=()=>run(closePanel);
  const submit=document.createElement('button');submit.className='primary-button';submit.textContent='挂上去';submit.onclick=()=>run(async()=>{if(!text.value.trim()){text.focus();return}if(native){await api.AddText(text.value)}else{addDemo({kind:'text',text:text.value.trim(),source:'便签'})}await closePanel();toast('小纸条已经挂好了')},submit);
  text.onkeydown=e=>{if((e.metaKey||e.ctrlKey)&&e.key==='Enter'){e.preventDefault();submit.click()}};
  actions.append(hint,cancel,submit);$('#panel-content').append(text,actions);setTimeout(()=>text.focus(),100);
}
async function preview(i,fromHistory=false){
  previewFromHistory=fromHistory;
  await showPanel(i.kind==='text'?'一张小纸条':'一帧拾光','preview');const content=$('#panel-content');
  if(i.kind==='image'){const img=document.createElement('img');img.className='preview-image';img.src=media(i);img.alt='截图原图';content.append(img)}else{const p=document.createElement('pre');p.className='preview-text';p.textContent=i.text;content.append(p)}
  const actions=document.createElement('div');actions.className='panel-actions';const hint=document.createElement('span');hint.className='hint';hint.textContent=`${timeLabel(i.createdAt)} · ${i.source}${i.width?` · ${i.width} × ${i.height}`:''}`;
  const save=document.createElement('button');save.className='secondary-button';save.textContent='保存到文件';save.onclick=()=>run(()=>exportItem(i),save);
  const cp=document.createElement('button');cp.className='primary-button';cp.textContent='复制内容';cp.onclick=()=>run(()=>copy(i),cp);actions.append(hint,save,cp);content.append(actions);
}
async function settings(){
  await showPanel('让它刚刚好','settings');const content=$('#panel-content');
  const appearance=document.createElement('section');appearance.className='settings-section';
  const heading=document.createElement('h2');heading.className='section-title';heading.textContent='选择你的主题';
  const desc=document.createElement('p');desc.className='section-description';desc.textContent='颜色与字号会即时预览，关闭并重开应用后仍会保留。';
  const options=document.createElement('div');options.className='theme-options';options.setAttribute('role','radiogroup');options.setAttribute('aria-label','外观主题');
  for(const t of themes){const b=document.createElement('button');b.className='theme-option';b.setAttribute('role','radio');b.setAttribute('aria-label',t.name);b.setAttribute('aria-checked',state.settings.theme===t.id);const swatch=document.createElement('span');swatch.className='theme-swatch';swatch.style.setProperty('--sample-bg',t.bg);swatch.style.setProperty('--sample-ink',t.ink);swatch.style.setProperty('--sample-note',t.note);swatch.append(document.createElement('i'),document.createElement('i'));const label=document.createElement('span');label.className='theme-name';label.textContent=t.name;b.append(swatch,label);b.onclick=()=>run(async()=>{await updateSettings({theme:t.id});[...options.children].forEach(x=>x.setAttribute('aria-checked',x===b))},b);options.append(b)}
  appearance.append(heading,desc,options);content.append(appearance);
  function row(title,description){const div=document.createElement('div');div.className='settings-row';const label=document.createElement('div'),strong=document.createElement('strong'),p=document.createElement('p');strong.textContent=title;p.textContent=description;label.append(strong,p);div.append(label);content.append(div);return div}
  for(const [key,title,description,min,max,unit] of [['fontSize','界面字号','按钮、设置和历史记录的界面文字。',12,20,'px'],['noteFontSize','便签与正文字号','挂绳小纸条、文字编辑器和放大阅读。',12,26,'px'],['metaFontSize','时间与说明字号','卡片时间、来源和底部提示。',10,16,'px'],['cardWidth','卡片宽度','卡片更宽时，文字能在一行里显示得更多。',140,260,'px']]){
    const r=row(title,description),control=document.createElement('div');control.className='range-control';const input=document.createElement('input');input.type='range';input.min=min;input.max=max;input.step=key==='cardWidth'?10:1;input.value=state.settings[key];input.setAttribute('aria-label',title);const value=document.createElement('output');value.textContent=input.value+' '+unit;
    input.oninput=()=>{state.settings={...state.settings,[key]:Number(input.value)};value.textContent=input.value+' '+unit;applyAppearance()};input.onchange=()=>run(()=>updateSettings({[key]:Number(input.value)}));control.append(input,value);r.append(control);
  }
  const sample=document.createElement('div');sample.className='size-sample';sample.textContent='字号预览：把截图挂起来，把想法留下来。Aa 0123456789';content.append(sample);
  for(const [key,title,desc] of [['autoCollect','自动收集截图','接住剪贴板里新出现的图片；普通复制的文字不会自动记录。'],['alwaysOnTop','始终浮在上面','挂绳和小篮子在其他窗口之上，随手就能够到。']]){
    const r=row(title,desc),b=document.createElement('button');b.className='toggle';b.setAttribute('role','switch');b.setAttribute('aria-label',title);b.setAttribute('aria-checked',state.settings[key]);b.onclick=()=>run(async()=>{const v=await updateSettings({[key]:!state.settings[key]});b.setAttribute('aria-checked',v[key])},b);r.append(b);
  }
  {const r=row('全局快捷键',`${hotkeys[0]} 截图 · ${hotkeys[1]} 滚动截图，在任何应用中都可用。`),b=document.createElement('button');b.className='toggle';b.setAttribute('role','switch');b.setAttribute('aria-label','全局快捷键');b.setAttribute('aria-checked',!state.settings.disableHotkeys);b.onclick=()=>run(async()=>{const v=await updateSettings({disableHotkeys:!state.settings.disableHotkeys});b.setAttribute('aria-checked',!v.disableHotkeys)},b);r.append(b)}
  if(platform==='darwin'){const r=row('在 macOS Dock 显示图标','关闭后仍可通过菜单栏和常驻小入口打开。'),b=document.createElement('button');b.className='toggle';b.setAttribute('role','switch');b.setAttribute('aria-label','在 macOS Dock 显示图标');b.setAttribute('aria-checked',state.settings.showDockIcon);b.onclick=()=>run(async()=>{const v=await updateSettings({showDockIcon:!state.settings.showDockIcon});b.setAttribute('aria-checked',v.showDockIcon)},b);r.append(b)}
  const r=row('展开时的高度','按当前屏幕的可用高度（不含菜单栏和程序坞）计算。也可以拖动挂绳的边缘自由调整大小，会记住；在这里重新选择则恢复默认大小。'),select=document.createElement('select');select.setAttribute('aria-label','展开时的屏幕占比');for(const n of [12,15,20,25,30]){const o=document.createElement('option');o.value=n;o.textContent=n+'%'+(n===20?' · 默认':'');select.append(o)}select.value=state.settings.heightPercent;select.onchange=()=>run(()=>updateSettings({heightPercent:Number(select.value)}));r.append(select);
  const idleRow=row('无操作后自动收起','只在挂绳空闲时收进小入口；编辑、截图和拖出卡片时暂停。'),idleControls=document.createElement('div');idleControls.className='idle-controls';const timeout=document.createElement('select');timeout.setAttribute('aria-label','无操作后自动收起');
  for(const [v,label] of [['0','永不隐藏'],['10','10 秒'],['30','30 秒'],['60','1 分钟'],['180','3 分钟'],['300','5 分钟'],['600','10 分钟'],['custom','自定义秒数']]){const o=document.createElement('option');o.value=v;o.textContent=label;timeout.append(o)}
  const custom=document.createElement('input');custom.type='number';custom.min='1';custom.max='86400';custom.step='1';custom.setAttribute('aria-label','自动收起自定义秒数');const current=state.settings.idleSeconds||0;const known=[0,10,30,60,180,300,600].includes(current);timeout.value=known?String(current):'custom';custom.value=current||60;custom.hidden=timeout.value!=='custom';
  timeout.onchange=()=>{custom.hidden=timeout.value!=='custom';if(!custom.hidden)custom.focus();run(()=>updateSettings({idleSeconds:timeout.value==='custom'?Number(custom.value):Number(timeout.value)}))};custom.onchange=()=>run(()=>{const n=Number(custom.value);if(!Number.isInteger(n)||n<1||n>86400)throw Error('请输入 1 到 86400 秒，或选择永不隐藏');return updateSettings({idleSeconds:n})});idleControls.append(timeout,custom);idleRow.append(idleControls);
  const historyRow=row('历史记录',`当前挂绳 ${state.items.length} 条，已移入历史 ${state.historyCount??state.history.length} 条。不会自动删除，可在历史记录中永久删除。`),viewHistory=document.createElement('button');viewHistory.className='secondary-button';viewHistory.textContent='查看历史';viewHistory.onclick=()=>run(showHistory);historyRow.append(viewHistory);
  const recoveryRow=row('找回旧版记录','选择包含 shelf.json 的旧数据目录进行合并，原目录和已有记录都会保留。'),recover=document.createElement('button');recover.className='secondary-button';recover.textContent='导入旧记录';recover.onclick=()=>run(async()=>native?api.ImportHistory():toast('旧记录恢复在桌面版中可用'),recover);recoveryRow.append(recover);
  const info=document.createElement('p');info.className='settings-foot';info.textContent='截图和文字只保存在本机。每次保存都会保留上次有效备份；从挂绳移除的内容可在历史中找回。';
  const path=document.createElement('div');path.className='data-path';path.textContent='固定数据目录：'+(state.dataDir||'浏览器演示存储，与桌面版分开');
  const end=document.createElement('div');end.className='settings-end';const ver=document.createElement('span');ver.className='version';ver.textContent='拾光绳 0.4.1 · '+(native?'本地预览版':'交互演示');const quit=document.createElement('button');quit.className='quit';quit.textContent='退出拾光绳';quit.onclick=()=>run(async()=>{if(native)await quitAfterSettings(settingsQueue,()=>api.Quit());else toast('这是交互演示，可直接关闭浏览器标签页')});end.append(ver,quit);content.append(info,path,end);
}
async function showHistory(){
  previewFromHistory=false;await showPanel('每一段拾光，都留着','history');const content=$('#panel-content');
  const tools=document.createElement('div');tools.className='history-tools';const search=document.createElement('input');search.className='history-search';search.type='search';search.placeholder='搜索文字、来源或日期';search.setAttribute('aria-label','搜索历史记录');search.value=historyQuery;
  const kind=document.createElement('select');kind.setAttribute('aria-label','历史记录类型');for(const [value,label] of [['all','所有类型'],['image','截图与图片'],['text','文字便签']]){const o=document.createElement('option');o.value=value;o.textContent=label;kind.append(o)}kind.value=historyFilter;
  const scope=document.createElement('select');scope.setAttribute('aria-label','历史记录范围');for(const [value,label] of [['all','全部历史'],['active','挂绳上的'],['archived','已移入历史']]){const o=document.createElement('option');o.value=value;o.textContent=label;scope.append(o)}scope.value=historyScope;
  const summary=document.createElement('div');summary.className='history-summary';const grid=document.createElement('div');grid.className='history-grid';grid.setAttribute('role','list');grid.setAttribute('aria-label','历史记录列表');
  const more=document.createElement('button');more.className='secondary-button history-more';more.textContent='再显示 40 条';
  const clear=document.createElement('button');clear.className='secondary-button history-clear';clear.textContent='清空已移入历史';clear.onclick=()=>run(()=>confirmTwice(clear,'再点一次，永久删除',async()=>{if(native){await api.PurgeHistory([]);await refresh()}else{state.history=[];saveDemo();draw()}}),clear);
  async function refresh(){if(native)state.history=await api.GetHistory();draw()}
  function draw(){
    clear.hidden=!state.history.length;
    const current=new Set(state.items.map(i=>i.id)),all=[...state.items,...(state.history||[])].sort((a,b)=>new Date(b.createdAt)-new Date(a.createdAt)),q=historyQuery.trim().toLocaleLowerCase();
    const matches=all.filter(i=>(historyFilter==='all'||i.kind===historyFilter)&&(historyScope==='all'||(historyScope==='active')===current.has(i.id))&&(!q||[i.text||'',i.source,timeLabel(i.createdAt),new Date(i.createdAt).toLocaleDateString('zh-CN')].join(' ').toLocaleLowerCase().includes(q)));
    summary.textContent=`共 ${all.length} 条本地记录 · 筛选出 ${matches.length} 条 · 重启后仍可查看`;grid.replaceChildren();more.hidden=matches.length<=historyLimit;
    if(!matches.length){const empty=document.createElement('div');empty.className='history-empty';empty.textContent=q?'没有找到匹配的记录，试试其他关键词。':'这里会保留你收纳过的截图和文字。';grid.append(empty)}
    for(const i of matches.slice(0,historyLimit)){
      const card=document.createElement('article');card.className='history-item';card.setAttribute('role','listitem');const open=document.createElement('button');open.className='history-open';open.setAttribute('aria-label',i.kind==='text'?'查看历史便签：'+i.text.slice(0,50):'查看历史图片：'+timeLabel(i.createdAt));
      if(i.kind==='image'){const img=document.createElement('img');img.src=media(i,true);img.alt='';img.loading='lazy';open.append(img)}else{const text=document.createElement('p');text.textContent=i.text;open.append(text)}open.onclick=()=>run(()=>preview(i,true));makeDraggable(open,i.id,i.kind==='text'?i.text:null);
      const meta=document.createElement('div');meta.className='history-item-meta';meta.textContent=`${timeLabel(i.createdAt)} · ${i.source} · ${current.has(i.id)?'在挂绳上':'已移入历史'}`;
      const actions=document.createElement('div');actions.className='history-item-actions';if(!current.has(i.id)){const restore=document.createElement('button');restore.className='restore-button';restore.textContent='重新挂上';restore.onclick=()=>run(async()=>{if(native){await api.RestoreItem(i.id);state=await api.GetState();render()}else{state.history=state.history.filter(x=>x.id!==i.id);state.items.push(i);state.items.sort((a,b)=>new Date(a.createdAt)-new Date(b.createdAt));saveDemo();render()}draw();toast('已经重新挂好了')},restore);actions.append(restore)}actions.append(button('copy','复制历史内容',()=>copy(i)),button('download','保存历史内容',()=>exportItem(i)));if(!current.has(i.id)){const purge=button('trash','永久删除',()=>confirmTwice(purge,'',async()=>{if(native){await api.PurgeHistory([i.id]);await refresh()}else{state.history=state.history.filter(x=>x.id!==i.id);saveDemo();draw()}}),'icon-button remove');actions.append(purge)}card.append(open,meta,actions);grid.append(card);
    }
  }
  search.oninput=()=>{historyQuery=search.value;historyLimit=40;draw()};kind.onchange=()=>{historyFilter=kind.value;historyLimit=40;draw()};scope.onchange=()=>{historyScope=scope.value;historyLimit=40;draw()};more.onclick=()=>{historyLimit+=40;draw()};tools.append(search,kind,scope,clear);content.append(tools,summary,grid,more);historyRefresh=refresh;await refresh();
}
// Destructive buttons ask for a second click within a few seconds.
async function confirmTwice(b,label,fn){
  if(b.dataset.armed){delete b.dataset.armed;clearTimeout(b._disarm);if(b._label!==undefined)b.textContent=b._label;b.classList.remove('armed');await fn();return}
  b.dataset.armed='1';b.classList.add('armed');if(label){b._label=b.textContent;b.textContent=label}else toast('再点一次即可永久删除，无法撤销');
  b._disarm=setTimeout(()=>{delete b.dataset.armed;b.classList.remove('armed');if(label)b.textContent=b._label},3500);
}
async function copy(i){if(native){await api.CopyItem(i.id)}else{if(i.kind==='text'){await navigator.clipboard.writeText(i.text)}else{const b=await(await fetch(media(i))).blob();if(!window.ClipboardItem)throw Error('图片复制请在桌面版使用');await navigator.clipboard.write([new ClipboardItem({[b.type]:b})])}}toast('已复制，可以粘贴到其他应用')}
async function showSchedules(){
 await showPanel('给时间留一个位置','schedule');const content=$('#panel-content');
 const tools=document.createElement('div');tools.className='schedule-tools';const tabs=document.createElement('nav');tabs.className='schedule-tabs';tabs.setAttribute('aria-label','日程查看范围');
 for(const [value,label] of [['day','当天'],['week','本周'],['month','本月']]){const b=document.createElement('button');b.textContent=label;b.setAttribute('aria-pressed',scheduleView===value);b.onclick=()=>{scheduleView=value;scheduleAnchor=ScheduleDates.iso(new Date());run(showSchedules)};tabs.append(b)}
 const anchor=document.createElement('input');anchor.type='date';anchor.value=scheduleAnchor;anchor.setAttribute('aria-label','日程日期');anchor.onchange=()=>{if(anchor.value){scheduleAnchor=anchor.value;draw()}};
 const add=document.createElement('button');add.className='primary-button';add.textContent='新建日程';add.onclick=()=>run(()=>editSchedule());tools.append(tabs,anchor,add);
 const navigation=document.createElement('div');navigation.className='schedule-navigation';const previous=document.createElement('button');previous.className='secondary-button';previous.textContent='←';previous.setAttribute('aria-label','上一个周期');previous.onclick=()=>{scheduleAnchor=ScheduleDates.shift(scheduleAnchor,scheduleView,-1);anchor.value=scheduleAnchor;draw()};const title=document.createElement('strong');const next=document.createElement('button');next.className='secondary-button';next.textContent='→';next.setAttribute('aria-label','下一个周期');next.onclick=()=>{scheduleAnchor=ScheduleDates.shift(scheduleAnchor,scheduleView,1);anchor.value=scheduleAnchor;draw()};navigation.append(previous,title,next);
 const list=document.createElement('div');list.className='schedule-list';list.setAttribute('role','list');list.setAttribute('aria-label','日程列表');
 function draw(){
  const [start,end]=ScheduleDates.range(scheduleAnchor,scheduleView);title.textContent=start===end?start:`${start} — ${end}`;const entries=(state.schedules||[]).filter(i=>i.startDate<=end&&i.endDate>=start).sort((a,b)=>a.startDate.localeCompare(b.startDate)||(a.atTime||'').localeCompare(b.atTime||''));list.replaceChildren();
  if(!entries.length){const p=document.createElement('p');p.className='history-empty';p.textContent='这段时间还没有日程，留一个想做的事吧。';list.append(p)}
  for(const i of entries){const card=document.createElement('article');card.className='schedule-entry'+(i.done?' done':'');card.setAttribute('role','listitem');const check=document.createElement('input');check.type='checkbox';check.checked=i.done;check.setAttribute('aria-label',(i.done?'标记未完成：':'标记完成：')+i.title);check.onchange=()=>run(async()=>{if(native){await api.SetScheduleDone(i.id,check.checked);state=await api.GetState()}else{i.done=check.checked;saveDemo()}draw()},check);
   const text=document.createElement('button');text.className='schedule-entry-main';text.setAttribute('aria-label','编辑日程：'+i.title);text.onclick=()=>run(()=>editSchedule(i));const heading=document.createElement('strong');heading.textContent=i.title;const meta=document.createElement('span');meta.className='schedule-entry-meta';meta.textContent=(i.startDate===i.endDate?i.startDate:`${i.startDate} 至 ${i.endDate}`)+' · '+(i.atTime||'全天')+(i.done?' · 已完成':'');text.append(heading,meta);if(i.notes){const notes=document.createElement('p');notes.textContent=i.notes;text.append(notes)}card.append(check,text);list.append(card)}
 }
 content.append(tools,navigation,list);scheduleRefresh=draw;draw();
}
async function editSchedule(entry){
 await showPanel(entry?'编辑日程':'新建一个日程','schedule-edit');const content=$('#panel-content'),form=document.createElement('form');form.className='schedule-form';
 function field(label,type,value,required=false){const wrap=document.createElement('label');wrap.className='schedule-field';const text=document.createElement('span');text.textContent=label;const input=document.createElement('input');input.type=type;input.value=value||'';input.required=required;input.setAttribute('aria-label',label);wrap.append(text,input);return {wrap,input}}
 const title=field('日程标题','text',entry?.title,true);title.input.maxLength=200;title.input.placeholder='例如：项目评审 / 本周学习计划';
 const dates=document.createElement('div');dates.className='schedule-date-fields';const start=field('开始日期','date',entry?.startDate||scheduleAnchor,true),end=field('结束日期','date',entry?.endDate||scheduleAnchor,true),time=field('时间（留空为全天）','time',entry?.atTime||'');start.input.onchange=()=>{if(end.input.value<start.input.value)end.input.value=start.input.value};dates.append(start.wrap,end.wrap,time.wrap);
 const quick=document.createElement('div');quick.className='schedule-quick';for(const [value,label] of [['day','仅当天'],['week','连续 7 天'],['month','至本月末']]){const b=document.createElement('button');b.type='button';b.textContent=label;b.onclick=()=>{const d=ScheduleDates.parse(start.input.value||scheduleAnchor);if(value==='week')d.setDate(d.getDate()+6);else if(value==='month')d.setMonth(d.getMonth()+1,0);end.input.value=ScheduleDates.iso(d)};quick.append(b)}
 const notes=document.createElement('label');notes.className='schedule-field';const label=document.createElement('span');label.textContent='备注';const textarea=document.createElement('textarea');textarea.setAttribute('aria-label','日程备注');textarea.value=entry?.notes||'';textarea.maxLength=5000;textarea.placeholder='这段时间想完成什么？';notes.append(label,textarea);
 const actions=document.createElement('div');actions.className='panel-actions';const cancel=document.createElement('button');cancel.type='button';cancel.className='secondary-button';cancel.textContent='返回日程';cancel.onclick=()=>run(showSchedules);const save=document.createElement('button');save.type='submit';save.className='primary-button';save.textContent='保存日程';actions.append(cancel,save);
 form.onsubmit=e=>{e.preventDefault();run(async()=>{const draft={title:title.input.value,startDate:start.input.value,endDate:end.input.value,atTime:time.input.value,notes:textarea.value};if(native){await api.SaveSchedule(entry?.id||'',draft);state=await api.GetState();render()}else{if(entry)Object.assign(entry,draft);else state.schedules.push({...draft,id:crypto.randomUUID(),done:false,createdAt:new Date().toISOString()});saveDemo()}await showSchedules();toast('日程已保存')},save)};
 form.append(title.wrap,dates,quick,notes,actions);content.append(form);title.input.focus();
}
async function exportItem(i){if(native){await api.ExportItem(i.id);return}const a=document.createElement('a');a.href=i.kind==='text'?URL.createObjectURL(new Blob([i.text],{type:'text/plain;charset=utf-8'})):media(i);a.download='拾光-'+i.id+(i.kind==='text'?'.txt':'.svg');a.click();if(i.kind==='text')setTimeout(()=>URL.revokeObjectURL(a.href),500)}
async function remove(i){if(native){await api.RemoveItem(i.id)}else{sessionStorage.setItem('cl-undo',JSON.stringify(state.items));state.items=state.items.filter(x=>x.id!==i.id);state.history.push(i);saveDemo();render()}toast('已移入历史记录',true)}
async function paste(){if(native){await api.Paste();return}const items=await navigator.clipboard.read();for(const item of items){const type=item.types.find(t=>t.startsWith('image/'));if(type){await importBlob(await item.getType(type));return}if(item.types.includes('text/plain')){addDemo({kind:'text',text:await(await item.getType('text/plain')).text(),source:'剪贴板'});return}}toast('剪贴板没有图片或文字')}
async function importBlob(file){if(file.size>32*1024*1024)throw Error('图片不能超过 32 MB');const data=await new Promise((resolve,reject)=>{const r=new FileReader();r.onload=()=>resolve(r.result);r.onerror=reject;r.readAsDataURL(file)});if(native){await api.AddImage(data.split(',')[1])}else{addDemo({kind:'image',demoImage:data,source:'导入'})}}
async function importImages(){if(native){await api.ImportImages()}else{$('#file-input').click()}}
async function collapse(){if(native){await api.Collapse()}else{setMode('basket');parent.postMessage({clothesline:'mode',mode:'basket'},location.origin)}}
async function expand(){if(native){await api.Expand()}else{setMode('shelf');parent.postMessage({clothesline:'mode',mode:'shelf'},location.origin)}}
// The whole basket moves the window once the pointer travels a few pixels;
// a press without movement still expands it.
let basketPress=null,basketMoved=false;
$('#basket').addEventListener('pointerdown',e=>{basketMoved=false;basketPress=e.button===0?{x:e.screenX,y:e.screenY}:null});
$('#basket').addEventListener('pointermove',e=>{if(!basketPress||!(e.buttons&1)||Math.hypot(e.screenX-basketPress.x,e.screenY-basketPress.y)<4)return;basketPress=null;basketMoved=true;if(native)window.WailsInvoke('drag')});
$('#basket').addEventListener('pointerup',()=>{basketPress=null});
// Windows resizes frameless windows from the page's edges; only the shelf may.
const edgeResize=native&&/Windows/.test(navigator.userAgent);
function syncResize(on=mode==='shelf'&&!$('#panel').open){if(!edgeResize||!window.wails)return;window.wails.flags.enableResize=on;if(!on&&window.wails.flags.resizeEdge){window.wails.flags.resizeEdge=undefined;document.documentElement.style.cursor=''}}
function setMode(v){const from=mode;mode=v;if(v==='shelf'){captureBusy=false;idle.activity()}else if($('#panel').open){$('#panel').close();panelKind=''}$('#shelf').hidden=v!=='shelf';$('#basket').hidden=v!=='basket';$('#scroll-selection').hidden=v!=='scroll-select';$('#scroll-controls').hidden=v!=='scroll-controls';if(v==='scroll-select'){$('#scroll-selection-image').src='/capture/background.png?v='+Date.now();$('#scroll-region-box').hidden=true;$('#scroll-region-confirm').disabled=true;$('#scroll-region-size').textContent='先选择一个区域';scrollRegion=null}else{$('#scroll-selection-image').removeAttribute('src')}if(from==='basket'&&v==='shelf')$$('.card-wrap.fresh').forEach(e=>e.classList.remove('fresh'));syncResize();updateIdle()}
async function startScrollCapture(){
 if(native){captureBusy=true;updateIdle();try{await api.StartScrollCapture()}catch(e){captureBusy=false;updateIdle();throw e}return}
 await showPanel('滚动截图','scroll-info');const p=document.createElement('p');p.className='settings-foot';p.textContent='桌面版中：先框选滚动内容区域，然后直接在目标页面向下滚动，拾光绳会自动采集并拼接，最后点击“完成并挂上”。遇到固定导航或重复图案时，可用“手动补采”并选择重叠比例。长图最多 2400 万像素、32760 px 高。浏览器演示不读取电脑屏幕。';$('#panel-content').append(p);
}
let scrollRegion=null,selectionStart=null;
const selection=$('#scroll-selection');
selection.addEventListener('pointerdown',e=>{if(e.target.closest('button,.selection-actions'))return;selectionStart={x:e.clientX,y:e.clientY};selection.setPointerCapture(e.pointerId);scrollRegion=null});
selection.addEventListener('pointermove',e=>{if(!selectionStart)return;const width=selection.clientWidth,height=selection.clientHeight,x=Math.max(0,Math.min(selectionStart.x,e.clientX,width)),y=Math.max(0,Math.min(selectionStart.y,e.clientY,height)),right=Math.min(width,Math.max(selectionStart.x,e.clientX)),bottom=Math.min(height,Math.max(selectionStart.y,e.clientY));scrollRegion={x:x/width,y:y/height,w:(right-x)/width,h:(bottom-y)/height};const box=$('#scroll-region-box');box.hidden=false;Object.assign(box.style,{left:x+'px',top:y+'px',width:(right-x)+'px',height:(bottom-y)+'px'});const image=$('#scroll-selection-image');$('#scroll-region-size').textContent=Math.round(scrollRegion.w*image.naturalWidth)+' × '+Math.round(scrollRegion.h*image.naturalHeight)+' px';$('#scroll-region-confirm').disabled=right-x<30||bottom-y<30});
selection.addEventListener('pointerup',e=>{selectionStart=null;if(selection.hasPointerCapture(e.pointerId))selection.releasePointerCapture(e.pointerId);if(scrollRegion&&!$('#scroll-region-confirm').disabled)$('#scroll-region-confirm').click()});
function showScrollStatus(s){
  $('#scroll-progress').textContent=`已拼接 ${s.count} 段 · ${s.width} × ${s.height} px`;
  $('#scroll-instruction').textContent=s.hint||(s.auto?'直接在目标页面向下滚动，会自动采集并拼接；滚到底后点「完成并挂上」。':'选区太大，控制台放不下：每次向下滚动约半屏后，点「采集一段」。');
  $('#scroll-instruction').classList.toggle('warning',!!s.hint);$('#scroll-next').textContent=s.auto?'手动补采':'采集一段';
}
async function scrollBusy(fn){const controls=$$('#scroll-controls button,#scroll-controls select');controls.forEach(b=>b.disabled=true);try{await fn()}finally{controls.forEach(b=>b.disabled=false)}}
$('#scroll-region-confirm').onclick=()=>run(async()=>{if(!scrollRegion)return;const r=scrollRegion;showScrollStatus(await api.ConfirmScrollRegion(r.x,r.y,r.w,r.h))},$('#scroll-region-confirm'));
$('#scroll-region-cancel').onclick=()=>run(()=>api.CancelScrollCapture());$('#scroll-cancel').onclick=()=>run(()=>api.CancelScrollCapture());
$('#scroll-next').onclick=()=>run(()=>scrollBusy(async()=>{showScrollStatus(await api.AddScrollSegment(Number($('#scroll-overlap').value)))}));
$('#scroll-finish').onclick=()=>run(()=>scrollBusy(async()=>{await api.FinishScrollCapture();toast('长图已经挂到绳上了')}));
const actions={paste,compose,settings,schedule:showSchedules,history:showHistory,collapse,import:importImages,scrollcapture:startScrollCapture,capture:async()=>{if(native){captureBusy=true;updateIdle();try{await api.Capture()}catch(e){captureBusy=false;updateIdle();throw e}}else{toast('桌面版会打开系统区域截图；这里可以体验便签、导入和收起')}}};
$$('[data-action]').forEach(b=>b.onclick=()=>run(actions[b.dataset.action],b));
$$('[data-filter]').forEach(b=>b.onclick=()=>{filter=b.dataset.filter;$$('[data-filter]').forEach(x=>x.classList.toggle('selected',x===b));render()});
$('#auto').onclick=()=>run(()=>updateSettings({autoCollect:!state.settings.autoCollect}),$('#auto'));$('#basket').onclick=()=>{if(basketMoved){basketMoved=false;return}run(expand)};$('#panel-close').onclick=()=>run(closePanel);
$('#panel').addEventListener('cancel',e=>{e.preventDefault();run(closePanel)});
document.addEventListener('keydown',e=>{if(e.key==='Escape'){e.preventDefault();if(mode.startsWith('scroll-')){run(()=>api.CancelScrollCapture());return}run($('#panel').open?closePanel:mode==='basket'?expand:collapse)}if((e.metaKey||e.ctrlKey)&&e.key.toLowerCase()==='n'&&!$('#panel').open&&mode==='shelf'){e.preventDefault();run(compose)}});
document.addEventListener('paste',e=>{if(e.target.matches('textarea,input'))return;e.preventDefault();if(native){run(paste);return}const images=[...e.clipboardData.files].filter(f=>f.type.startsWith('image/'));if(images.length){run(async()=>{for(const f of images)await importBlob(f)})}else{const text=e.clipboardData.getData('text/plain');if(text)addDemo({kind:'text',text,source:'剪贴板'})}});
$('#file-input').onchange=e=>run(async()=>{for(const f of e.target.files)await importBlob(f);e.target.value=''});
let dragDepth=0;document.addEventListener('dragenter',e=>{e.preventDefault();if(e.dataTransfer.types.includes('Files')){dragDepth++;$('#drop-hint').hidden=false}});document.addEventListener('dragover',e=>e.preventDefault());document.addEventListener('dragleave',()=>{if(--dragDepth<=0){dragDepth=0;$('#drop-hint').hidden=true}});document.addEventListener('drop',e=>{e.preventDefault();dragDepth=0;$('#drop-hint').hidden=true;if(!native)run(async()=>{for(const f of e.dataTransfer.files)await importBlob(f)})});
$('#cards').addEventListener('wheel',e=>{if(Math.abs(e.deltaY)>Math.abs(e.deltaX)){e.preventDefault();$('#cards').scrollLeft+=e.deltaY}},{passive:false});
for(const name of ['pointermove','pointerdown','keydown','wheel','input'])document.addEventListener(name,()=>idle.activity(),{passive:true});
function saveDemo(){try{localStorage.setItem('clothesline-demo',JSON.stringify(state))}catch{toast('浏览器演示存储已满，请在桌面版继续')}}
function addDemo(i){if(i.kind==='text'&&!i.text.trim())return;state.items.push({...i,id:crypto.randomUUID(),createdAt:new Date().toISOString()});saveDemo();render(true)}
function demoData(){
  const svg=s=>'data:image/svg+xml;charset=utf-8,'+encodeURIComponent(s);
  const picture=svg(`<svg xmlns="http://www.w3.org/2000/svg" width="520" height="320" viewBox="0 0 520 320"><defs><linearGradient id="s" x2="0" y2="1"><stop stop-color="#a9c7dc"/><stop offset="1" stop-color="#e0ecec"/></linearGradient><linearGradient id="m" x2="0" y2="1"><stop stop-color="#668990"/><stop offset="1" stop-color="#426b77"/></linearGradient></defs><path fill="url(#s)" d="M0 0h520v320H0z"/><circle cx="395" cy="73" r="29" fill="#fbf3d9" opacity=".8"/><path d="m0 214 95-119 101 104 103-136 126 163 95-103v197H0" fill="#8ea7ac"/><path d="m0 276 145-133 129 109 121-111 125 121v58H0" fill="url(#m)"/><path d="M0 272q135-40 260 4t260-8v52H0" fill="#b9d1d6"/><path d="m0 296 520-8" stroke="#d9e9e9" stroke-width="2"/></svg>`);
  const chart=svg(`<svg xmlns="http://www.w3.org/2000/svg" width="520" height="320"><path fill="#fcfdfd" d="M0 0h520v320H0z"/><text x="30" y="40" fill="#4d6574" font-family="sans-serif" font-size="17">Weekly overview</text><text x="30" y="70" fill="#9aaab3" font-family="sans-serif" font-size="12">A little progress, every day.</text><g stroke="#e8eff2"><path d="M30 120h460M30 170h460M30 220h460M30 270h460"/></g><path d="m30 240 65-25 65 8 65-65 65 25 65-60 65 12 70-57v192H30" fill="#e5eff4"/><path d="m30 240 65-25 65 8 65-65 65 25 65-60 65 12 70-57" stroke="#6d9ab0" stroke-width="4" fill="none"/><circle cx="420" cy="135" r="6" fill="#e9b694"/></svg>`);
  const palette=svg(`<svg xmlns="http://www.w3.org/2000/svg" width="520" height="320"><path fill="#eff4f6" d="M0 0h520v320H0z"/><text x="30" y="45" fill="#4e6777" font-family="sans-serif" font-size="16">A quieter kind of blue.</text><rect x="30" y="78" width="108" height="180" rx="54" fill="#bcd0dd"/><rect x="148" y="78" width="108" height="180" rx="54" fill="#7496a9"/><rect x="266" y="78" width="108" height="180" rx="54" fill="#e5c8ae"/><rect x="384" y="78" width="108" height="180" rx="54" fill="#9eb8ad"/><text x="30" y="293" fill="#8c9fa9" font-family="sans-serif" font-size="11" letter-spacing="3">FOUND ALONG THE WAY</text></svg>`);
  const now=Date.now();return{settings:{autoCollect:true,alwaysOnTop:true,heightPercent:20},items:[{kind:'image',demoImage:picture,source:'截图'},{kind:'text',text:'让桌面留一点空白。\n把灵感挂起来，\n然后，继续专注。',source:'便签'},{kind:'image',demoImage:chart,source:'剪贴板'},{kind:'image',demoImage:palette,source:'导入'},{kind:'text',text:'下午的待办\n\n☐ 整理刚刚的截图\n☐ 给自己泡杯茶',source:'便签'}].map((i,n)=>({...i,id:'demo-'+n,createdAt:new Date(now-(5-n)*11*60000).toISOString(),width:i.kind==='image'?520:0,height:i.kind==='image'?320:0}))};
}
async function init(){
 if(native){
   window.runtime.EventsOn('changed',s=>{const scroll=s.items.length>state.items.length,history=state.history;state=pendingSettings?{...s,settings:pendingSettings}:s;state.history=history;render(scroll);if(!nativeDragActive&&panelKind==='history'&&historyRefresh)run(historyRefresh);if(panelKind==='schedule'&&scheduleRefresh)scheduleRefresh()});window.runtime.EventsOn('notice',toast);window.runtime.EventsOn('scroll-status',showScrollStatus);window.runtime.EventsOn('mode',setMode);window.runtime.EventsOn('compose',()=>run(compose));window.runtime.EventsOn('drag-started',()=>{nativeDragActive=true;updateIdle()});window.runtime.EventsOn('drag-ended',endNativeDrag);
   window.runtime.OnFileDrop((x,y,paths)=>run(async()=>{$('#drop-hint').hidden=true;await api.ImportPaths(paths)}),false);
   state=await api.GetState();platform=await api.Platform();hotkeys=await api.HotkeyLabels();
   $('[data-action="capture"]').title=`截取屏幕区域（${hotkeys[0]}）`;$('[data-action="scrollcapture"]').title=`滚动截图（${hotkeys[1]}）`;
 }else{const saved=localStorage.getItem('clothesline-demo');try{state=saved?JSON.parse(saved):demoData()}catch{state=demoData()}}
 render(true);
 if(state.warning)toast(state.warning);
}
run(init);
