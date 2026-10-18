"use strict";
(() => {
  const page = document.body.dataset.page;
  const kinds = new Set(["skill", "tool", "capability", "provider", "binding", "taxonomy"]);
  const labels = { skill: "Skill", tool: "Tool", capability: "Capability", provider: "Provider", binding: "Binding", taxonomy: "Taxonomy", connection: "Connection", resolution: "Resolution" };
  const node = (tag, text, className) => {
    const el = document.createElement(tag);
    if (text !== undefined) el.textContent = text;
    if (className) el.className = className;
    return el;
  };
  const text = (value, fallback = "") => typeof value === "string" ? value : fallback;
  const utf8Length = value => new TextEncoder().encode(value).length;
  const button = (label, action) => {
    const el = node("button", label, "button button-outline");
    el.type = "button";
    el.addEventListener("click", action);
    return el;
  };
  const link = (label, href, className) => {
    const el = node("a", label, className);
    el.href = href;
    return el;
  };
  const signinURL = "/auth/github/start?return_to=%2Fworkspace";
  async function api(url, signal) {
    const response = await fetch(url, { credentials: "same-origin", cache: "no-store", headers: { Accept: "application/json" }, signal });
    if (!response.ok) {
      const error = new Error(response.status === 401 ? "Sign in required" : "Service unavailable");
      error.status = response.status;
      if (response.status === 400) {
        try {
          const detail = await response.json();
          error.code = detail.code === "invalid_cursor" ? "invalid_cursor" : "invalid_query";
        } catch (_) { error.code = "invalid_query"; }
      }
      throw error;
    }
    return response.json();
  }
  function notice(container, title, description, error = false) {
    container.replaceChildren();
    const box = node("div", undefined, error ? "notice error" : "notice");
    box.append(node("h2", title), node("p", description));
    container.append(box);
    return box;
  }
  function unauthorized(container) {
    const box = notice(container, "Your library is a sign-in away", "Use your invited GitHub account to browse your workspace’s published skills and tools.");
    box.append(link("Sign in with GitHub →", signinURL, "button button-primary"), node("p", "No repository permissions requested. Account connections remain in your client.", "quiet"));
  }
  function failure(container, error, retry) {
    if (error.status === 401) { unauthorized(container); return; }
    if (error.status === 400) {
      const expired = error.code === "invalid_cursor";
      const box = notice(container, expired ? "This page link has expired" : "Check this search or artifact link", expired ? "Start a new search to retrieve a fresh page of catalog results." : "Try a shorter search phrase. For artifact details, choose an item from the library.", true);
      box.append(link("Back to workspace →", "/workspace", "text-link"));
      return;
    }
    const title = error.status === 403 ? "This account does not have access" : error.status === 404 ? "This artifact was not found" : "We couldn’t load this just yet";
    const description = error.status === 403 ? "Ask your workspace operator to confirm your invitation and permissions." : error.status === 404 ? "Check the artifact link, or return to the library to find an available version." : "The service may be starting up or temporarily unavailable. Try again in a moment. If this is the first sign-in, the configured founder must complete enrollment.";
    const box = notice(container, title, description, true);
    if (error.status !== 403 && error.status !== 404) box.append(button("Try again", retry));
    box.append(link("Back to workspace →", "/workspace", "text-link"));
  }
  async function loadSession() {
    try {
      const session = await api("/app/api/session");
      if (session.authenticated !== true) return null;
      const account = document.getElementById("account-nav");
      const menu = node("div", undefined, "account-menu");
      menu.append(link(text(session.workspace_name, text(session.workspace_id, "Your workspace")), "/workspace"));
      const logout = node("button", "Sign out");
      logout.type = "button";
      logout.addEventListener("click", async () => {
        logout.disabled = true;
        try {
          const response = await fetch("/app/api/logout", { method: "POST", credentials: "same-origin", cache: "no-store", headers: { Origin: window.location.origin } });
          if (!response.ok) throw new Error("logout failed");
          window.location.assign("/signin");
        } catch (_) {
          logout.disabled = false;
          let message = account.querySelector(".logout-error");
          if (!message) { message = node("p", "Sign-out failed. Please try again.", "logout-error"); message.setAttribute("role", "alert"); account.append(message); }
        }
      });
      menu.append(link("Private integrations", "/workspace/servers"), logout);
      account.replaceChildren(menu);
      return session;
    } catch (_) { return null; }
  }
  function artifactURL(item) {
    const params = new URLSearchParams({ kind: text(item.kind), id: text(item.id), version: text(item.version) });
    return "/library/item?" + params;
  }
  function catalogCard(item) {
    const card = link("", artifactURL(item), "catalog-card");
    card.append(node("p", labels[item.kind] || text(item.kind, "Artifact"), "eyebrow"));
    card.append(node("h2", text(item.title, text(item.id, "Untitled artifact"))));
    card.append(node("p", text(item.description, "Open this artifact to inspect its version, provenance, and published metadata.")));
    const meta = node("div", undefined, "record-meta");
    meta.append(node("span", text(item.version, "Version unavailable")), node("span", text(item.state, "State unavailable"), "pill"), node("span", "View details →"));
    card.append(meta);
    return card;
  }
  async function library() {
    const form = document.getElementById("catalog-search");
    const input = document.getElementById("catalog-query");
    const results = document.getElementById("catalog-results");
    const messages = document.getElementById("catalog-notice");
    const status = document.getElementById("catalog-status");
    const more = document.getElementById("load-more");
    const extra = document.getElementById("other-kind");
    const params = new URLSearchParams(window.location.search);
    let q = params.get("q") || "";
    let kind = kinds.has(params.get("kind")) ? params.get("kind") : "";
    let next = "";
    let count = 0;
    let pages = 0;
    let controller;
    let generation = 0;
    const seen = new Set();
    input.value = q;
    const updateFilters = () => {
      document.querySelectorAll(".filter").forEach(el => {
        const active = el.dataset.kind === kind;
        el.classList.toggle("active", active);
        el.setAttribute("aria-pressed", String(active));
      });
      extra.value = ["binding", "taxonomy"].includes(kind) ? kind : "";
    };
    async function search(append = false) {
      if (controller) controller.abort();
      controller = new AbortController();
      const request = ++generation;
      updateFilters();
      if (utf8Length(q) > 200 || q.includes("\0")) {
        input.setAttribute("aria-invalid", "true");
        status.textContent = "Search needs a shorter phrase";
        more.hidden = true;
        results.setAttribute("aria-busy", "false");
        notice(messages, "Try a shorter search", "This search is too long. Try a shorter phrase.", true);
        return;
      }
      input.removeAttribute("aria-invalid");
      if (!append) { results.replaceChildren(); messages.replaceChildren(); count = 0; pages = 0; next = ""; seen.clear(); }
      results.setAttribute("aria-busy", "true");
      more.hidden = true;
      status.textContent = append ? "Loading more artifacts…" : "Searching your workspace…";
      const query = new URLSearchParams();
      if (q) query.set("q", q);
      if (kind) query.set("kind", kind);
      if (append && next) query.set("cursor", next);
      const locationQuery = new URLSearchParams();
      if (q) locationQuery.set("q", q);
      if (kind) locationQuery.set("kind", kind);
      window.history.replaceState(null, "", "/workspace" + (locationQuery.size ? "?" + locationQuery : ""));
      updateFilters();
      try {
        const data = await api("/app/api/catalog?" + query, controller.signal);
        if (request !== generation) return;
        if (!Array.isArray(data.items)) throw new Error("Invalid catalog response");
        messages.replaceChildren();
        for (const item of data.items) {
          if (!item || !kinds.has(item.kind) || typeof item.id !== "string" || typeof item.version !== "string") continue;
          const key = JSON.stringify([item.kind, item.id, item.version]);
          if (seen.has(key)) continue;
          seen.add(key);
          results.append(catalogCard(item));
          count++;
        }
        pages++;
        const cursor = text(data.next_cursor);
        next = cursor && cursor !== next ? cursor : "";
        status.textContent = count ? `${count} artifact${count === 1 ? "" : "s"} shown${next ? " · More available" : ""}` : "No matching artifacts";
        more.hidden = !next || pages >= 20;
        if (next && pages >= 20) status.textContent += " · Refine your search to see more";
        if (!count) {
          const box = notice(messages, q || kind ? "Nothing matches just yet" : "A little room for possibility", q || kind ? "Try a broader phrase or clear your filters to explore everything in your workspace." : "Your workspace has no published catalog records yet. Ask your operator about adding content, or read how Gist works.");
          if (q || kind) box.append(button("Clear search and filters", () => { q = ""; kind = ""; input.value = ""; search(); }));
          else box.append(link("Read the getting started guide →", "/docs", "text-link"));
        }
      } catch (error) {
        if (error.name === "AbortError" || request !== generation) return;
        status.textContent = "";
        failure(messages, error, () => search(append));
        if (error.status === 400 && error.code === "invalid_cursor") messages.querySelector(".notice").append(button("Start a new search", () => search(false)));
      } finally {
        if (request === generation) results.setAttribute("aria-busy", "false");
      }
    }
    form.addEventListener("submit", event => { event.preventDefault(); q = input.value.trim(); search(); });
    document.querySelectorAll(".filter").forEach(el => el.addEventListener("click", () => { kind = el.dataset.kind; q = input.value.trim(); search(); }));
    extra.addEventListener("change", () => { kind = extra.value; q = input.value.trim(); search(); });
    more.addEventListener("click", () => { if (next && pages < 20) search(true); });
    updateFilters();
    await search();
  }
  async function artifact() {
    const container = document.getElementById("artifact");
    const params = new URLSearchParams(window.location.search);
    const kind = params.get("kind");
    const id = params.get("id");
    const version = params.get("version");
    if (!kinds.has(kind) || !id || !version || utf8Length(id) > 2048 || utf8Length(version) > 256 || id.includes("\0") || version.includes("\0")) {
      notice(container, "This link is incomplete", "Choose an artifact from the library to see its exact version and published details.");
      container.append(link("Browse your workspace →", "/workspace", "button button-outline"));
      container.setAttribute("aria-busy", "false");
      return;
    }
    try {
      const item = await api("/app/api/artifact?" + new URLSearchParams({ kind, id, version }));
      container.replaceChildren();
      const title = text(item.title, text(item.id, id));
      document.title = title + " · Gist";
      container.append(node("p", labels[item.kind] || labels[kind], "eyebrow"), node("h1", title), node("p", text(item.id, id), "artifact-id"));
      if (item.description) container.append(node("p", text(item.description), "artifact-description"));
      const facts = node("dl", undefined, "artifact-facts");
      const fact = (label, value, className) => { const entry = node("div", undefined, className); entry.append(node("dt", label), node("dd", value)); facts.append(entry); };
      fact("Version", text(item.version, version));
      fact("State", text(item.state, "Unavailable"));
      const digest = item.digest || {};
      fact("Content digest", digest.value ? text(digest.algorithm, "sha256") + ":" + text(digest.value) : "Not supplied", "digest-fact");
      container.append(facts);
      const metadata = item.metadata && typeof item.metadata === "object" ? item.metadata : {};
      const declared = node("div", undefined, "declaration-grid");
      function declaration(label, value) {
        const box = node("section", undefined, "declaration");
        box.append(node("h2", label), node("p", value));
        declared.append(box);
      }
      if (Array.isArray(metadata.required_capabilities) && metadata.required_capabilities.length) {
        const box = node("section", undefined, "declaration");
        box.append(node("h2", "Required capabilities"));
        const list = node("ul");
        for (const requirement of metadata.required_capabilities) {
          if (!requirement || typeof requirement.id !== "string") continue;
          list.append(node("li", requirement.id + (typeof requirement.contract_version === "string" ? " · " + requirement.contract_version : "") + (requirement.optional === true ? " · optional" : "")));
        }
        box.append(list);
        declared.append(box);
      }
      if (Array.isArray(metadata.effects) && metadata.effects.length) declaration("Declared effects", metadata.effects.filter(value => typeof value === "string").join(" · "));
      if (Array.isArray(metadata.required_scopes) && metadata.required_scopes.length) declaration("Declared scopes", metadata.required_scopes.filter(value => typeof value === "string").join(" · "));
      if (typeof metadata.execution_location === "string") declaration("Declared execution location", metadata.execution_location);
      if (metadata.publisher && typeof metadata.publisher === "object" && typeof metadata.publisher.id === "string") declaration("Publisher", metadata.publisher.id + (typeof metadata.publisher.license === "string" ? " · " + metadata.publisher.license : ""));
      if (typeof metadata.support_state === "string") declaration("Declared support state", metadata.support_state);
      if (declared.childNodes.length) container.append(declared);
      container.append(node("h2", "Published metadata"), node("p", "These are publisher-asserted declarations for this exact artifact version. Catalog metadata does not establish provider availability or grant permissions; your client validates the requirements."));
      const pre = node("pre");
      pre.append(node("code", JSON.stringify(metadata, null, 2)));
      container.append(pre);
      const setup = node("div", undefined, "callout");
      setup.append(node("strong", "Use through your client"), node("p", "Your client retrieves the complete package, verifies its pinned identity, and resolves the tools and connections it needs. This page does not install or run artifacts."), link("Client integration guide →", "/developers", "text-link"));
      container.append(setup);
    } catch (error) { failure(container, error, artifact); }
    finally { container.setAttribute("aria-busy", "false"); }
  }
  if (page === "home") {
    const form = document.getElementById("home-library-search");
    const input = document.getElementById("home-search");
    form.addEventListener("submit", event => {
      if (utf8Length(input.value.trim()) <= 200 && !input.value.includes("\0")) return;
      event.preventDefault();
      input.setAttribute("aria-invalid", "true");
      const help = document.getElementById("home-search-help");
      help.textContent = "This search is too long. Try a shorter phrase.";
      help.setAttribute("role", "alert");
      input.focus();
    });
    input.addEventListener("input", () => input.removeAttribute("aria-invalid"));
  }
  if (page === "signin") {
    const code = new URLSearchParams(window.location.search).get("error");
    const errors = {
      access_denied: ["This account does not have workspace access", "Use the GitHub account your workspace operator invited. Ask the operator to check your invitation if you expected access."],
      github_failed: ["GitHub sign-in did not finish", "Start a fresh sign-in below. Check that GitHub is available and that you selected the intended account."],
      service_unavailable: ["Sign-in is temporarily unavailable", "Your sign-in could not be saved. Try again shortly; if this continues, ask your workspace operator to check service readiness."]
    };
    if (Object.prototype.hasOwnProperty.call(errors, code)) {
      const card = document.querySelector(".signin-card");
      const message = node("div", undefined, "signin-error");
      message.setAttribute("role", "alert");
      message.append(node("strong", errors[code][0]), node("p", errors[code][1]));
      card.prepend(message);
    }
  }
  loadSession();
  if (page === "workspace") library();
  if (page === "artifact") artifact();
})();
