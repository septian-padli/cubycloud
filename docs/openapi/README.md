# CubyCloud OpenAPI 3.1 & Swagger UI Maintenance Guide

CubyCloud maintains an **OpenAPI 3.1.0** specification for all backend HTTP endpoints. The specification is authored as modular YAML source fragments and deterministically bundled into a single self-contained artifact that powers the interactive Swagger UI.

---

## 1. Directory Structure

The OpenAPI source files live under `docs/openapi/`:

```text
docs/
  openapi.yaml                   # Bundled OpenAPI 3.1 specification (generated artifact)
  openapi/
    README.md                    # Contributor maintenance guide (this file)
    root.yaml                    # Entry metadata, server URLs, tags, security definitions
    paths/
      admin.yaml                 # Super Admin endpoints (/api/v1/admin/*)
      auth.yaml                  # Authentication & user profile endpoints (/api/v1/auth/*)
      core.yaml                  # /health, /api/v1/events (SSE), and /api/docs endpoints
      drive-common.yaml          # Common drive endpoints (/api/v1/drive/items/{id}/*, home, etc.)
      my-storage.yaml            # Personal storage VFS, TUS, and multipart uploads
      organization.yaml          # Shared organization drive endpoints
      public-share.yaml          # Public token links and public-link write operations
      search-jobs.yaml           # Global search and background job queue endpoints
      setup.yaml                 # System initialization & storage onboarding endpoints
      share.yaml                 # Internal item sharing and Shared With Me endpoints
      trash.yaml                 # Soft-delete trash listing, restore, and purging
    components/
      admin.yaml                 # Reusable schemas for admin operations
      auth.yaml                  # Reusable schemas for login, profile, and passwords
      common.yaml                # Shared schemas (ErrorResponse, StatusOkResponse, Breadcrumb)
      core.yaml                  # SSE event models and health schemas
      organization.yaml          # Organization item and folder models
      setup.yaml                 # Onboarding setup schemas
      sharing.yaml               # Share permission, grant, and public link models
      storage.yaml               # VFS item models, TUS headers, multipart state schemas
```

A synchronized copy of the bundled spec is embedded directly into the Go backend binary at:
`apps/backend/internal/modules/docs/openapi.yaml`

---

## 2. Interactive Swagger UI & Access Control

The interactive Swagger UI is served at:
- **Interactive UI**: `GET /api/docs`
- **Raw Specification**: `GET /api/docs/openapi.yaml`

### Access Control
Both documentation routes are strictly restricted to **Super Admin** users:
- **Unauthenticated requests** receive `401 Unauthorized` (`{"error": "authentication required"}`).
- **Authenticated regular users** (`REGULAR` role) receive `403 Forbidden` (`{"error": "super admin access required"}`).
- **Super Admins** (`SUPER_ADMIN` role) receive `200 OK`.

### Try it out & Cookie Authentication
The Swagger UI includes a pre-configured request interceptor that forwards the current browser session:
```javascript
requestInterceptor: function(req) {
  req.credentials = 'same-origin';
  return req;
}
```
When accessing Swagger UI in a browser where you are logged in as Super Admin, clicking **Try it out** and **Execute** automatically sends the HTTP-only `cuby_token` cookie. No manual JWT copying or credential entry is required.

---

## 3. How to Update an Endpoint Definition

1. Locate the appropriate file in `docs/openapi/paths/` based on the URL prefix:
   - Auth endpoints: `paths/auth.yaml`
   - File & folder CRUD: `paths/my-storage.yaml` or `paths/organization.yaml`
   - Sharing & public links: `paths/share.yaml` or `paths/public-share.yaml`
   - Admin APIs: `paths/admin.yaml`
2. Update the operation object under the matching path. Use standard OpenAPI 3.1 syntax:
   - Ensure the path uses `{param}` syntax (e.g. `/api/v1/jobs/{id}`, not `:id`).
   - Provide a unique and stable `operationId` (e.g. `getJob`, `createStorageNodeLocal`).
   - Document request bodies, query parameters, path parameters, and headers.
   - Accurately document success status codes (e.g. `200`, `201`, `204`, `206`).
   - Document error status codes (e.g. `400`, `401`, `403`, `404`, `409`, `413`, `416`, `500`, `503`).
3. Re-bundle and validate:
   ```bash
   make docs-bundle
   make docs-check
   ```

---

## 4. How to Add Schemas & Examples

1. Place reusable schemas in `docs/openapi/components/<module>.yaml` or `docs/openapi/components/common.yaml` for shared models.
2. In path definitions, reference schemas using JSON pointer notation:
   ```yaml
   schema:
     $ref: '#/components/schemas/YourSchemaName'
   ```
3. Include realistic examples for both request bodies and response payloads. Do not use placeholders or generic unbounded objects when concrete Go DTOs exist.

---

## 5. Bundling, Validation & Route Coverage Verification

### Bundling
Run the automated bundler to compile all modular YAML fragments into `docs/openapi.yaml` and sync the Go embedded copy:
```bash
make docs-bundle
```
Behind the scenes, this executes:
```bash
cd apps/backend && go run ./cmd/openapi-bundler -source ../../docs/openapi -output ../../docs/openapi.yaml -backend-copy internal/modules/docs/openapi.yaml
```

### Validation & Route Coverage
Run the validation suite:
```bash
make docs-check
```
The automated validation suite tests:
1. **YAML Syntax**: Parses every individual `.yaml` fragment in `docs/openapi/`.
2. **OpenAPI 3.1 Compliance**: Verifies the specification root and structural schemas.
3. **Reference Integrity**: Asserts that every `#/components/schemas/...` `$ref` resolves to a real defined schema.
4. **Operation ID Uniqueness**: Asserts that every operation has a unique, non-empty `operationId`.
5. **100% Route Coverage**: Enumerates all HTTP routes directly from Echo route registration (`buildEchoRouter`), normalizes `:id` to `{id}`, and verifies that:
   - Every registered backend route has corresponding documentation.
   - Every documented operation corresponds to an actual registered route.
   - If an endpoint is added or deleted in Go code without updating OpenAPI, tests fail immediately.
6. **Freshness & Staleness**: Confirms that `docs/openapi.yaml` and `apps/backend/internal/modules/docs/openapi.yaml` match the source fragments exactly.
7. **Access Control**: Validates 401/403/200 protections on `/api/docs` and `/api/docs/openapi.yaml`.

---

## 6. Viewing Swagger UI Locally

1. Start the CubyCloud development backend:
   ```bash
   make dev-backend
   ```
2. Log in as a Super Admin user via the frontend or API (`POST /api/v1/auth/login`) to obtain the `cuby_token` cookie.
3. Open your browser and navigate to:
   ```
   http://localhost:8383/api/docs
   ```
4. Swagger UI will load offline with no internet access or CDN dependencies required. Use **Try it out** to execute live API calls against your local instance.

---

## 7. Troubleshooting Validation Failures

| Failure Message | Cause | Resolution |
| --- | --- | --- |
| `YAML error in ...: did not find expected ',' or '}'` | Unbalanced braces or unquoted colons/semicolons in flow mappings | Wrap descriptions with colons or semicolons in double quotes (e.g. `description: "..."`), or use block style. |
| `Discovered N undocumented backend route(s)` | An endpoint was added to the Go backend without being documented in `docs/openapi/paths/` | Add the missing route and methods to the corresponding YAML file, then run `make docs-bundle`. |
| `Discovered N documented operation(s) not registered` | An operation exists in OpenAPI but has no matching Echo route | Check the path syntax and HTTP method. Ensure parameter names match Echo route registrations. |
| `Unresolved schema references: [...]` | A `$ref` references a schema not present in `components/*.yaml` | Define the missing schema in the appropriate `components/` file or correct the typo. |
| `bundled file docs/openapi.yaml is stale` | Source YAML fragments were edited without re-bundling | Run `make docs-bundle` to update the bundled YAML files. |
| `Duplicate operationId '...'` | Two operations share the same operationId | Assign unique, descriptive names to both operations. |
