"use strict";
(() => {
  const page = document.body.dataset.page;
  if (!["home", "directory", "directory-detail", "private-directory"].includes(page)) return;
  const el = (tag, text, cls) => { const n = document.createElement(tag); if (text !== undefined) n.textContent = text; if (cls) n.className = cls; return n; };
  const button = (text, fn) => { const n = el("button", text, "button button-outline"); n.type = "button"; n.addEventListener("click", fn); return n; };
  const link = (text, href, cls) => { const n = el("a", text, cls); n.href = href; return n; };
  const external = value => { try { const u = new URL(value); return ["https:", "http:"].includes(u.protocol) && !u.username && !u.password && !u.search && !u.hash ? u.href : null; } catch (_) { return null; } };
  async function api(url, options = {}) {
    const r = await fetch(url, {credentials:"same-origin", cache:"no-store", ...options, headers:{Accept:"application/json", ...options.headers}});
    if (!r.ok) { const e = new Error("Request failed"); e.status = r.status; throw e; }
    return r.status === 204 ? null : r.json();
  }
  function failure(target, e, retry) {
    const box = el("div", undefined, "notice error");
    const text = e.status === 401 ? "Sign in with your invited GitHub account to open private integrations." : e.status === 403 ? "Your account does not have permission for this action. Ask your workspace operator." : e.status === 404 ? "This integration was not found. Return to the library and choose an available entry." : e.status === 400 ? "Check your input. Use a short search, plain credential names, and URLs without credentials, queries or fragments." : e.status === 409 ? "This workspace has reached its private integration limit. Remove an unused entry first." : "We couldn’t load this just yet. Try again shortly.";
    box.append(el("p", text));
    if (e.status === 401) box.append(link("Sign in with GitHub →", "/auth/github/start?return_to=%2Fworkspace%2Fservers", "button button-primary"));
    else if (![400,403,404,409].includes(e.status) && retry) box.append(button("Try again", retry));
    target.replaceChildren(box);
  }
  function badges(record, target) {
    target.append(el("span", record.kind === "openapi" ? "OpenAPI" : record.kind === "mcp" ? "MCP" : "Unavailable", "pill"));
  }
  // Login accepts fixed destinations only; never forward the current search URL.
  const bookmarkSignIn = "/auth/github/start?return_to=%2Flibrary";
  let bookmarkIDs = new Set(), bookmarkCSRF = "", bookmarkState = "loading", bookmarksReady;
  const bookmarkRefreshers = new WeakMap();
  function refreshBookmarkControls() {
    document.querySelectorAll(".bookmark-control").forEach(control => bookmarkRefreshers.get(control)?.());
  }
  async function loadBookmarks() {
    try {
      const session = await api("/app/api/session");
      if (!session.authenticated || !session.csrf_token) { bookmarkState = "guest"; return; }
      const data = await api("/app/api/bookmarks");
      bookmarkIDs = new Set(data.ids); bookmarkCSRF = session.csrf_token; bookmarkState = "ready";
    } catch (e) { bookmarkState = e.status === 401 ? "guest" : "error"; }
    finally { refreshBookmarkControls(); }
  }
  function bookmarkButton(record) {
    const control = el("div", undefined, "bookmark-control");
    let pending = false;
    const save = button("", async () => {
      if (pending || bookmarkState === "loading" || bookmarkState === "guest") return;
      pending = true; message.textContent = ""; refresh();
      if (bookmarkState === "error") {
        await loadBookmarks(); pending = false; refresh();
        if (bookmarkState === "error") message.textContent = "Couldn’t load bookmarks. Try again.";
        return;
      }
      const removing = bookmarkIDs.has(record.id);
      try {
        await api("/app/api/bookmarks?" + new URLSearchParams({id:record.id}), {method:removing ? "DELETE" : "PUT", headers:{"X-Gist-CSRF":bookmarkCSRF}});
        if (removing) bookmarkIDs.delete(record.id); else bookmarkIDs.add(record.id);
        refreshBookmarkControls();
        message.textContent = removing ? "Removed from your bookmarks." : "Saved to your bookmarks.";
        document.dispatchEvent(new CustomEvent("bookmarks-changed", {detail:{id:record.id, removed:removing}}));
      } catch(e) {
        message.textContent = e.status === 409 ? "You can save up to 200 bookmarks. Remove one first." : e.status === 401 ? "Your session expired. Sign in again to save." : e.status === 403 ? "Reload the page before trying again." : "Couldn’t update your bookmark. Try again.";
        if (e.status === 401) { bookmarkState = "guest"; refreshBookmarkControls(); signIn.focus(); }
      } finally { pending = false; refresh(); }
    });
    save.className += " bookmark-icon-button";
    const icon = el("img"); icon.alt = ""; icon.width = 18; icon.height = 18;
    save.append(icon);
    const signIn = link("", bookmarkSignIn, "button button-outline bookmark-icon-button");
    const signInLabel = "Sign in to bookmark " + (record.title || record.id);
    signIn.setAttribute("aria-label", signInLabel); signIn.title = signInLabel;
    const guestIcon = el("img"); guestIcon.src = "/assets/bookmark.svg"; guestIcon.alt = ""; guestIcon.width = 18; guestIcon.height = 18;
    signIn.append(guestIcon);
    const message = el("span", "", "quiet bookmark-message"); message.setAttribute("role", "status");
    function refresh() {
      save.hidden = bookmarkState === "guest";
      signIn.hidden = bookmarkState !== "guest";
      save.disabled = pending || bookmarkState === "loading";
      const label = (bookmarkState === "loading" ? "Loading bookmarks for" : bookmarkState === "error" ? "Retry bookmarks for" : "Bookmark") + " " + (record.title || record.id);
      save.setAttribute("aria-label", label); save.title = label;
      icon.src = bookmarkState === "ready" && bookmarkIDs.has(record.id) ? "/assets/bookmark-filled.svg" : "/assets/bookmark.svg";
      if (bookmarkState === "ready") save.setAttribute("aria-pressed", String(bookmarkIDs.has(record.id)));
      else save.removeAttribute("aria-pressed");
    }
    bookmarkRefreshers.set(control, refresh);
    control.append(save, signIn, message); refresh(); return control;
  }
  const localAppIcon = value => /^\/assets\/apps\/[a-zA-Z0-9_-]+\.(svg|png)$/.test(value || "");
  const connectorIcon = record => record.icon_url || globalThis.GIST_APP_ICONS?.[record.service_id];
  // One renderer owns the geometry and contents of public app/connector tiles.
  function integrationTile({href, title, iconURL, color, category, arrow = false}) {
    const content = link("", href, "directory-card-content");
    const heading = el("div", undefined, "directory-card-heading");
    const icon = el("img", undefined, "directory-logo app-logo");
    if (iconURL) icon.src = iconURL;
    icon.alt = ""; icon.width = 48; icon.height = 48; icon.loading = "lazy";
    const frame = el("span", undefined, "directory-logo-frame");
    if (/^#[0-9a-f]{6}$/i.test(color || "")) frame.style.backgroundColor = color;
    frame.append(icon); heading.append(frame, el("h2", title));
    content.append(heading, el("p", category, "directory-card-use-case"));
    if (arrow) { const hint = el("span", "↗", "directory-card-arrow"); hint.setAttribute("aria-hidden", "true"); content.append(hint); }
    const wrapper = el("article", undefined, "catalog-card directory-card app-card");
    wrapper.append(content); return wrapper;
  }
  function card(record) {
    const logos = {"brave-search":"brave", github:"github", "google-gmail":"gmail", "google-calendar":"googlecalendar", "google-drive":"googledrive"};
    const selectedIcon = connectorIcon(record);
    const tile = integrationTile({href:"/connectors/" + encodeURIComponent(record.id), title:record.title || record.id,
      iconURL:localAppIcon(selectedIcon) ? selectedIcon : "/assets/brand/" + (logos[record.service_id] || (record.kind === "openapi" ? "openapiinitiative" : "modelcontextprotocol")) + ".svg",
      color:globalThis.GIST_APP_ICON_COLORS?.[selectedIcon], category:record.kind === "unavailable" ? "Unavailable" : (record.tags || [])[0] || ""});
    tile.className += " bookmark-card"; tile.dataset.entryId = record.id;
    tile.append(bookmarkButton(record)); return tile;
  }
  function appCard(record) {
    const category = (record.categories || []).filter(category => category.slug !== "app-families").map(category => category.name).find(Boolean) || "";
    const tile = integrationTile({href:"/apps/" + encodeURIComponent(record.slug), title:record.name || record.slug,
      iconURL:localAppIcon(record.icon_url) ? record.icon_url : null, color:record.background_color, category, arrow:true});
    tile.dataset.appSlug = record.slug; return tile;
  }
  async function home() {
    const target = document.getElementById("home-directory");
    try { const result = await api("/api/directory/apps?limit=16"); target.replaceChildren(...result.items.slice(0,16).map(appCard)); if (!target.children.length) target.append(link("Browse integrations →", "/library", "text-link")); }
    catch (_) { target.replaceChildren(link("Browse the public library →", "/library", "text-link")); }
  }
  function directory() {
    const form = document.getElementById("directory-search"), query = document.getElementById("directory-query"), tag = document.getElementById("directory-tag"), results = document.getElementById("directory-results"), status = document.getElementById("directory-status"), notice = document.getElementById("directory-notice"), more = document.getElementById("directory-more");
    const params = new URLSearchParams(location.search); query.value = params.get("q") || ""; tag.value = params.get("category") || params.get("tag") || "";
    const pathKind = ({"/library/mcp":"mcp","/library/openapi":"openapi"})[location.pathname];
    let kind = pathKind || (["mcp","openapi"].includes(params.get("kind")) ? params.get("kind") : ""), cursor = "", generation = 0, total = 0;
    function filters() { tag.disabled = Boolean(kind); document.querySelectorAll("[data-directory-kind]").forEach(b=>{const active=b.dataset.directoryKind===kind; b.classList.toggle("active",active);b.setAttribute("aria-pressed",String(active));}); }
    async function load(append = false, preserve = false) {
      const own=++generation, personalized=bookmarkState==="ready"&&bookmarkIDs.size>0, appView=!kind;
      const pageLimit=personalized?100:appView?100:50;
      const wanted=preserve?Math.max(40,results.children.length):40, focused=preserve?document.activeElement:null, hadFocus=preserve&&results.contains(document.activeElement), scrollX=globalThis.scrollX||0, scrollY=globalThis.scrollY||0;
      if(!append&&!preserve)more.hidden=true;more.disabled=true;results.setAttribute("aria-busy","true");notice.replaceChildren();
      const p=new URLSearchParams({limit:String(Math.min(pageLimit,wanted))});
      if(query.value.trim())p.set("q",query.value.trim());if(kind)p.set("kind",kind);if(tag.value)p.set(appView&&!personalized?"category":"tag",tag.value);
      if(append&&cursor)p.set(personalized||appView?"offset":"cursor",cursor);
      const visible=new URLSearchParams(p);for(const key of ["limit","cursor","offset","kind","saved"])visible.delete(key);
      if(appView&&visible.has("tag")){visible.set("category",visible.get("tag"));visible.delete("tag");}
      history.replaceState(null,"","/library"+(kind?"/"+kind:"")+(visible.size?"?"+visible:""));
      let endpoint=personalized?"/app/api/bookmarks?":"/api/directory"+(appView?"/apps?":"?");if(personalized)p.set("view","catalog");
      try {
        if(new TextEncoder().encode(query.value.trim()).length>200){const e=new Error();e.status=400;throw e;}
        let data,items=[],next="";
        do {
          data=await api(endpoint+p);if(own!==generation)return;
          items.push(...data.items);next=personalized?data.next_cursor||"":appView?String(data.next_offset||""):data.next_cursor||"";
          if(!preserve||items.length>=wanted||!next)break;
          p.set(personalized||appView?"offset":"cursor",next);p.set("limit",String(Math.min(pageLimit,wanted-items.length)));
        } while(true);
        if(appView) {
          const categories=new Map();for(const option of tag.children)if(option.value)categories.set(option.value,option.textContent);
          for(const category of data.categories||items.flatMap(item=>item.categories||[]))categories.set(category.slug,category.name);
          const selected=tag.value;tag.replaceChildren(el("option","All categories"));tag.children[0].value="";
          for(const [slug,name]of [...categories].sort((a,b)=>a[1].localeCompare(b[1]))){const option=el("option",name);option.value=slug;tag.append(option);}tag.value=selected;
        }
        const existing=new Map([...results.children].map(node=>[node.dataset.appSlug||node.dataset.entryId,node]));
        const nodes=items.map(item=>preserve&&existing.get(item.slug||item.id)|| (appView?appCard(item):card(item)));
        if(append)results.append(...nodes);else results.replaceChildren(...nodes);
        if(hadFocus){const next=results.contains(focused)?focused:results.querySelector(".bookmark-control button:not([hidden])");if(next)next.focus({preventScroll:true});else{status.tabIndex=-1;status.focus({preventScroll:true});}}
        if(preserve)globalThis.scrollTo?.(scrollX,scrollY);
        total=data.total;cursor=next;more.hidden=!cursor;
        status.textContent=total.toLocaleString()+(appView?" matching apps":" matching integrations");
        if(!items.length&&!append)notice.append(el("p","No matching integrations. Try another term or clear your filters.","notice"));
      }catch(e){if(own===generation){
        if(personalized&&[401,403].includes(e.status)){bookmarkIDs=new Set();bookmarkState=e.status===401?"guest":"error";refreshBookmarkControls();load(false,true);return;}
        status.textContent="Search unavailable";failure(notice,e,()=>load(append,preserve));}}
      finally{if(own===generation){results.setAttribute("aria-busy","false");more.disabled=false;}}
    }
    form.addEventListener("submit",e=>{e.preventDefault();load();}); tag.addEventListener("change",()=>load());
    document.querySelectorAll("[data-directory-kind]").forEach(b=>b.addEventListener("click",()=>{kind=b.dataset.directoryKind;tag.value="";filters();load();}));
    document.addEventListener("bookmarks-changed",()=>load(false,true));
    bookmarksReady.then(()=>{if(bookmarkState==="ready"&&bookmarkIDs.size)load(false,true);});
    more.addEventListener("click",()=>load(true)); filters();load();
  }
  function field(target,label,value) { if(value) {const d=el("div"); d.append(el("dt",label),el("dd",String(value)));target.append(d);} }
  function sourceLink(target,label,value) {const url=external(value);if(url)target.append(link(label+" ↗",url,"text-link"));}
  async function detail() {
    const target=document.getElementById("directory-detail"), params=new URLSearchParams(location.search), cleanConnector=(location.pathname||"").startsWith("/connectors/"), id=cleanConnector ? decodeURIComponent(location.pathname.slice(12)) : params.get("id"), app=cleanConnector ? null : params.get("app");
    try {
      if (app) {
        const r = await api("/api/directory/app?" + new URLSearchParams({slug:app}));
        target.replaceChildren();
        if(localAppIcon(r.icon_url)) {const icon=el("img",undefined,"directory-logo app-logo");icon.src=r.icon_url;icon.alt="";icon.width=64;icon.height=64;target.append(icon);}
        target.append(el("h1",r.name),el("p",r.description,"artifact-description"));
        const categories=(r.categories||[]).map(category=>category.name).filter(Boolean).join(" · ");
        if(categories)target.append(el("p",categories,"quiet"));
        const setup=el("section");setup.append(el("h2","Connection options"));
        if((r.connector_ids||[]).length) {
          setup.append(el("p","Choose an integration to see its setup details and connect it in your client."));
          for(const connectorID of r.connector_ids) setup.append(link("View connection details →","/connectors/"+encodeURIComponent(connectorID),"text-link app-connection"));
        } else setup.append(el("p","MCP connection details are not available yet."));
        setup.append(link("Direct provider setup metadata →","/api/directory/authorization?"+new URLSearchParams({app,version:"2"}),"text-link"));
        target.append(setup); return;
      }
      if(!id){const e=new Error();e.status=400;throw e;}
      const r=await api("/api/directory/item?"+new URLSearchParams({id}));target.replaceChildren();
      const badgesEl=el("div",undefined,"directory-badges");badges(r,badgesEl);
      if(localAppIcon(connectorIcon(r))) {const icon=el("img",undefined,"directory-logo app-logo");icon.src=connectorIcon(r);icon.alt="";icon.width=64;icon.height=64;target.append(icon);}
      target.append(badgesEl,el("h1",r.title),bookmarkButton(r),el("p",r.description,"artifact-description"));
      const info=el("dl",undefined,"definition-list");field(info,"Descriptor version",r.version||"Not specified");field(info,"Authentication",r.auth?.type||"Check provider documentation");field(info,"Credential names",r.auth?.credential_names?.join(", "));target.append(info);
      const setup=el("section");setup.append(el("h2","Connect in your client"),el("p","Add this server to an MCP client that supports the listed transport, then authorize your provider account in that client. Review the requested access before connecting."));
      for(const remote of r.remotes||[]) {setup.append(el("h3",remote.transport==="streamable-http"?"Streamable HTTP endpoint":"SSE endpoint"),el("pre",remote.url));}
      for(const pkg of r.packages||[]) {setup.append(el("h3","Package · "+pkg.registry),el("pre",pkg.identifier+(pkg.version?" @ "+pkg.version:"")));}
      if(r.openapi) {setup.append(el("p","Use the OpenAPI specification with a compatible client or adapter."));sourceLink(setup,"OpenAPI specification",r.openapi.spec_url);sourceLink(setup,"Mint generation guide",r.openapi.mint_guide_url);}
      sourceLink(setup,"Authentication setup",r.auth?.setup_url);sourceLink(setup,"Project",r.homepage);sourceLink(setup,"Repository",r.repository);target.append(setup);
      try {
        const guide=await api("/api/directory/authorization?"+new URLSearchParams({id}));
        if(guide.format==="gist.directory.authorization/v1") {
          const section=el("section");section.append(el("h2","Provider authorization"));
          section.append(el("p",guide.status==="documented"?"Provider documentation reviewed. A connection has not been tested in your client.":"Authorization instructions for this exact connection still need research."));
          for(const requirement of guide.prerequisites||[])section.append(el("p",requirement));
          if((guide.steps||[]).length){const steps=el("ol");for(const step of guide.steps)steps.append(el("li",step));section.append(steps);}
          for(const note of guide.notes||[])section.append(el("p",note,"quiet"));
          for(const source of guide.sources||[])sourceLink(section,"Provider documentation · "+source.checked_at,source.url);
          section.append(el("p","Authorize and store credentials in your client. Gist does not receive provider credentials.","quiet"));
          target.append(section);
        }
      } catch (_) { /* Existing setup links remain usable when guidance is unavailable. */ }
      if((r.skills||[]).length){const skillSection=el("section");skillSection.append(el("h2","Related skills"));for(const s of r.skills){skillSection.append(el("h3",s.title||s.id));sourceLink(skillSection,"Skill guide",s.url);}target.append(skillSection);}
      const status=el("p",undefined,"quiet"); target.append(button("Export setup descriptor",async()=>{try {const data=await api("/api/directory/export?"+new URLSearchParams({id}));const u=URL.createObjectURL(new Blob([JSON.stringify(data,null,2)+"\n"],{type:"application/json"}));const a=link("",u);a.download="gist-integration.json";document.body.append(a);a.click();a.remove();setTimeout(()=>URL.revokeObjectURL(u),1000);status.textContent="Descriptor downloaded. Review it in your client before use.";}catch(e){failure(status,e);}}),status);
    }catch(e){failure(target,e,detail);}finally{target.setAttribute("aria-busy","false");}
  }
  async function privateDirectory() {
    const notice=document.getElementById("private-notice"), content=document.getElementById("private-content"), results=document.getElementById("private-results"), form=document.getElementById("private-create"), more=document.getElementById("private-more"), saveStatus=document.getElementById("private-save-status");
    let csrf="",cursor="",loadGeneration=0;
    const write=()=>({"Content-Type":"application/json","X-Gist-CSRF":csrf});
    function saved(entry) {const c=el("article",undefined,"catalog-card");c.append(el("p","Workspace only","eyebrow"),el("h3",entry.record.title),el("p",entry.record.description||"Custom setup metadata"),el("pre",JSON.stringify(entry.record,null,2)));const remove=button("Remove",async()=>{remove.disabled=true;try{await api("/app/api/directory?"+new URLSearchParams({id:entry.id}),{method:"DELETE",headers:write()});await load();}catch(e){failure(notice,e);}finally{remove.disabled=false;}});c.append(remove);return c;}
    async function load(append=false) {
      const own=++loadGeneration;more.hidden=true;
      try {
        const data=await api("/app/api/directory"+(append&&cursor?"?"+new URLSearchParams({cursor}):""));
        if(own!==loadGeneration)return;
        if(!append)results.replaceChildren();
        results.append(...data.items.map(saved));cursor=data.next_cursor||"";more.hidden=!cursor;
        if(!results.children.length)results.append(el("p","No private integrations saved yet.","quiet"));
      }catch(e){if(own===loadGeneration)failure(notice,e,()=>load(append));}
    }
    try {const session=await api("/app/api/session");if(!session.authenticated||!session.csrf_token){const e=new Error();e.status=401;throw e;}csrf=session.csrf_token;content.hidden=false;await load();}catch(e){failure(notice,e,privateDirectory);return;}
    more.addEventListener("click",()=>load(true));
    form.addEventListener("submit",async e=>{e.preventDefault();const submit=form.querySelector("button[type=submit]");submit.disabled=true;saveStatus.replaceChildren();notice.replaceChildren();
      try{const d=new FormData(form),title=d.get("title").trim(),type=d.get("connection"),target=d.get("target").trim(),version=d.get("version").trim()||"unversioned";
        const slug=title.toLowerCase().replace(/[^a-z0-9]+/g,"-").slice(0,80)||"integration";
        const record={id:"custom/"+slug,service_id:"custom/"+slug,title,description:d.get("description").trim(),kind:type==="openapi"?"openapi":"mcp",version,source:{id:"custom",entry_id:slug},auth:{type:d.get("auth"),credential_names:d.get("names").split(",").map(s=>s.trim()).filter(Boolean)}};
        if(type==="openapi")record.openapi={spec_url:target,mint_guide_url:"https://github.com/sirerun/mint"};else if(type==="local")record.packages=[{registry:"local",identifier:target,version,runtime:"native"}];else record.remotes=[{url:target,transport:type}];
        await api("/app/api/directory",{method:"POST",headers:write(),body:JSON.stringify({record})});form.reset();saveStatus.textContent="Saved privately. Your client still needs to admit and connect this integration.";await load();
      }catch(err){failure(saveStatus,err);}finally{submit.disabled=false;}
    });
  }
  async function start() {
    if (page !== "private-directory") bookmarksReady = loadBookmarks();
    if(page==="home")home();if(page==="directory")directory();if(page==="directory-detail")detail();if(page==="private-directory")privateDirectory();
  }
  start();
})();
