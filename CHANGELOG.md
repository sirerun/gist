# Changelog

## 1.0.0 (2026-03-14)


### Features

* add Phase 4 advanced features ([90e3777](https://github.com/sirerun/gist/commit/90e3777a417cfb7fbc4b6c96ed97cf70fba3240a))
* add Phase 5 open-source launch assets ([5355056](https://github.com/sirerun/gist/commit/53550561ffa3d42a5cca3f6c2f24ba7040029c86))
* **chunk:** add JSON and YAML structured chunking ([33ee8c6](https://github.com/sirerun/gist/commit/33ee8c6122c3fa3c4c4d54d39d8a563d153eda08))
* **chunk:** add Markdown-aware content chunking ([5380b22](https://github.com/sirerun/gist/commit/5380b22caf747f44c081db0f90d5a2635a5ac180))
* **cli:** add gist bench performance benchmarking command ([78908dd](https://github.com/sirerun/gist/commit/78908dd65b7cab4944573ece944ed3f5e3e0bdc3))
* **cli:** add gist CLI with index, search, stats, serve, doctor ([077a4d3](https://github.com/sirerun/gist/commit/077a4d34cd2514673e8a8fdec54f8431bcf1f93f))
* **cli:** add gist doctor diagnostics command ([28a0c51](https://github.com/sirerun/gist/commit/28a0c51ed62dfbf7f0df05fbbbdb60e41603a584))
* **cli:** add version subcommand with build info ([89476c2](https://github.com/sirerun/gist/commit/89476c2dab4ccb3e3b348dd7877b89c9c5f4d99d))
* **executor:** add polyglot subprocess executor with security policy ([13d5264](https://github.com/sirerun/gist/commit/13d52646632532692362845703b77e76a34b08fc))
* **fuzzy:** add Levenshtein vocabulary matching ([8fbde81](https://github.com/sirerun/gist/commit/8fbde81ac9f46a37308a57a27c67d0cc3d79301f))
* **gist:** add batch indexing with goroutine pool ([2a3383e](https://github.com/sirerun/gist/commit/2a3383eef19e32eb47420939e7ece8620af61392))
* **gist:** add top-level API with New, Index, Search, Stats, Close ([51b76b8](https://github.com/sirerun/gist/commit/51b76b8bcb6551204c0c0c478d7b008ded4f8956))
* **mcp:** add MCP server with gist_index, gist_search, gist_stats tools ([077a4d3](https://github.com/sirerun/gist/commit/077a4d34cd2514673e8a8fdec54f8431bcf1f93f))
* **mcp:** add MCP server with gist_index, gist_search, gist_stats tools ([48653b6](https://github.com/sirerun/gist/commit/48653b63596c815e91af0212cd8433686a287126))
* **release:** add Homebrew tap and update README with brew install ([37e02fa](https://github.com/sirerun/gist/commit/37e02fa565edab8c72fb55d5c2479d7ab350619a))
* **search:** add three-tier search with porter/trigram/fuzzy fallback ([8915e4d](https://github.com/sirerun/gist/commit/8915e4dfab2ba2cc29f2f3abfb933862d374824f))
* **session:** add session event tracking and resume snapshots ([cf3aa19](https://github.com/sirerun/gist/commit/cf3aa19a7d9c2d273d0c6e86c4fe1166c2159c88))
* **snippet:** add smart snippet extraction ([08cc806](https://github.com/sirerun/gist/commit/08cc806252912b8945637b851384b7a72cb1d02f))
* **store:** add ContentStore interface and supporting types ([08cc806](https://github.com/sirerun/gist/commit/08cc806252912b8945637b851384b7a72cb1d02f))
* **store:** add PostgreSQL backend with tsvector and pg_trgm ([8915e4d](https://github.com/sirerun/gist/commit/8915e4dfab2ba2cc29f2f3abfb933862d374824f))


### Bug Fixes

* **ci:** use goinstall mode for golangci-lint and remove duplicate release job ([3e2d24b](https://github.com/sirerun/gist/commit/3e2d24b2f2e1a341d0ea2c066b4bb736991a8dec))
* **lint:** handle errcheck violations in production code, exclude test files ([56fae30](https://github.com/sirerun/gist/commit/56fae3095a78d734ad3c8aa00c26e62ae157a799))
* **lint:** handle unchecked error returns flagged by errcheck ([9ff3ae0](https://github.com/sirerun/gist/commit/9ff3ae0f7cd377538293836b6a0cbc33f3277dfb))
* **lint:** resolve all golangci-lint findings ([4e9ab4d](https://github.com/sirerun/gist/commit/4e9ab4d96284e0f61df970d140a40a7740cb633b))
