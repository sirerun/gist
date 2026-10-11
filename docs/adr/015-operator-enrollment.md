# ADR015 — RFC-002 operator enrollment identity

Date: 2026-10-11
Status: Accepted enrollment mechanism; concrete operator/workspace inputs and implementation acceptance remain open.

## Context

The founder approved permissions-free GitHub App sign-in as the identity proof
for invited Gist operators. This is an operator enrollment path. It must remain
separate from RFC-002 MCP client OAuth, Gist-issued workload grants, and Treg
or Composio provider connections.

GitHub documents that the REST `GET /user` endpoint works with GitHub App user
access tokens without permissions and returns the authenticated user's
account record. The integration needs only the stable numeric `id`; it does
not need repository, organization, email, or profile permissions. See [Get the
authenticated user](https://docs.github.com/en/rest/users/users#get-the-authenticated-user)
and [Generate a user access token for a GitHub App](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-user-access-token-for-a-github-app).

## Decision

1. Use a GitHub App user authorization flow only to verify the human operator's
   numeric account ID. Configure no repository or organization permissions and
   request no scopes that provide resource access. User authorization is still
   interactive; it authenticates the user without granting the app access to
   their repositories or organizations.
2. Exchange the authorization code server-side, call only `GET /user`, compare
   its numeric `id` with the configured invitation, and discard the GitHub
   user access token immediately. Do not persist access tokens, refresh
   tokens, authorization codes, client credentials, or GitHub private keys in
   the Gist user/session database. Use short-lived, transient request handling
   and redacted logs.
3. Invitations are exact numeric-account-ID grants to one private Gist
   workspace. They do not match mutable usernames or email addresses. The
   approved mechanism is fixed; the actual workspace and initial numeric
   operator ID remain owner inputs and no operator is enrolled until both are
   configured and verified.
4. After successful invite verification, Gist issues its own bounded workload
   grant with explicit workspace, audience, capabilities, parent authority,
   and expiry. A GitHub login does not grant execution, provider access, or
   arbitrary workspace selection.
5. MCP client OAuth and workload-token refresh remain governed by RFC-002's
   resource-server authorization design. They do not reuse the ephemeral
   GitHub token. Treg and Composio remain separately configured provider
   connections, never login authorities.
6. Offboarding disables the invite/member and revokes its Gist grants. The
   service fails closed when the configured workspace or operator allowlist is
   absent or ambiguous.

## Consequences and acceptance

The enrollment implementation must prove that no GitHub token or code reaches
persistent storage, only `GET /user` is called, the returned numeric ID is
used for exact invite matching, wrong/uninvited accounts fail closed, and
workload grants remain bound to the configured workspace and parent authority.
Live onboarding acceptance remains blocked until the owner supplies the exact
workspace and operator account.
