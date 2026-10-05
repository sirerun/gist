# KEYS initial independent source review

Reviewer: root coordinator, nonauthor/noncoauthor of keyring.go and its tests. Covered task T-GR-KEYS.1, base235b1f3f86c66d6f943a910369688be46291e1d6, head6e8097cba2863a2f123e57ee3997edcff01f2b16. Outcome: changes required; this early source review does not satisfy final review readiness while quality verification remains pending.

KEYS-R1: Ed25519 PrivateKey.Public returns its stored public suffix, so the loader accepts an inconsistent seed when the suffix matches configured public material. Derive from seed and compare the complete key. A corrupted-seed regression must fail before correction.

KEYS-R2: retired_at accepts arbitrary future overlap, contrary to the bounded token-lifetime-plus-skew design. Enforce the agreed bound; test actual tokens minted by the old issuer across reload/rotation and expiry.

KEYS-R3: standard JSON decoding accepts duplicate fields and silently selects the last current configuration. Reject ambiguous duplicate members recursively, without parser/key contents in errors.

Fixes are owned by the original author under T-GR-KEYS.R1, followed by affected verification and independent final-head re-review. Loader component verification remains distinct from composed API startup acceptance in INTEGRATE.2.
