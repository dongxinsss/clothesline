(function(root){
 'use strict';
 const iso=d=>`${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,'0')}-${String(d.getDate()).padStart(2,'0')}`;
 const parse=s=>new Date(s+'T12:00:00');
 function range(anchor,view){const a=parse(anchor),b=parse(anchor);if(view==='week'){a.setDate(a.getDate()-(a.getDay()+6)%7);b.setTime(a.getTime());b.setDate(a.getDate()+6)}else if(view==='month'){a.setDate(1);b.setMonth(b.getMonth()+1,0)}return [iso(a),iso(b)]}
 function shift(anchor,view,delta){const d=parse(anchor);if(view==='month'){d.setDate(1);d.setMonth(d.getMonth()+delta)}else d.setDate(d.getDate()+delta*(view==='week'?7:1));return iso(d)}
 const api={iso,parse,range,shift};if(typeof module!=='undefined'&&module.exports)module.exports=api;else root.ScheduleDates=api;
})(typeof window!=='undefined'?window:globalThis);
