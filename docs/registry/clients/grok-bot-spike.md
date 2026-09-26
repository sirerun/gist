# Grok Bot identification spike

Status: **UNRESOLVED — excluded from acceptance.**

Time box: 30 minutes. Date: 2026-09-25. Scope was read-only inspection of
`/Applications/Grok Bot.app`; no app launch, network request, configuration
edit, bridge, or consumer-repository change was made.

## Evidence collected

| Check | Result |
| --- | --- |
| `Info.plist` | `CFBundleDisplayName=Grok Bot`; `CFBundleIdentifier=com.anysphere.sand`; `CFBundleShortVersionString=0.58.0`; Electron-style `AtomApplication`; copyright `SpaceXAI`. |
| Code signature | arm64 app bundle; Team ID `DCNK4UB866`; notarization ticket stapled; executable identifier `com.anysphere.sand`. |
| URL schemes | `grokbot://` and `sand://`, registered as viewer schemes. |
| Vendor identification | Unresolved. SpaceXAI copyright identifies a vendor signal, while `com.anysphere.sand` and Electron/native names are Cursor/Anysphere-family signals. This is not enough to assert the product owner. |
| Transport | Unresolved. Bundle metadata and URL schemes do not identify a supported remote MCP, Streamable HTTP, SSE, REST, or other registry transport. |
| OAuth | Unresolved. No OAuth/OIDC/PKCE support was established by the permitted metadata/signature/scheme inspection. The custom schemes alone are not OAuth evidence. |

The app bundle contains Cursor-family implementation names, but that is an
implementation signal, not proof of public product compatibility or an
accepted transport. The spike therefore does not qualify Grok Bot for the
client matrix.

## Required follow-up

Run these commands on a machine where the app and its signed resources are
reachable, then attach redacted output to a new dated spike record:

```sh
APP="/Applications/Grok Bot.app"
plutil -p "$APP/Contents/Info.plist"
codesign -dv --verbose=4 "$APP" 2>&1
/usr/libexec/PlistBuddy -c 'Print :CFBundleURLTypes' "$APP/Contents/Info.plist"
find "$APP/Contents/Resources" -maxdepth 2 -type f -print
strings "$APP/Contents/Resources/app.asar" | rg -i 'oauth|openid|pkce|mcp|streamable|sse|websocket|https?://'
```

Then, using only a disposable test account and a public build's documented
interfaces, identify the vendor, supported transport, authorization flow
(including OAuth issuer, redirect, PKCE/resource behavior), and whether the
client can retrieve exact Gist artifacts without a global-config mutation.
Record a named build and a passing end-to-end receipt before changing this
status. Until then, Grok Bot is not an acceptance line and no bridge is
assumed.
