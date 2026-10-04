# Gist source preflight — T-GR-SCOPE.1

Date: 2026-10-04. Coordinator source-only receipt; no code or live acceptance.

- Local, fetched and remote main: `84e14128565058419a9b90619f27fa517ba4f494`.
- Open pull requests: none at inspection. GitHub authentication succeeds. Main rules and CI policy are independently inspected by T-GR-SCOPE.8.
- Original checkout has preexisting CLAUDE.md type change plus the authored planning/assessment documents; all are preserved. New isolated coordinator and three read-only worker branches start from exact remote main on the mounted external SSD.
- Native workers: gist_capacity owns only capacity receipt; gist_ci owns only CI receipt; gist_providers owns only provider receipt. Coordinator owns plan/requirements/design/contracts and joins receipts. Each worker has its own atomic task claim.
- Other live project sessions were identified through process IDs and their working directories; none besides this session was found in Gist. Private process paths/identifiers stay outside the public receipt.
- `hosted/internal/app/app.go` generates a fresh startup EdDSA key per replica and instantiates an in-memory event feed. Its resolver adapter accepts legacy skill/runtime fields. The app command does not configure an external login path. Existing OAuth/session seams do not establish external enrollment.
- `hosted/migrations/004_events.sql` contains durable outbox/cursor schema with forced RLS, but app composition does not use it. `storage.RevokeVersion` commits before app event append; the append error is ignored. No fabricated atomicity/replica evidence is claimed.
- Composio catalog remains source-unavailable/synthetic; actual provider and consumer qualification remain independent required gates.

Engineering build sanity is held under the shared host rule: one-minute load was above 10. This inspection wave has no build/test target; no build was started and no other owner's build lease was touched. Capacity/source preflight completion does not admit engineering build stages.
