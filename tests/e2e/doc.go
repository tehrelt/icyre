// Package e2e holds end-to-end tests (EPIC-037) that drive the whole compose
// stack through the API Gateway: the Catalog, media upload/transcode and
// playback flows. They are behind the e2e build tag and skip unless
// E2E_BASE_URL is set; run them with `make test-e2e` against `make up`.
//
// Environment:
//
//	E2E_BASE_URL      API Gateway, e.g. http://localhost:8080 (required)
//	E2E_CATALOG_URL   Catalog Service for writes the gateway does not expose
//	                  (default http://localhost:8081)
//	E2E_DATABASE_DSN  PostgreSQL, used only to grant the ARTIST role for the
//	                  upload flow (there is no admin API yet); the upload test
//	                  skips without it
//	E2E_TIMEOUT       wait for asynchronous steps (default 2m)
package e2e
