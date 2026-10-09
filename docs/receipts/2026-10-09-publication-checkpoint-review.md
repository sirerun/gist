# Historical checkpoint review

This negative report at7058490 is retained. Accepted PUB-CLOSEOUT-PRIVACY-1 has tracked fix/verify/re-review PUBLISH.21/.22/.23; exact head/base guard remains required for corrected artifact delivery.

**REQUEST_CHANGES — exact base `eb1bd86596b61bd423cc3f75757a2bdd38682c24`, exact head `705849008800e33f973fc0f81829bb10cfba435b`.**

Finding **PUB-CLOSEOUT-PRIVACY-1**: [the landing receipt](../../docs/receipts/2026-10-09-publication-landed.md#L54) publishes private home/worktree paths paths, including the user’s home directory and internal worktree layout. Replace the command with a portable repo-relative form before publishing.

The remaining reviewed evidence is consistent: both commits resolve to the requested SHAs; the diff touches only the three scoped documents; the preservation record identifies exactly PUBLISH.4/.5/.6/.17/.20 as status changes and reports owners, stages, kinds, acceptance, dependencies, and other repository files preserved. The landed provenance records reviewed/landed tree parity and remote-main reachability. The saved closeout Wazi record matches the candidate plan digest, reports 278 valid tasks, and retains `authorityAuthenticated=false`. No production terminal is marked complete. `git diff --check` is clean.

This is a documentation closeout review only; I did not rerun source review, builds, or test suites.

## Corrected privacy source approval

This actual R2 approval closes the privacy re-review source obligation at the recorded head. Recording it changes checkpoint metadata; the final artifact head still requires fresh exact-head review and guarded merge before delivery.

**APPROVE — exact base `eb1bd86596b61bd423cc3f75757a2bdd38682c24`, exact head `0c1c9ae0432ab08354d1552712a598b30da498c5`.** I found no remaining blocker in the R2 privacy correction.

The candidate is HEAD, the base and candidate resolve to the pinned commits, and the full diff is limited to the five stated documentation files. `git diff --check` passes, and the worktree is clean. The 281-task Wazi record matches the current plan digest; reader compatibility, schema, and semantic checks pass, with `authorityAuthenticated=false`. The plan retains the three new privacy-fix, verification, and review tasks; `.23` remains open.

The landing receipt’s four published hashes match the private receipt files. Its claims preserve the distinction between local evidence and hosted CI, describe the AWS and DNS observations narrowly, and leave production gates open. Current command tables use portable forms; I found no private home paths, hosts, or addresses in the reviewed receipts. The historical negative review is retained and explicitly makes no claim of Git-history erasure. The earlier source approvals and landed proof remain evidence about the code already approved and landed through PR60; this disposition is for the documentation checkpoint only.

**Finding PUB-CLOSEOUT-PRIVACY-1: resolved.** No source review, build, or test suite was rerun.
