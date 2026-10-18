# Browser checks

`browser_check.py` exercises the actual embedded website with synthetic API
responses in an isolated Chromium context. It never signs into GitHub, connects
provider accounts, calls models, or changes a production workspace. The script
requires an owned localhost server running `webui.Handler()` and Python
Playwright 1.58.0 with a compatible Chromium executable.

```sh
python browser_check.py --base-url http://127.0.0.1:PORT \
  --output /path/to/private/artifacts --chromium /path/to/chromium
```

The runner owns server launch/readiness/cleanup and bounds execution. The check
emits desktop/mobile screenshots and a JSON receipt. It covers public pages,
sign-in target, signed-out access, search and Unicode query limits, kind filters,
paging, empty and unavailable states, exact details, hostile metadata, retry,
logout, and layout at 320, 390, 768 and 1440 pixels. An uncaught browser error or
horizontal overflow fails the run.

The Go `internal/webui` tests cover real public HTTP routes, methods, assets and
security headers. `internal/app` integration tests use PostgreSQL and a restricted
tenant role to exercise the actual browser APIs, membership revocation, isolation
and metadata projection. Enrollment tests cover the fixed return target. These
checks complement one another; intercepted browser responses do not qualify
live identity, provider or authenticated production behavior.
