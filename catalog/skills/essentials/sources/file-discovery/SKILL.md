---
name: file-discovery
description: Find relevant files and compare their metadata without opening executable content.
license: Apache-2.0
---

# Read-only file discovery

**Screening candidate v0.1.0. Not an admitted skill, implemented provider binding,
qualified workflow or permission grant.** `assets/requirements.json` contains
semantic draft requirements, not executable tool schemas. Stop if the client
has not explicitly reviewed this exact package and mapped every draft requirement
to current authorized read-only tools with validated input/output schemas.

1. Confirm one selected storage connection, the search intent and any user-specified folder boundary. Use up to two admitted search calls and at most 20 total file references.
2. Select at most five promising results and read their metadata through the admitted metadata binding on that same storage connection. Inspect title, owner alias, modified time, type and provider-returned snippet only when available.
3. Rank results using those observed fields and label relevance judgments as inference. A snippet or title does not establish the contents of a document. Do not claim a full-document summary.
4. Return the file-finding template with provider source references, observed timestamps and search limitations. Opening file content requires a separately designed and admitted read capability.

Never upload, move, rename, share, delete, edit or change permissions; download files; run macros; execute files; follow document links; or search other accounts.

Budget: One storage connection; at most two searches and five metadata reads, 20 total search references. No content downloads. Stop when any declared bound is reached and report partial
coverage. Validate outputs before use; never infer a successful call or silently
retry a failed or ambiguous one. Do not place credentials, session tokens or
unrelated personal details in the result. Gist supplies this inert content; the
client owns account selection, current authorization, least-privilege scopes,
provider costs, network policy, execution, cancellation and revocation checks.

Do not infer real-world verification from source or synthetic package tests.
Use `assets/outcome-template.md` for the result and disclose every material limit.
