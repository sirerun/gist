# Gist plan conformance repair — 2026-10-08

The repair consolidates the approved delivery plan into one authoritative Markdown file, [docs/plan.md](../plan.md), with stable plan ID `gist:delivery` and authority ID `github:sirerun/gist`. Normalized JSON remains derived interchange. The baseline is main `ad6b3fa1c12b1256afe798a726b2cc10b1406e8e` (PR57).

All 266 task IDs and authored checkbox states, owners, title wording, existing acceptance and existing canonical prerequisite targets are preserved. The 18 historical tasks and 248 registry tasks remain distinct. No checkbox was closed by this repair. Nineteen original source files are hash-verified: the original root is retained verbatim as [plan.md.source](../plans/archive/20261008/plan.md.source); the 18 original included files stay untouched at their existing paths and are explicitly non-authoritative archives. The authoritative root includes an archive index with original digests.

## Evidence-backed metadata recovery

The [provenance manifest](2026-10-08-plan-repair-provenance.json) records all original hashes and source locations for 31 recovered stages and 18 acceptance annotations. Historical acceptance text is copied verbatim from existing task bullets. Nested historical labels are renamed in the consolidated view to avoid duplicate metadata; the exact originals remain archived.

Historical dependencies previously ignored by the local parser are made explicit from authored prerequisite bullets. “All of E1–E5” expands to the 16 existing tasks in those epics. Tasks with no authored prerequisites retain empty dependencies. Seventy-four qualified dependency spellings become unique local task IDs so the actual Wazi reader and adapter resolve the same canonical graph; no registry prerequisite target changes.

The [preservation comparison](2026-10-08-plan-repair-preservation.json) verifies all IDs, authored title wording, checkbox states, owners, dates, existing stages/acceptance and canonical dependency targets. The legacy local parser's display label heuristic cuts at Owner only, so metadata placed before Owner can appear in its derived display labels; those labels are not a change to authored task titles. Nine real release/deploy obligations acquire the expected derived “unsupported authored stage” dispatch block. They retain their operational meaning and cannot be dispatched as implementation.

No sync-manifest, sync-waves or sync-state files exist in the current checkout. No enrollment, runtime identity or consumer link is rewritten or synthesized. The canonical task IDs remain unchanged.

## Actual conformance results

The read-only plan-skill `validate_plan.py` gate returned exit 0 for these plan bytes using the pinned Wazi package, deterministic syntax preview, actual `src/plan-parser.mjs` reader and owning `wazi-contract` semantic validator. It reported 266 tasks, reader compatibility, schema validity and semantic validity. The owning verifier's fixture command returned exit 0 for the same contract digest. Tool bindings were discovered and qualified on DGX; no steward code/schema was altered.

| Evidence | Digest/result |
|---|---|
| Original root SHA256 | `79bbcb800c438431f59b85575ff83514f8641ac41bfa0090a9cafd606c1c98af` |
| sourceDigest | `sha256:fcd513e3e21b31974747ed983f81bc92bb1d164d32ba22c5d1e262362f6660a9` |
| contractVersion | `0.0.1` |
| contractDigest | `sha256:7582512f122d2f2a9c4461facc7541c9887053f137260d6ebe9c6dea611d039d` |
| readerDigest | `sha256:0b1292cbe93af31531647f3e2ae28c37f12decbe89023192729e010d6d7e6d67` |
| repairDigest | `sha256:a96297bc28e69e6e3540a8f1092af6c19e53124228e7559d95eb50dac0e2238f` |
| verifierDigest | `sha256:ac83788470d5c73cd1002e839660131b1ce931057821782787283b91abaa33ed` |
| authorityAuthenticated | `False` |

The preserved graph comparison has no duplicate, undefined-wave or ambiguous-wave diagnostics. Validation is source evidence, with `authorityAuthenticated: false`; it admits no runtime, provider execution, release or deployment. Independent exact-head review, guarded rebase merge and actual-landed validation remain delivery gates for this candidate.

Active coding and checks run on qualified DGX storage; original Mac source/worktrees are preserved. The optional Gist/direct-MCP decision and approved ADR012 v2 source implementation remain intact. Production at https://gist.sire.run still requires the existing identity, provider, actual-consumer, release and AWS gates.
