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
    target.append(el("span", record.kind === "unavailable" ? "Unavailable entry" : record.kind === "openapi" ? "OpenAPI · Mint required" : "MCP server", "pill"));
    if (record.featured === true) target.append(el("span", "Featured · tested in Zati", "pill"));
    else if (record.recommended === true) target.append(el("span", "Recommended candidate", "pill"));
    if (record.official === true) target.append(el("span", "Vendor official", "pill"));
    else if (record.kind === "mcp") target.append(el("span", "Operator not verified by Gist", "pill"));
    const checked = (record.remotes || []).map(r=>r.check).filter(c=>c && c.checked_at).sort((a,b)=>String(b.checked_at).localeCompare(String(a.checked_at)))[0];
    if (checked) target.append(el("span", "Endpoint responded · " + new Date(checked.checked_at).toLocaleDateString(), "pill"));
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
  function card(record) {
    const c = link("", "/library/server?" + new URLSearchParams({id:record.id}), "catalog-card directory-card");
    const logos = {"brave-search":"brave", github:"github", "google-gmail":"gmail", "google-calendar":"googlecalendar", "google-drive":"googledrive"};
    const icon = el("img", undefined, "directory-logo");
    icon.src = "/assets/brand/" + (logos[record.service_id] || (record.kind === "openapi" ? "openapiinitiative" : "modelcontextprotocol")) + ".svg";
    icon.alt = "";
    icon.width = 48; icon.height = 48;
    icon.loading = "lazy";
    const heading = el("div", undefined, "directory-card-heading");
    heading.append(icon, el("h2", record.title || record.id));
    const b = el("div", undefined, "directory-badges"); badges(record,b);
    c.append(heading, el("p", record.description || "Inspect setup and provenance."), el("p", record.featured ? "Dated workflow evidence available" : "Workflow qualification pending", "quiet"));
    const wrapper = el("article", undefined, "catalog-card directory-card bookmark-card");
    c.className = "bookmark-entry-link";
    wrapper.dataset.entryId = record.id;
    wrapper.append(c, b, bookmarkButton(record));
    return wrapper;
  }
  async function home() {
    const target = document.getElementById("home-directory");
    try { const result = await api("/api/directory?limit=6"); target.replaceChildren(...result.items.filter(r=>r.recommended).map(card)); if (!target.children.length) target.append(link("Browse integrations →", "/library", "text-link")); }
    catch (_) { target.replaceChildren(link("Browse the public library →", "/library", "text-link")); }
  }
  function directory() {
    const form = document.getElementById("directory-search"), query = document.getElementById("directory-query"), tag = document.getElementById("directory-tag"), results = document.getElementById("directory-results"), status = document.getElementById("directory-status"), notice = document.getElementById("directory-notice"), more = document.getElementById("directory-more");
    const params = new URLSearchParams(location.search); query.value = params.get("q") || ""; tag.value = params.get("tag") || "";
    let savedOnly = params.get("saved") === "1";
    const savedFilter = document.getElementById("bookmark-filter");
    savedFilter.setAttribute("aria-pressed", String(savedOnly)); savedFilter.classList.toggle("active", savedOnly);
    let kind = ["mcp","openapi"].includes(params.get("kind")) ? params.get("kind") : "", cursor = "", generation = 0, total = 0;
    function filters() { document.querySelectorAll("[data-directory-kind]").forEach(b=>{const active=b.dataset.directoryKind===kind; b.classList.toggle("active",active);b.setAttribute("aria-pressed",String(active));}); }
    async function load(append = false) {
      const ownSaved = savedOnly, own = ++generation; more.hidden = true; results.setAttribute("aria-busy","true"); notice.replaceChildren();
      if (!append) results.replaceChildren();
      const p = new URLSearchParams({limit:"24"}); if(query.value.trim()) p.set("q",query.value.trim()); if(kind)p.set("kind",kind); if(tag.value)p.set("tag",tag.value); if(append&&cursor)p.set(ownSaved ? "offset" : "cursor",cursor);
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
        let endpoint = "/api/directory?";
        if (ownSaved) { p.delete("limit"); p.set("view", "entries"); endpoint = "/app/api/bookmarks?"; }
        const data = await api(endpoint+p); if(own!==generation)return;
        results.append(...data.items.map(card)); total=data.total; cursor=data.next_cursor||""; more.hidden=!cursor;
        status.textContent=data.total.toLocaleString()+(ownSaved ? " matching bookmarks · only visible to you" : " matching integrations · public source snapshot");
        if(!data.items.length&&!append)notice.append(el("p",(ownSaved ? "No matching bookmarks. Bookmark entries from the library to find them here." : "No matching integrations. Try another term or clear your filters."),"notice"));
      } catch(e) {if(own===generation){status.textContent="Search unavailable";if(ownSaved){const box=el("div",undefined,"notice error");box.append(el("p","Couldn’t load your bookmarks."),button("Try again",async()=>{await loadBookmarks();load(append);}));notice.replaceChildren(box);}else failure(notice,e,()=>load(append));}}
      finally {if(own===generation)results.setAttribute("aria-busy","false");}
    }
    form.addEventListener("submit",e=>{e.preventDefault();load();}); tag.addEventListener("change",()=>load());
    document.querySelectorAll("[data-directory-kind]").forEach(b=>b.addEventListener("click",()=>{kind=b.dataset.directoryKind;filters();load();}));
    savedFilter.addEventListener("click", () => { savedOnly = !savedOnly; savedFilter.setAttribute("aria-pressed", String(savedOnly)); savedFilter.classList.toggle("active", savedOnly); load(); });
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
    const target=document.getElementById("directory-detail"), id=new URLSearchParams(location.search).get("id");
    try {
      if(!id){const e=new Error();e.status=400;throw e;}
      const r=await api("/api/directory/item?"+new URLSearchParams({id}));target.replaceChildren();
      const badgesEl=el("div",undefined,"directory-badges");badges(r,badgesEl);
      target.append(badgesEl,el("h1",r.title),bookmarkButton(r),el("p",r.description,"artifact-description"));
      const info=el("dl",undefined,"definition-list");field(info,"Descriptor version",r.version||"Not specified");field(info,"Authentication",r.auth?.type||"Check provider documentation");field(info,"Credential names",r.auth?.credential_names?.join(", "));field(info,"Source entry",r.source?.entry_id);target.append(info);
      const setup=el("section");setup.append(el("h2","Connect in your client"),el("p","Inspect and admit this descriptor in an MCP client that supports the listed transport. Authorize your provider account in that client. Zati currently supports Streamable HTTP; SSE entries require an SSE-capable client. Exporting a descriptor does not install, authorize or execute it."));
      for(const remote of r.remotes||[]) {setup.append(el("h3",remote.transport==="streamable-http"?"Streamable HTTP endpoint":"SSE endpoint"),el("pre",remote.url));}
      for(const remote of r.remotes||[]){const check=remote.check;if(!check)continue;const detail=check.result==="mcp-initialize"?"completed the MCP initialize handshake":check.result==="authentication-required"?"responded with an authentication challenge":check.result==="payment-required"?"responded that payment is required":"responded to the configured "+remote.transport+" reachability check";setup.append(el("p","Unauthenticated endpoint check: "+detail+" (HTTP "+check.http_status+") on "+new Date(check.checked_at).toLocaleString()+". This check did not list tools or make a provider call."));}
      for(const pkg of r.packages||[]) {setup.append(el("h3","Package · "+pkg.registry),el("pre",pkg.identifier+(pkg.version?" @ "+pkg.version:"")),el("p","Requires a client with local package support. Zati currently supports admitted Streamable HTTP connections; this listing does not imply local package compatibility.","quiet"));}
      if(r.openapi) {setup.append(el("p","An OpenAPI descriptor requires generation and hosting or a compatible adapter. Mint supports OpenAPI 3.x; specification validity and generated tool behavior still need verification. Mint’s stdio/SSE output is not directly compatible with Zati’s current Streamable HTTP admission."));sourceLink(setup,"OpenAPI specification",r.openapi.spec_url);sourceLink(setup,"Mint generation guide",r.openapi.mint_guide_url);}
      sourceLink(setup,"Authentication setup",r.auth?.setup_url);sourceLink(setup,"Project",r.homepage);sourceLink(setup,"Repository",r.repository);target.append(setup);
      const skillSection=el("section");skillSection.append(el("h2","Skills and workflow evidence"));
      if(!r.featured)skillSection.append(el("p","This candidate has not been qualified as a featured Zati workflow. Source listing, vendor status and recommendation do not establish successful execution."));
      for(const s of r.skills||[]) {skillSection.append(el("h3",s.title||s.id),el("p","Status: "+s.status));sourceLink(skillSection,"Skill source",s.url);}
      for(const v of r.verification||[]) {skillSection.append(el("p",[v.client,v.workflow,v.result,v.checked_at,v.expires_at?"Expires "+v.expires_at:""].filter(Boolean).join(" · ")));sourceLink(skillSection,"Dated evidence",v.evidence_url);}
      target.append(skillSection);
      const provenance=el("section");provenance.append(el("h2","Provenance"));sourceLink(provenance,"Original source",r.source?.url);sourceLink(provenance,"Vendor ownership evidence",r.official_evidence_url);sourceLink(provenance,r.server_listing_url?.includes("apps.make.com/")?"Make verified MCP server list":"Independent MCP server listing",r.server_listing_url);target.append(provenance);
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
