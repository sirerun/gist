# Publication implementation checkpoint

Source/local observations only. PUBLISH.1/2/3/4/5/6 remain open pending complete implementation, actual-store acceptance, aggregate quality checks, independent exact-head review, guarded merge and fresh landed checks.

## Observed old-source regression

Tracked source 8a24c72a596a757e13426945911d87446cc3a4e0 plus a preserved temporary probe used real TLS HTTP against App.New, actual isolated PostgreSQL16, an asserted NOSUPERUSER/NOBYPASSRLS application role and owned filesystem. All six authenticated POST /v2/publish/{kind} requests returned404 instead of201. Setup errors were corrected before this observation and are not regression evidence. This proves missing old routes only; new admission/readback/provider behavior remains unqualified.

Hosted command: go test -tags=integration ./internal/app -run ^TestV2BaselinePublicationRoutes$ -count=1 -v.
Preserved temporary probe SHA256: b33d888199d50d5e8f261b8d46e6d88381f102b7f09a65cc5d2358f77240030f.

## Integration and changed validation

First combined units passed publicationv2, REST and app but failed existing filesystem failed-upload cleanup and legacy S3 conditional-write compatibility checks. These failures keep the affected gate open and are assigned to the isolated object-safety lane without weakening assertions.

The coordinator reproduced acceptance of an unretained local file: JSON Schema reference under the default library loader. TestDynamicSchemaCannotReadUnretainedLocalFile failed before the fix and the complete publicationv2 package passed afterward. Commit91d2da9 explicitly denies unretained resources for owner and dynamic schema compilation; retained schemas and built-in dialects remain available, original authoritative contracts unchanged.

Commands: go test ./internal/publicationv2 -run ^TestDynamicSchemaCannotReadUnretainedLocalFile$ -count=1 (failed), then go test ./internal/publicationv2 -count=1 (passed).

## Open completion work

The accepted plan assigns isolated transaction/PG, object-safety and app/REST/HTTP completion. Actual-store gates cover current authorization/evidence, immutable bytes/digests, permanent replay identity, races/event-once, outbox rollback, retired-key retry, fair bounded cleanup, revocation and forced-RLS isolation. Source fixtures do not qualify deployment or providers.

Read-only default-profile AWS STS qualification returned NoCredentials. The CLI is installed, but this did not qualify a Gist production account/role/stack. Existing operator/provider/AWS/release gates remain open; no deployment, live S3/provider call, DNS mutation, new paid infrastructure or production acceptance occurred.
