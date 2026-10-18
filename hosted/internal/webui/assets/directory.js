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
  function card(record) {
    const c = link("", "/connectors/" + encodeURIComponent(record.id), "directory-card-content");
    const logos = {"brave-search":"brave", github:"github", "google-gmail":"gmail", "google-calendar":"googlecalendar", "google-drive":"googledrive"};
    const icon = el("img", undefined, "directory-logo");
    icon.src = localAppIcon(connectorIcon(record)) ? connectorIcon(record) : "/assets/brand/" + (logos[record.service_id] || (record.kind === "openapi" ? "openapiinitiative" : "modelcontextprotocol")) + ".svg";
    if (localAppIcon(connectorIcon(record))) icon.className += " app-logo";
    icon.alt = "";
    icon.width = 48; icon.height = 48;
    icon.loading = "lazy";
    const heading = el("div", undefined, "directory-card-heading");
    heading.append(icon, el("h2", record.title || record.id));
    const useCase = (record.tags || [])[0] || "";
    const b = el("div", undefined, "directory-badges"); badges(record,b);
    c.append(heading);
    if (useCase) c.append(el("p", useCase, "directory-card-use-case"));
    const wrapper = el("article", undefined, "catalog-card directory-card bookmark-card");
    c.className = "directory-card-content";
    wrapper.dataset.entryId = record.id;
    wrapper.append(c, b, bookmarkButton(record));
    return wrapper;
  }
  function appCard(record) {
    const c = link("", "/apps/" + encodeURIComponent(record.slug), "directory-card-content");
    const heading = el("div", undefined, "directory-card-heading");
    const icon = el("img", undefined, "directory-logo app-logo");
    if (localAppIcon(record.icon_url)) icon.src = record.icon_url;
    icon.alt = ""; icon.width = 54; icon.height = 54; icon.loading = "lazy";
    heading.append(icon, el("h2", record.name || record.slug));
    c.append(heading);
    const category = (record.categories || []).filter(category => category.slug !== "app-families").map(category => category.name).find(Boolean) || "";
    c.append(el("p", category, "directory-card-use-case"));
    const arrow = el("span", "↗", "directory-card-arrow");
    arrow.setAttribute("aria-hidden", "true");
    c.append(arrow);
    const wrapper = el("article", undefined, "catalog-card directory-card app-card");
    wrapper.append(c); return wrapper;
  }
  async function home() {
    const target = document.getElementById("home-directory");
    try { const result = await api("/api/directory?limit=6"); target.replaceChildren(...result.items.slice(0,6).map(card)); if (!target.children.length) target.append(link("Browse integrations →", "/library", "text-link")); }
    catch (_) { target.replaceChildren(link("Browse the public library →", "/library", "text-link")); }
  }
  function directory() {
    const form = document.getElementById("directory-search"), query = document.getElementById("directory-query"), tag = document.getElementById("directory-tag"), results = document.getElementById("directory-results"), status = document.getElementById("directory-status"), notice = document.getElementById("directory-notice"), more = document.getElementById("directory-more");
    const params = new URLSearchParams(location.search); query.value = params.get("q") || ""; tag.value = params.get("category") || params.get("tag") || "";
    let savedOnly = params.get("saved") === "1";
    const savedFilter = document.getElementById("bookmark-filter");
    savedFilter.setAttribute("aria-pressed", String(savedOnly)); savedFilter.classList.toggle("active", savedOnly);
    let kind = ["mcp","openapi"].includes(params.get("kind")) ? params.get("kind") : "", cursor = "", generation = 0, total = 0;
    function filters() { tag.disabled = Boolean(kind || savedOnly); document.querySelectorAll("[data-directory-kind]").forEach(b=>{const active=b.dataset.directoryKind===kind; b.classList.toggle("active",active);b.setAttribute("aria-pressed",String(active));}); }
    async function load(append = false) {
      const ownSaved = savedOnly, own = ++generation; if (!append) more.hidden = true; more.disabled = true; results.setAttribute("aria-busy","true"); notice.replaceChildren();
      if (!append) results.replaceChildren();
      const p = new URLSearchParams({limit:"24"}); if(query.value.trim()) p.set("q",query.value.trim()); if(kind)p.set("kind",kind); if(tag.value)p.set(!ownSaved && !kind ? "category" : "tag",tag.value); if(append&&cursor)p.set(ownSaved || !kind ? "offset" : "cursor",cursor);
      const visible = new URLSearchParams(p); visible.delete("limit");visible.delete("cursor");visible.delete("offset");if(ownSaved)visible.set("saved","1"); history.replaceState(null,"","/library"+(visible.size?"?"+visible:""));
      try {
        if(new TextEncoder().encode(query.value.trim()).length>200){const e=new Error();e.status=400;throw e;}
        if (ownSaved && bookmarkState === "loading") await bookmarksReady;
        if (ownSaved && bookmarkState === "error") await loadBookmarks();
        if (own !== generation) return;
        if (ownSaved && bookmarkState === "guest") {
          status.textContent = "Sign in to view your personal bookmarks.";
          notice.append(link("Sign in with GitHub →", bookmarkSignIn, "button button-primary")); return;
        }
        if (ownSaved && bookmarkState === "error") throw new Error("Bookmarks unavailable");
        const appView = !ownSaved && !kind;
        let endpoint = appView ? "/api/directory/apps?" : "/api/directory?";
        if (ownSaved) { p.delete("limit"); p.set("view", "entries"); endpoint = "/app/api/bookmarks?"; }
        const data = await api(endpoint+p); if(own!==generation)return;
        if(appView) {
          const categories=new Map();
          for(const option of tag.children) if(option.value)categories.set(option.value,option.textContent);
          for(const category of data.categories || data.items.flatMap(item=>item.categories||[]))categories.set(category.slug,category.name);
          const selected=tag.value;
          tag.replaceChildren(el("option","All categories"));tag.children[0].value="";
          for(const [slug,name] of [...categories].sort((a,b)=>a[1].localeCompare(b[1]))) {const option=el("option",name);option.value=slug;tag.append(option);}
          tag.value=selected;
        }
        results.append(...data.items.map(appView ? appCard : card)); total=data.total; cursor=appView ? String(data.next_offset || "") : data.next_cursor||""; more.hidden=!cursor;
        status.textContent=data.total.toLocaleString()+(ownSaved ? " matching bookmarks · only visible to you" : (appView ? " matching apps" : " matching integrations"));
        if(!data.items.length&&!append)notice.append(el("p",(ownSaved ? "No matching bookmarks. Bookmark entries from the library to find them here." : "No matching integrations. Try another term or clear your filters."),"notice"));
      } catch(e) {if(own===generation){status.textContent="Search unavailable";if(ownSaved){const box=el("div",undefined,"notice error");box.append(el("p","Couldn’t load your bookmarks."),button("Try again",async()=>{await loadBookmarks();load(append);}));notice.replaceChildren(box);}else failure(notice,e,()=>load(append));}}
      finally {if(own===generation){results.setAttribute("aria-busy","false");more.disabled=false;}}
    }
    form.addEventListener("submit",e=>{e.preventDefault();load();}); tag.addEventListener("change",()=>load());
    document.querySelectorAll("[data-directory-kind]").forEach(b=>b.addEventListener("click",()=>{kind=b.dataset.directoryKind;tag.value="";filters();load();}));
    savedFilter.addEventListener("click", () => { savedOnly = !savedOnly; tag.value=""; filters(); savedFilter.setAttribute("aria-pressed", String(savedOnly)); savedFilter.classList.toggle("active", savedOnly); load(); });
    document.addEventListener("bookmarks-changed", event => {
      if (!savedOnly || !event.detail.removed) return;
      const cards = [...results.children], index = cards.findIndex(card => card.dataset.entryId === event.detail.id);
      if (index < 0) return;
      const removed = cards[index], focused = removed.contains(document.activeElement);
      const neighbour = cards[index + 1] || cards[index - 1];
      removed.remove(); total = Math.max(0, total - 1);
      if (cursor) cursor = String(Math.max(0, Number(cursor) - 1));
      status.textContent = total.toLocaleString() + " matching bookmarks · only visible to you · Bookmark removed.";
      if (!results.children.length && !cursor) notice.replaceChildren(el("p", "No matching bookmarks. Bookmark entries from the library to find them here.", "notice"));
      if (focused) {
        const next = neighbour?.querySelector(".bookmark-control button:not([hidden])");
        if (next) next.focus(); else { status.tabIndex = -1; status.focus(); }
      }
    });
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
