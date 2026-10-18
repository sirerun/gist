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
    target.append(el("span", record.kind === "openapi" ? "OpenAPI · Mint required" : "MCP server", "pill"));
    if (record.featured === true) target.append(el("span", "Featured · tested in Zati", "pill"));
    else if (record.recommended === true) target.append(el("span", "Recommended candidate", "pill"));
    if (record.official === true) target.append(el("span", "Vendor official", "pill"));
  }
  // Repository ownership is attribution, not a claim of workflow verification.
  function publisher(record) {
    const repository = external(record.repository);
    if (repository) {
      const url = new URL(repository);
      if (["github.com", "gitlab.com", "bitbucket.org"].includes(url.hostname)) {
        const owner = url.pathname.split("/").filter(Boolean)[0];
        if (owner) return {name: owner, prefix: "by "};
      }
    }
    const homepage = external(record.homepage);
    if (homepage) return {name: new URL(homepage).hostname.replace(/^www\./, ""), prefix: "from "};
    return {name: record.source?.id || "Public directory", prefix: "via "};
  }
  function card(record) {
    const c = link("", "/library/server?" + new URLSearchParams({id:record.id}), "catalog-card directory-card");
    const title = record.title || record.id;
    const identity = el("div", undefined, "directory-identity");
    const initials = title.replace(/[^\p{L}\p{N} ]/gu, " ").trim().split(/\s+/).slice(0, 2).map(word => Array.from(word)[0] || "").join("").toLocaleUpperCase();
    const icon = el("span", initials || "M", "directory-icon");
    icon.setAttribute("aria-hidden", "true");
    const heading = el("div", undefined, "directory-heading");
    const titleRow = el("div", undefined, "directory-title-row");
    titleRow.append(el("h2", title));
    if (record.official === true) {
      const shield = el("span", undefined, "directory-official");
      shield.title = "Vendor official · ownership evidence available";
      shield.append(el("span", "Vendor official", "sr-only"));
      titleRow.append(shield);
    }
    const attribution = publisher(record);
    const byline = el("p", undefined, "directory-byline");
    byline.append(el("span", attribution.prefix), el("span", attribution.name, "directory-publisher"));
    heading.append(titleRow, byline);
    identity.append(icon, heading);
    const b = el("div", undefined, "directory-badges");
    b.append(el("span", record.kind === "openapi" ? "OpenAPI · Mint required" : "MCP server", "pill"));
    if (record.featured === true) b.append(el("span", "Featured · tested in Zati", "pill"));
    else if (record.recommended === true) b.append(el("span", "Recommended candidate", "pill"));
    c.append(identity, el("p", record.description || "Inspect setup and provenance.", "directory-description"), b);
    return c;
  }
  async function home() {
    const target = document.getElementById("home-directory");
    try { const result = await api("/api/directory?limit=6"); target.replaceChildren(...result.items.filter(r=>r.recommended).map(card)); if (!target.children.length) target.append(link("Browse integrations →", "/library", "text-link")); }
    catch (_) { target.replaceChildren(link("Browse the public library →", "/library", "text-link")); }
  }
  function directory() {
    const form = document.getElementById("directory-search"), query = document.getElementById("directory-query"), tag = document.getElementById("directory-tag"), results = document.getElementById("directory-results"), status = document.getElementById("directory-status"), notice = document.getElementById("directory-notice"), more = document.getElementById("directory-more");
    const params = new URLSearchParams(location.search); query.value = params.get("q") || ""; tag.value = params.get("tag") || "";
    let kind = ["mcp","openapi"].includes(params.get("kind")) ? params.get("kind") : "", cursor = "", generation = 0;
    function filters() { document.querySelectorAll("[data-directory-kind]").forEach(b=>{const active=b.dataset.directoryKind===kind; b.classList.toggle("active",active);b.setAttribute("aria-pressed",String(active));}); }
    async function load(append = false) {
      const own = ++generation; more.hidden = true; results.setAttribute("aria-busy","true"); notice.replaceChildren();
      if (!append) results.replaceChildren();
      const p = new URLSearchParams({limit:"24"}); if(query.value.trim()) p.set("q",query.value.trim()); if(kind)p.set("kind",kind); if(tag.value)p.set("tag",tag.value); if(append&&cursor)p.set("cursor",cursor);
      const visible = new URLSearchParams(p); visible.delete("limit");visible.delete("cursor"); history.replaceState(null,"","/library"+(visible.size?"?"+visible:""));
      try {
        if(new TextEncoder().encode(query.value.trim()).length>200){const e=new Error();e.status=400;throw e;}
        const data = await api("/api/directory?"+p); if(own!==generation)return;
        results.append(...data.items.map(card)); cursor=data.next_cursor||""; more.hidden=!cursor;
        status.textContent=data.total.toLocaleString()+" matching integrations · public source snapshot";
        if(!data.items.length&&!append)notice.append(el("p","No matching integrations. Try another term or clear your filters.","notice"));
      } catch(e) {if(own===generation){status.textContent="Search unavailable";failure(notice,e,()=>load(append));}}
      finally {if(own===generation)results.setAttribute("aria-busy","false");}
    }
    form.addEventListener("submit",e=>{e.preventDefault();load();}); tag.addEventListener("change",()=>load());
    document.querySelectorAll("[data-directory-kind]").forEach(b=>b.addEventListener("click",()=>{kind=b.dataset.directoryKind;filters();load();}));
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
      target.append(badgesEl,el("h1",r.title),el("p",r.description,"artifact-description"));
      const info=el("dl",undefined,"definition-list");field(info,"Descriptor version",r.version||"Not specified");field(info,"Authentication",r.auth?.type||"Check provider documentation");field(info,"Credential names",r.auth?.credential_names?.join(", "));field(info,"Source entry",r.source?.entry_id);target.append(info);
      const setup=el("section");setup.append(el("h2","Connect in your client"),el("p","Inspect and admit this descriptor in Zati or your MCP client. Connect your provider account there. Exporting a descriptor does not install, authorize or execute it."));
      for(const remote of r.remotes||[]) {setup.append(el("h3",remote.transport==="streamable-http"?"Streamable HTTP endpoint":"SSE endpoint"),el("pre",remote.url));}
      for(const pkg of r.packages||[]) {setup.append(el("h3","Package · "+pkg.registry),el("pre",pkg.identifier+(pkg.version?" @ "+pkg.version:"")),el("p","Requires a client with local package support. Zati currently supports admitted Streamable HTTP connections; this listing does not imply local package compatibility.","quiet"));}
      if(r.openapi) {setup.append(el("p","An OpenAPI descriptor requires generation and hosting or a compatible adapter. Mint supports OpenAPI 3.x; specification validity and generated tool behavior still need verification. Mint’s stdio/SSE output is not directly compatible with Zati’s current Streamable HTTP admission."));sourceLink(setup,"OpenAPI specification",r.openapi.spec_url);sourceLink(setup,"Mint generation guide",r.openapi.mint_guide_url);}
      sourceLink(setup,"Authentication setup",r.auth?.setup_url);sourceLink(setup,"Project",r.homepage);sourceLink(setup,"Repository",r.repository);target.append(setup);
      const skillSection=el("section");skillSection.append(el("h2","Skills and workflow evidence"));
      if(!r.featured)skillSection.append(el("p","This candidate has not been qualified as a featured Zati workflow. Source listing, vendor status and recommendation do not establish successful execution."));
      for(const s of r.skills||[]) {skillSection.append(el("h3",s.title||s.id),el("p","Status: "+s.status));sourceLink(skillSection,"Skill source",s.url);}
      for(const v of r.verification||[]) {skillSection.append(el("p",[v.client,v.workflow,v.result,v.checked_at,v.expires_at?"Expires "+v.expires_at:""].filter(Boolean).join(" · ")));sourceLink(skillSection,"Dated evidence",v.evidence_url);}
      target.append(skillSection);
      const provenance=el("section");provenance.append(el("h2","Provenance"));sourceLink(provenance,"Original source",r.source?.url);sourceLink(provenance,"Vendor ownership evidence",r.official_evidence_url);target.append(provenance);
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
  if(page==="home")home();if(page==="directory")directory();if(page==="directory-detail")detail();if(page==="private-directory")privateDirectory();
})();
