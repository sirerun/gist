const {test} = require('node:test');
const assert = require('node:assert/strict');
const {readFileSync} = require('node:fs');
const vm = require('node:vm');
const source = readFileSync(__dirname + '/assets/directory.js', 'utf8');
const records = [
  {id:'github', service_id:'github', title:'GitHub', kind:'mcp', recommended:true, remotes:[{url:'https://example.com/mcp',transport:'streamable-http',check:{checked_at:'2026-10-18T03:00:00Z',http_status:401,result:'authentication-required'}}]},
  {id:'community', service_id:'linkedin-ads-library', title:'LinkedIn Ads Library', kind:'mcp', recommended:true, official:false, server_listing_url:'https://registry.example.com/server'},
];
const flush = () => new Promise(resolve => setImmediate(resolve));
function fixture({page='home', search='', handle}={}) {
  const calls=[], listeners={}; let document;
  class Element {
    constructor(tag) { this.tag=tag; this.children=[];this.attrs={};this.dataset={};this.handlers={};this.hidden=false;this.className='';this.textContent='';this.classList={toggle(){}}; }
    append(...children) {for(const child of children){if(child.parent)child.remove();child.parent=this;this.children.push(child);}}
    replaceChildren(...children) {this.children.forEach(c=>c.parent=null);this.children=[];this.append(...children);}
    setAttribute(k,v){this.attrs[k]=String(v);}
    removeAttribute(k){delete this.attrs[k];}
    addEventListener(k,fn){this.handlers[k]=fn;}
    contains(el){return el===this||this.children.some(c=>c.contains?.(el));}
    remove(){if(this.parent)this.parent.children=this.parent.children.filter(c=>c!==this);this.parent=null;}
    focus(){document.activeElement=this;}
    querySelector(selector){return this.walk().find(el=>selector.includes('button')&&el.tag==='button'&&!el.hidden);}
    walk(){return this.children.flatMap(c=>[c,...(c.walk?.()||[])]);}
  }
  const root=new Element('body');root.dataset.page=page;const ids={};
  for(const id of ['home-directory','directory-search','directory-query','directory-tag','directory-results','directory-status','directory-notice','directory-more','bookmark-filter']) {ids[id]=new Element('div');ids[id].value='';root.append(ids[id]);}
  document={body:root,activeElement:root,createElement:tag=>new Element(tag),getElementById:id=>ids[id],querySelectorAll:selector=>selector==='.bookmark-control'?root.walk().filter(e=>e.className==='bookmark-control'):[],addEventListener:(name,fn)=>{(listeners[name]??=[]).push(fn);},dispatchEvent:e=>{for(const fn of listeners[e.type]||[])fn(e);}};
  const fetch=async(url,options={})=>{calls.push({url,options});let response=await handle?.(url,options);if(!response){if(url==='/api/directory?limit=6'||url.startsWith('/api/directory?'))response={data:{items:records,total:2}};else if(url==='/app/api/session')response={data:{authenticated:true,csrf_token:'token'}};else response={data:{ids:[]}};}return {ok:!response.status||response.status<400,status:response.status||200,json:async()=>response.data};};
  vm.runInNewContext(source,{document,fetch,URL,URLSearchParams,TextEncoder,WeakMap,Set,Event:class{constructor(type){this.type=type}},CustomEvent:class{constructor(type,options){this.type=type;Object.assign(this,options)}},location:{search,pathname:'/library'},history:{replaceState(){}},console});
  return {root,ids,calls,document,controls:()=>root.walk().filter(e=>e.className==='bookmark-control'),click:async el=>{if(el.disabled)throw Error('disabled');el.focus();await el.handlers.click({preventDefault(){}});await flush();}};
}
test('public cards render while authentication is unresolved; guest links use an allowed return URL',async()=>{
 let release;const f=fixture({handle:url=>url==='/app/api/session'?new Promise(resolve=>release=resolve):undefined});await flush();assert.equal(f.controls().length,2);assert.equal(f.controls()[0].children[0].disabled,true);
 assert.ok(f.root.walk().some(node=>node.textContent.startsWith('Endpoint responded')));
 assert.ok(f.root.walk().some(node=>node.textContent==='Operator not verified by Gist'));
 release({status:401});await flush();for(const c of f.controls()){const [save,signIn]=c.children;assert.equal(save.hidden,true);assert.equal(signIn.hidden,false);assert.equal(signIn.tag,'a');assert.equal(signIn.textContent,'');assert.ok(signIn.attrs['aria-label'].startsWith('Sign in to bookmark '));assert.equal(new URL(signIn.href,'https://gist.example').searchParams.get('return_to'),'/library');}
});
test('retry refreshes every control, and saving has a stable accessible toggle name',async()=>{
 let failing=true;const f=fixture({handle:url=>url==='/app/api/session'&&failing?{status:503}:undefined});await flush();assert.ok(f.controls().every(c=>c.children[0].attrs['aria-label'].startsWith('Retry bookmarks for')));
 failing=false;await f.click(f.controls()[0].children[0]);assert.ok(f.controls().every(c=>c.children[0].textContent==='' && c.children[0].attrs['aria-label'].startsWith('Bookmark ')));assert.equal(f.calls.filter(c=>c.options.method==='PUT').length,0);
 const save=f.controls()[0].children[0];await f.click(save);assert.equal(save.textContent,'');assert.equal(save.children[0].src,'/assets/bookmark-filled.svg');assert.equal(save.attrs['aria-label'],'Bookmark GitHub');assert.equal(save.attrs['aria-pressed'],'true');
});
test('an expired write session updates all controls to sign-in links',async()=>{
 const f=fixture({handle:(url,options)=>options.method==='PUT'?{status:401}:undefined});await flush();await f.click(f.controls()[0].children[0]);assert.ok(f.controls().every(c=>c.children[0].hidden&&!c.children[1].hidden));
});
test('saved removal preserves other cards, keyboard focus and loaded-page offset',async()=>{
 const f=fixture({page:'directory',search:'?saved=1',handle:(url,options)=>{
  if(url==='/app/api/bookmarks')return {data:{ids:records.map(r=>r.id)}};
  if(url.startsWith('/app/api/bookmarks?view='))return {data:{items:records,total:3,next_cursor:'2'}};
  if(options.method==='DELETE')return {status:204};
 }});await flush();const cards=f.ids['directory-results'].children;assert.equal(cards.length,2);const second=cards[1];const views=()=>f.calls.filter(c=>c.url.includes('view=entries'));
 const before=views().length;await f.click(f.controls()[0].children[0]);assert.equal(f.ids['directory-results'].children[0],second);assert.equal(f.document.activeElement,second.querySelector('button'));assert.equal(views().length,before);
 await f.click(f.ids['directory-more']);assert.ok(f.calls.some(c=>c.url.includes('offset=1')));
});
