const {test} = require('node:test');
const assert = require('node:assert/strict');
const {readFileSync} = require('node:fs');
const vm = require('node:vm');
const source = readFileSync(__dirname + '/assets/directory.js', 'utf8');
const records = [
  {id:'github', service_id:'github', title:'GitHub', kind:'mcp', remotes:[{url:'https://example.com/mcp',transport:'streamable-http',check:{checked_at:'2026-10-18T03:00:00Z',http_status:401,result:'authentication-required'}}]},
  {id:'community', service_id:'linkedin-ads-library', title:'LinkedIn Ads Library', kind:'mcp', official:false, server_listing_url:'https://registry.example.com/server'},
];
const flush = () => new Promise(resolve => setImmediate(resolve));
function fixture({page='directory', search='', pathname='/library/mcp', handle, icons={}, colors={}}={}) {
  const calls=[], listeners={},historyURLs=[]; let document;
  class Element {
    constructor(tag) { this.tag=tag; this.children=[];this.attrs={};this.style={};this.dataset={};this.handlers={};this.hidden=false;this.className='';this.textContent='';this.classList={toggle(){}}; }
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
  for(const id of ['home-directory','directory-search','directory-query','directory-tag','directory-results','directory-status','directory-notice','directory-more','bookmark-filter','directory-detail']) {ids[id]=new Element('div');ids[id].value='';root.append(ids[id]);}
  document={body:root,activeElement:root,createElement:tag=>new Element(tag),getElementById:id=>ids[id],querySelectorAll:selector=>selector==='.bookmark-control'?root.walk().filter(e=>e.className==='bookmark-control'):[],addEventListener:(name,fn)=>{(listeners[name]??=[]).push(fn);},dispatchEvent:e=>{for(const fn of listeners[e.type]||[])fn(e);}};
  const fetch=async(url,options={})=>{calls.push({url,options});let response=await handle?.(url,options);if(!response){if(url.startsWith('/api/directory/apps?'))response={data:{items:[{slug:'github',name:'GitHub',icon_url:'/assets/apps/github.png',background_color:'#112233',categories:[{slug:'development',name:'Development'}]}],total:1}};else if(url==='/api/directory?limit=16'||url.startsWith('/api/directory?'))response={data:{items:records,total:2}};else if(url==='/app/api/session')response={data:{authenticated:true,csrf_token:'token'}};else response={data:{ids:[]}};}return {ok:!response.status||response.status<400,status:response.status||200,json:async()=>response.data};};
  vm.runInNewContext(source,{GIST_APP_ICONS:icons,GIST_APP_ICON_COLORS:colors,document,fetch,URL,URLSearchParams,TextEncoder,WeakMap,Set,Event:class{constructor(type){this.type=type}},CustomEvent:class{constructor(type,options){this.type=type;Object.assign(this,options)}},location:{search,pathname},history:{replaceState(_state,_title,url){historyURLs.push(url)}},console});
  return {root,ids,calls,document,historyURLs,controls:()=>root.walk().filter(e=>e.className==='bookmark-control'),click:async el=>{if(el.disabled)throw Error('disabled');el.focus();await el.handlers.click({preventDefault(){}});await flush();}};
}
test('public cards render while authentication is unresolved; guest links use an allowed return URL',async()=>{
 let release;const f=fixture({handle:url=>url==='/app/api/session'?new Promise(resolve=>release=resolve):undefined});await flush();assert.equal(f.controls().length,2);assert.equal(f.controls()[0].children[0].disabled,true);
 const publicText=f.root.walk().map(node=>node.textContent).join(' ').toLowerCase();
 assert.ok(!publicText.includes('endpoint responded'));
 assert.ok(!publicText.includes('operator not verified'));
 assert.ok(!publicText.includes('provenance'));
 release({status:401});await flush();for(const c of f.controls()){const [save,signIn]=c.children;assert.equal(save.hidden,true);assert.equal(signIn.hidden,false);assert.equal(signIn.tag,'a');assert.equal(signIn.textContent,'');assert.ok(signIn.attrs['aria-label'].startsWith('Sign in to bookmark '));assert.equal(new URL(signIn.href,'https://gist.example').searchParams.get('return_to'),'/library');}
});
test('home cards render from the public directory without private recommendation flags',async()=>{
 const f=fixture({page:'home'});await flush();assert.equal(f.ids['home-directory'].children.length,1);assert.ok(f.root.walk().some(node=>node.textContent==='GitHub'));assert.ok(!f.root.walk().some(node=>node.textContent.includes('verified')));
});
test('retry refreshes every control, and saving has a stable accessible toggle name',async()=>{
 let failing=true;const f=fixture({handle:url=>url==='/app/api/session'&&failing?{status:503}:undefined});await flush();assert.ok(f.controls().every(c=>c.children[0].attrs['aria-label'].startsWith('Retry bookmarks for')));
 failing=false;await f.click(f.controls()[0].children[0]);assert.ok(f.controls().every(c=>c.children[0].textContent==='' && c.children[0].attrs['aria-label'].startsWith('Bookmark ')));assert.equal(f.calls.filter(c=>c.options.method==='PUT').length,0);
 const save=f.controls()[0].children[0];await f.click(save);assert.equal(save.textContent,'');assert.equal(save.children[0].src,'/assets/bookmark-filled.svg');assert.equal(save.attrs['aria-label'],'Bookmark GitHub');assert.equal(save.attrs['aria-pressed'],'true');
});
test('an expired write session updates all controls to sign-in links',async()=>{
 const f=fixture({handle:(url,options)=>options.method==='PUT'?{status:401}:undefined});await flush();await f.click(f.controls()[0].children[0]);assert.ok(f.controls().every(c=>c.children[0].hidden&&!c.children[1].hidden));
});
test('bookmark changes preserve existing cards and keyboard focus while reranking',async()=>{
 const f=fixture({page:'directory',pathname:'/library/mcp',handle:(url,options)=>{
  if(url==='/app/api/bookmarks')return {data:{ids:records.map(r=>r.id)}};
  if(url.startsWith('/app/api/bookmarks?'))return {data:{items:records,total:2,next_cursor:''}};
  if(options.method==='DELETE')return {status:204};
 }});await flush();await flush();const cards=f.ids['directory-results'].children;assert.equal(cards.length,2);const first=cards[0],second=cards[1];
 await f.click(f.controls()[0].children[0]);await flush();assert.equal(f.ids['directory-results'].children[0],first);assert.equal(f.ids['directory-results'].children[1],second);assert.equal(f.document.activeElement,first.querySelector('button'));
});
test('all apps browse uses local identity icons without app bookmarks or connector claims',async()=>{
 const f=fixture({page:'directory',pathname:'/library',handle:url=>url.startsWith('/api/directory/apps?')?{data:{items:[{slug:'gmail',name:'Gmail',description:'Read and organize email.',categories:[{slug:'mail',name:'Mail'}],icon_url:'/assets/apps/gmail.svg',connector_ids:[]}],total:3380}}:undefined});
 await flush();assert.ok(f.calls.some(c=>c.url.startsWith('/api/directory/apps?')));assert.equal(f.controls().length,0);
 const nodes=f.ids['directory-results'].walk();assert.ok(nodes.some(n=>n.src==='/assets/apps/gmail.svg'));assert.ok(nodes.some(n=>n.href==='/apps/gmail'));
 assert.equal(f.ids['directory-status'].textContent,'3,380 matching apps');assert.ok(!nodes.some(n=>n.textContent==='MCP'));
});
test('app icon URL accepts only local generated asset paths',async()=>{
 const f=fixture({page:'directory',pathname:'/library',handle:url=>url.startsWith('/api/directory/apps?')?{data:{items:[{slug:'unsafe',name:'Unsafe',icon_url:'https://external.example/icon.svg'}],total:1}}:undefined});
 await flush();assert.ok(!f.ids['directory-results'].walk().some(n=>n.src==='https://external.example/icon.svg'));
});

test('app detail without an admitted route is explicit and has no app bookmark action',async()=>{
 const f=fixture({page:'directory-detail',search:'?app=gmail',handle:url=>url==='/api/directory/app?slug=gmail'?{data:{slug:'gmail',name:'Gmail',description:'Email',categories:[],connector_ids:[]}}:undefined});
 await flush();assert.ok(f.ids['directory-detail'].walk().some(n=>n.textContent==='MCP connection details are not available yet.'));assert.equal(f.controls().length,0);
});
test('app detail links admitted connectors without asserting execution',async()=>{
 const f=fixture({page:'directory-detail',search:'?app=gmail',handle:url=>url==='/api/directory/app?slug=gmail'?{data:{slug:'gmail',name:'Gmail',categories:[],connector_ids:['google/gmail']}}:undefined});
 await flush();assert.ok(f.ids['directory-detail'].walk().some(n=>n.href==='/connectors/google%2Fgmail'));
});
test('app pagination uses an offset and category filters use inventory categories',async()=>{
 const f=fixture({page:'directory',pathname:'/library',handle:url=>url.startsWith('/api/directory/apps?')?{data:{items:[{slug:'gmail',name:'Gmail'}],total:30,next_offset:24,categories:[{slug:'communication',name:'Communication'}]}}:undefined});
 await flush();assert.ok(f.ids['directory-tag'].children.some(n=>n.value==='communication'));await f.click(f.ids['directory-more']);assert.ok(f.calls.some(c=>c.url.includes('/api/directory/apps?')&&c.url.includes('offset=24')));
 f.ids['directory-tag'].value='communication';await f.ids['directory-tag'].handlers.change();await flush();assert.ok(f.calls.some(c=>c.url.includes('category=communication')));
});

test('connector cards prefer validated local brand tiles and reject remote icon URLs',async()=>{
 const f=fixture({handle:url=>url.startsWith('/api/directory?')?{data:{items:[{...records[0],icon_url:'/assets/apps/github.svg'},{...records[1],icon_url:'https://external.example/icon.svg'}],total:2}}:undefined});
 await flush();const icons=f.ids['directory-results'].walk().filter(n=>n.tag==='img'&&n.className.includes('directory-logo'));
 assert.equal(icons[0].src,'/assets/apps/github.svg');assert.ok(icons[0].className.includes('app-logo'));assert.equal(icons[1].src,'/assets/brand/modelcontextprotocol.svg');
});

test('app details show local PNG brand tiles and unavailable entries are not labeled MCP',async()=>{
 const f=fixture({page:'directory-detail',search:'?app=gmail',handle:url=>url==='/api/directory/app?slug=gmail'?{data:{slug:'gmail',name:'Gmail',icon_url:'/assets/apps/gmail.png'}}:undefined});
 await flush();assert.ok(f.ids['directory-detail'].walk().some(n=>n.src==='/assets/apps/gmail.png'&&n.className.includes('app-logo')));
 const home=fixture({handle:url=>url.startsWith('/api/directory?')?{data:{items:[{id:'removed',title:'Removed entry',kind:'unavailable'}],total:1}}:undefined});
 await flush();assert.ok(home.ids['directory-results'].walk().some(n=>n.textContent==='Unavailable'));assert.ok(!home.ids['directory-results'].walk().some(n=>n.textContent==='MCP'));
});

test('connector cards use the local brand manifest without extending the API record',async()=>{
 const f=fixture({icons:{github:'/assets/apps/github.png'}});await flush();assert.ok(f.ids['directory-results'].walk().some(n=>n.src==='/assets/apps/github.png'&&n.className.includes('app-logo')));
});

test('minimal app tiles show one useful category and a decorative navigation arrow',async()=>{
 const f=fixture({page:'directory',pathname:'/library',handle:url=>url.startsWith('/api/directory/apps?')?{data:{items:[{slug:'gmail',name:'Gmail',description:'Search email',categories:[{slug:'app-families',name:'App Families'},{slug:'communication',name:'Communication'},{slug:'productivity',name:'Productivity'}]}],total:1,categories:[]}}:undefined});
 await flush();const nodes=f.ids['directory-results'].walk();
 const category=nodes.find(n=>n.className==='directory-card-use-case');assert.equal(category.textContent,'Communication');
 const arrow=nodes.find(n=>n.className==='directory-card-arrow');assert.equal(arrow.attrs['aria-hidden'],'true');
 assert.ok(nodes.some(n=>n.href==='/apps/gmail'));
});

test('load more keeps its space and existing cards while a page is loading',async()=>{
 let release;
 const f=fixture({page:'directory',pathname:'/library',handle:url=>url.startsWith('/api/directory/apps?')?(url.includes('offset=24')?new Promise(resolve=>release=resolve):{data:{items:[{slug:'gmail',name:'Gmail'}],total:48,next_offset:24}}):undefined});
 await flush();const more=f.ids['directory-more'],first=f.ids['directory-results'].children[0];
 const pending=more.handlers.click({preventDefault(){}});await flush();
 assert.equal(more.hidden,false);assert.equal(more.disabled,true);assert.equal(f.ids['directory-results'].children[0],first);
 release({data:{items:[{slug:'slack',name:'Slack'}],total:48,next_offset:48}});await pending;await flush();
 assert.equal(more.hidden,false);assert.equal(more.disabled,false);assert.equal(f.ids['directory-results'].children.length,2);assert.equal(f.ids['directory-results'].children[0],first);
});

test('library cards omit descriptions and expose only one hover category',async()=>{
 const f=fixture({handle:url=>url.startsWith('/api/directory?')?{data:{items:[{...records[0],description:'Long description stays on the detail page.',tags:['Communication','Productivity']}],total:1}}:undefined});await flush();
 const nodes=f.ids['directory-results'].walk();assert.ok(!nodes.some(n=>n.className==='directory-card-description'));assert.ok(nodes.some(n=>n.className==='directory-card-use-case'&&n.textContent==='Communication'));assert.ok(nodes.some(n=>n.href==='/connectors/github'));
});

test('clean connector path wins over conflicting legacy query parameters',async()=>{
 const f=fixture({page:'directory-detail',pathname:'/connectors/github',search:'?id=other&app=gmail',handle:url=>url==='/api/directory/item?id=github'?{data:records[0]}:undefined});await flush();assert.ok(f.calls.some(c=>c.url==='/api/directory/item?id=github'));assert.ok(!f.calls.some(c=>c.url.startsWith('/api/directory/app?')));assert.ok(f.ids['directory-detail'].walk().some(n=>n.textContent==='GitHub'));
});

test('app logo keeps a padded circular brand frame and rejects invalid colors',async()=>{
 const f=fixture({page:'directory',pathname:'/library',handle:url=>url.startsWith('/api/directory/apps?')?{data:{items:[{slug:'gmail',name:'Gmail',background_color:'#f14336'},{slug:'unsafe',name:'Unsafe',background_color:'url(https://bad.example)'}],total:2}}:undefined});await flush();const frames=f.ids['directory-results'].walk().filter(n=>n.className==='directory-logo-frame');assert.equal(frames[0].style.backgroundColor,'#f14336');assert.equal(frames[0].children[0].tag,'img');assert.equal(frames[1].style.backgroundColor,undefined);
});

test('typed library paths select the API kind and keep clean display URLs',async()=>{
 for(const kind of ['mcp','openapi']) {
  const f=fixture({page:'directory',pathname:'/library/'+kind,search:'?q=github'});await flush();assert.ok(f.calls.some(c=>c.url.includes('/api/directory?')&&c.url.includes('kind='+kind)));assert.equal(f.historyURLs.at(-1),'/library/'+kind+'?q=github');
 }
});
test('connector cards share the compact app frame and brand padding',async()=>{
 const f=fixture({icons:{github:'/assets/apps/github.png'},colors:{'/assets/apps/github.png':'#112233'}});await flush();const card=f.ids['directory-results'].children[0];assert.ok(card.className.includes('app-card'));const frame=card.walk().find(n=>n.className==='directory-logo-frame');assert.equal(frame.style.backgroundColor,'#112233');assert.ok(!card.walk().some(n=>n.className==='directory-badges'));
});

test('personalized catalog pagination appends saved-first results without duplicates',async()=>{
 const list=Array.from({length:60},(_,i)=>({...records[0],id:'tool-'+i,title:'Tool '+i}));const ranked=[list[59],...list.slice(0,59)];
 const f=fixture({page:'directory',pathname:'/library/mcp',handle:url=>{
  if(url==='/app/api/bookmarks')return {data:{ids:['tool-59']}};
  if(url.startsWith('/app/api/bookmarks?')){const q=new URL(url,'https://gist.example').searchParams,o=Number(q.get('offset')||0),end=Math.min(60,o+Number(q.get('limit')||40));return {data:{items:ranked.slice(o,end),total:60,next_cursor:end<60?String(end):''}};}
 }});await flush();await flush();assert.equal(f.ids['directory-results'].children[0].dataset.entryId,'tool-59');assert.equal(f.ids['directory-results'].children.length,40);await f.click(f.ids['directory-more']);const ids=f.ids['directory-results'].children.map(n=>n.dataset.entryId);assert.equal(ids.length,60);assert.equal(new Set(ids).size,60);assert.ok(f.calls.some(c=>c.url.includes('offset=40')));
});

test('home and library app tiles expose identical markup without connector bookmarks',async()=>{
 const home=fixture({page:'home'}),library=fixture({page:'directory',pathname:'/library'});
 await flush();await flush();
 const shape=n=>({tag:n.tag,cls:n.className,text:n.textContent,href:n.href,src:n.src,style:n.style,attrs:n.attrs,children:n.children.map(shape)});
 assert.deepEqual(shape(home.ids['home-directory'].children[0]),shape(library.ids['directory-results'].children[0]));assert.equal(home.controls().length,0);assert.ok(home.calls.some(c=>c.url==='/api/directory/apps?limit=16'));
});

 test('home requests and caps 16 apps; library requests 40 per page',async()=>{
 const apps=Array.from({length:60},(_,i)=>({slug:'app-'+i,name:'App '+i}));
 const home=fixture({page:'home',handle:url=>url.startsWith('/api/directory/apps?')?{data:{items:apps,total:60}}:undefined});await flush();assert.equal(home.ids['home-directory'].children.length,16);assert.ok(home.calls.some(c=>c.url==='/api/directory/apps?limit=16'));
 const library=fixture({page:'directory',pathname:'/library',handle:url=>{if(!url.startsWith('/api/directory/apps?'))return;const q=new URL(url,'https://gist.example').searchParams,start=Number(q.get('offset')||0),limit=Number(q.get('limit'));return {data:{items:apps.slice(start,start+limit),total:60,next_offset:start+limit<60?start+limit:0}};}});await flush();assert.equal(library.ids['directory-results'].children.length,40);await library.click(library.ids['directory-more']);assert.equal(library.ids['directory-results'].children.length,60);assert.equal(new Set(library.ids['directory-results'].children.map(n=>n.dataset.appSlug)).size,60);assert.ok(library.calls.some(c=>c.url.includes('limit=40')&&c.url.includes('offset=40')));
 });
