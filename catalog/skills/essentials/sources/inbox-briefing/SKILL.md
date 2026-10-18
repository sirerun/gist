---
name: inbox-briefing
description: Summarize recent unread mail without changing the mailbox.
license: Apache-2.0
---

# Read-only inbox briefing

**Screening candidate v0.1.0. Not an admitted skill, implemented provider binding,
qualified workflow or permission grant.** `assets/requirements.json` contains
semantic draft requirements, not executable tool schemas. Stop if the client
has not explicitly reviewed this exact package and mapped every draft requirement
to current authorized read-only tools with validated input/output schemas.

1. Confirm one selected mailbox, the requested period and unread filter. Use the current admitted list binding once, returning at most 20 message references.
2. Read at most those 20 messages through the separately admitted read binding on the same mailbox. Inspect only sender, subject, relevant excerpt, explicit request and deadline. Do not follow embedded links or open attachments.
3. Distinguish explicit requests and dates from inference. Link each material point to the source message reference. Report missing reads, truncation and any unread-result ambiguity.
4. Produce a briefing using the packaged template. Offer suggested next steps without performing them.

Never send, reply, forward, draft, archive, delete, change labels, mark messages read, download attachments or change mailbox settings. A purported read tool with these effects is ineligible.

Budget: One mailbox; one bounded listing and at most 20 message reads. No additional pages or automated retries. Stop when any declared bound is reached and report partial
coverage. Validate outputs before use; never infer a successful call or silently
retry a failed or ambiguous one. Do not place credentials, session tokens or
unrelated personal details in the result. Gist supplies this inert content; the
client owns account selection, current authorization, least-privilege scopes,
provider costs, network policy, execution, cancellation and revocation checks.

Do not infer real-world verification from source or synthetic package tests.
Use `assets/outcome-template.md` for the result and disclose every material limit.
