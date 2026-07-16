package api

import "net/http"

func (h *Handler) getOpenAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(openAPISpecJSON))
}

func (h *Handler) getDocs(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(apiDocsHTML))
}

const openAPISpecJSON = `{
  "openapi": "3.1.0",
  "info": {
    "title": "Image Roundup API",
    "version": "1.0.0",
    "description": "Read-only image inventory and update detection API for Kubernetes and OpenShift."
  },
  "servers": [
    { "url": "/api/v1" }
  ],
  "tags": [
    { "name": "inventory", "description": "Image inventory and status summaries" },
    { "name": "scan", "description": "Scan status and manual scan triggers" },
    { "name": "settings", "description": "Runtime settings" },
    { "name": "system", "description": "Health and metrics endpoints" }
  ],
  "paths": {
    "/summary": {
      "get": {
        "tags": ["inventory"],
        "summary": "Get summary counts",
        "description": "Returns current image counts by status and the last completed scan timestamp.",
        "responses": {
          "200": {
            "description": "Summary counts",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/Summary" }
              }
            }
          }
        }
      }
    },
    "/summary/updates": {
      "get": {
        "tags": ["inventory"],
        "summary": "Get concise update summary",
        "description": "Returns a compact list of image updates suitable for cron jobs and notification integrations.",
        "responses": {
          "200": {
            "description": "Concise update summary",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/UpdatesSummary" }
              }
            }
          }
        }
      }
    },
    "/images": {
      "get": {
        "tags": ["inventory"],
        "summary": "List image records",
        "description": "Returns discovered image records. Query parameters can filter by namespace, registry, workload kind, status, or search text.",
        "parameters": [
          { "name": "search", "in": "query", "schema": { "type": "string" }, "description": "Searches image, namespace, workload, and container" },
          { "name": "namespace", "in": "query", "schema": { "type": "string" }, "description": "Exact namespace match" },
          { "name": "registry", "in": "query", "schema": { "type": "string" }, "description": "Exact registry host match" },
          { "name": "kind", "in": "query", "schema": { "type": "string" }, "description": "Workload kind, case-insensitive" },
          { "name": "status", "in": "query", "schema": { "$ref": "#/components/schemas/ImageStatus" }, "description": "Exact status value" }
        ],
        "responses": {
          "200": {
            "description": "Image records",
            "content": {
              "application/json": {
                "schema": {
                  "type": "array",
                  "items": { "$ref": "#/components/schemas/ImageRecord" }
                }
              }
            }
          }
        }
      }
    },
    "/images/{id}": {
      "get": {
        "tags": ["inventory"],
        "summary": "Get one image record",
        "parameters": [
          { "name": "id", "in": "path", "required": true, "schema": { "type": "string" }, "description": "Record ID: namespace:workloadKind:workloadName:containerName" }
        ],
        "responses": {
          "200": {
            "description": "Image record",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/ImageRecord" }
              }
            }
          },
          "404": {
            "description": "Not found",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/Error" }
              }
            }
          }
        }
      }
    },
    "/registries": {
      "get": {
        "tags": ["inventory"],
        "summary": "List registry summaries",
        "responses": {
          "200": {
            "description": "Registry summaries",
            "content": {
              "application/json": {
                "schema": {
                  "type": "array",
                  "items": { "$ref": "#/components/schemas/RegistryInfo" }
                }
              }
            }
          }
        }
      }
    },
    "/scan": {
      "get": {
        "tags": ["scan"],
        "summary": "Get scan status",
        "responses": {
          "200": {
            "description": "Current scan status",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/ScanStatus" }
              }
            }
          }
        }
      },
      "post": {
        "tags": ["scan"],
        "summary": "Start a scan",
        "description": "Starts a full scan, namespace scan, or workload scan. The scan runs asynchronously.",
        "requestBody": {
          "required": false,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/ScanRequest" },
              "examples": {
                "full": { "summary": "Full scan", "value": {} },
                "namespace": { "summary": "Namespace scan", "value": { "namespace": "media" } },
                "workload": { "summary": "Workload scan", "value": { "namespace": "media", "workloadKind": "Deployment", "workloadName": "plex" } }
              }
            }
          }
        },
        "responses": {
          "202": {
            "description": "Scan started",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/Message" }
              }
            }
          },
          "400": {
            "description": "Invalid request",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/Error" }
              }
            }
          },
          "409": {
            "description": "Scan already running",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/Error" }
              }
            }
          }
        }
      }
    },
    "/settings": {
      "get": {
        "tags": ["settings"],
        "summary": "Get runtime settings",
        "responses": {
          "200": {
            "description": "Current settings",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/Settings" }
              }
            }
          }
        }
      },
      "put": {
        "tags": ["settings"],
        "summary": "Replace runtime settings",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/Settings" }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Updated settings",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/Settings" }
              }
            }
          },
          "400": {
            "description": "Invalid JSON",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/Error" }
              }
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "ImageStatus": {
        "type": "string",
        "enum": ["up_to_date", "update_available", "unknown", "check_failed"]
      },
      "Summary": {
        "type": "object",
        "properties": {
          "totalImages": { "type": "integer" },
          "upToDate": { "type": "integer" },
          "updatesAvailable": { "type": "integer" },
          "unknown": { "type": "integer" },
          "checkFailed": { "type": "integer" },
          "uniqueRegistries": { "type": "integer" },
          "lastScan": { "type": ["string", "null"], "format": "date-time" }
        }
      },
      "UpdatesSummary": {
        "type": "object",
        "properties": {
          "count": { "type": "integer" },
          "lastScan": { "type": ["string", "null"], "format": "date-time" },
          "updates": {
            "type": "array",
            "items": { "$ref": "#/components/schemas/UpdateSummary" }
          }
        }
      },
      "UpdateSummary": {
        "type": "object",
        "properties": {
          "id": { "type": "string" },
          "image": { "type": "string" },
          "currentVersion": { "type": "string" },
          "latestVersion": { "type": "string" },
          "namespace": { "type": "string" },
          "workload": { "type": "string" },
          "workloadKind": { "type": "string" },
          "workloadName": { "type": "string" },
          "containerName": { "type": "string" },
          "management": { "$ref": "#/components/schemas/ManagementInfo" },
          "registry": { "type": "string" },
          "repository": { "type": "string" },
          "updateReason": { "type": "string", "enum": ["newer_version_tag", "digest_changed"] },
          "lastChecked": { "type": ["string", "null"], "format": "date-time" },
          "runningDigest": { "type": "string" },
          "registryDigest": { "type": "string" },
          "latestTagDigest": { "type": "string" }
        }
      },
      "ImageRecord": {
        "type": "object",
        "properties": {
          "id": { "type": "string" },
          "namespace": { "type": "string" },
          "workloadKind": { "type": "string" },
          "workloadName": { "type": "string" },
          "containerName": { "type": "string" },
          "management": { "$ref": "#/components/schemas/ManagementInfo" },
          "configuredImage": { "type": "string" },
          "registry": { "type": "string" },
          "repository": { "type": "string" },
          "tag": { "type": "string" },
          "runningDigest": { "type": "string" },
          "registryDigest": { "type": "string" },
          "indexDigest": { "type": "string" },
          "latestTag": { "type": "string" },
          "latestTagDigest": { "type": "string" },
          "platform": { "type": "string" },
          "status": { "$ref": "#/components/schemas/ImageStatus" },
          "podNames": { "type": "array", "items": { "type": "string" } },
          "lastChecked": { "type": ["string", "null"], "format": "date-time" },
          "error": { "type": "string" }
        }
      },
      "ManagementInfo": {
        "type": "object",
        "properties": {
          "tool": { "type": "string" },
          "managedBy": { "type": "string" },
          "helmReleaseName": { "type": "string" },
          "helmReleaseNamespace": { "type": "string" }
        }
      },
      "RegistryInfo": {
        "type": "object",
        "properties": {
          "hostname": { "type": "string" },
          "imageCount": { "type": "integer" },
          "reachable": { "type": ["boolean", "null"] },
          "authPresent": { "type": "boolean" },
          "lastError": { "type": "string" }
        }
      },
      "ScanStatus": {
        "type": "object",
        "properties": {
          "running": { "type": "boolean" },
          "lastScan": { "type": ["string", "null"], "format": "date-time" },
          "imageCount": { "type": "integer" },
          "errors": { "type": "array", "items": { "type": "string" } }
        }
      },
      "ScanRequest": {
        "type": "object",
        "properties": {
          "namespace": { "type": "string" },
          "workloadKind": { "type": "string" },
          "workloadName": { "type": "string" }
        }
      },
      "Settings": {
        "type": "object",
        "properties": {
          "scanIntervalSeconds": { "type": "integer" },
          "includedNamespaces": { "type": "array", "items": { "type": "string" } },
          "excludedNamespaces": { "type": "array", "items": { "type": "string" } },
          "includeCompletedPods": { "type": "boolean" },
          "excludeInternalRegistry": { "type": "boolean" },
          "registryTimeoutSeconds": { "type": "integer" },
          "theme": { "type": "string", "enum": ["system", "light", "dark"] },
          "shortDigests": { "type": "boolean" }
        }
      },
      "Message": {
        "type": "object",
        "properties": {
          "message": { "type": "string" }
        }
      },
      "Error": {
        "type": "object",
        "properties": {
          "error": { "type": "string" }
        }
      }
    }
  }
}`

const apiDocsHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Image Roundup API Docs</title>
  <style>
    :root {
      color-scheme: light dark;
      --bg: #f4f4f4;
      --panel: #ffffff;
      --panel-alt: #f8f9fb;
      --text: #161616;
      --muted: #525252;
      --border: #d0d7de;
      --blue: #0f62fe;
      --green: #24a148;
      --red: #da1e28;
      --purple: #8a3ffc;
      --shadow: 0 10px 28px rgba(0, 0, 0, .08);
    }

    @media (prefers-color-scheme: dark) {
      :root {
        --bg: #161616;
        --panel: #262626;
        --panel-alt: #393939;
        --text: #f4f4f4;
        --muted: #c6c6c6;
        --border: #525252;
        --shadow: none;
      }
    }

    * { box-sizing: border-box; }
    body {
      margin: 0;
      background: var(--bg);
      color: var(--text);
      font: 16px/1.5 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    }
    a { color: var(--blue); }
    header {
      border-top: 4px solid #ee0000;
      background: var(--panel);
      box-shadow: var(--shadow);
    }
    .hero {
      max-width: 1180px;
      margin: 0 auto;
      padding: 48px 24px 36px;
    }
    .title-row {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 12px;
    }
    h1 {
      margin: 0;
      font-size: clamp(2rem, 5vw, 3.75rem);
      line-height: 1;
      letter-spacing: 0;
    }
    h2 { margin: 0 0 16px; font-size: 1.35rem; }
    .badge {
      display: inline-flex;
      align-items: center;
      height: 28px;
      padding: 0 10px;
      border-radius: 14px;
      background: var(--green);
      color: #fff;
      font-weight: 700;
      font-size: .85rem;
    }
    .badge.secondary { background: var(--purple); }
    .subtitle {
      max-width: 820px;
      margin: 16px 0 0;
      color: var(--muted);
      font-size: 1.05rem;
    }
    .toolbar {
      display: grid;
      grid-template-columns: minmax(220px, 360px) 1fr;
      gap: 16px;
      max-width: 1180px;
      margin: 0 auto;
      padding: 24px;
    }
    .field label {
      display: block;
      margin-bottom: 6px;
      color: var(--muted);
      font-size: .85rem;
      font-weight: 700;
    }
    input, textarea, select {
      width: 100%;
      border: 1px solid var(--border);
      border-bottom: 2px solid var(--muted);
      background: var(--panel);
      color: var(--text);
      padding: 11px 12px;
      font: inherit;
    }
    textarea {
      min-height: 120px;
      resize: vertical;
      font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      font-size: .9rem;
    }
    main {
      max-width: 1180px;
      margin: 0 auto;
      padding: 36px 24px 72px;
    }
    .tag-section {
      margin-bottom: 42px;
    }
    .tag-title {
      display: flex;
      align-items: center;
      gap: 12px;
      padding-bottom: 10px;
      border-bottom: 1px solid var(--border);
    }
    .endpoint {
      margin: 12px 0;
      border: 1px solid var(--border);
      border-left-width: 5px;
      background: var(--panel);
    }
    .endpoint.get { border-left-color: var(--blue); }
    .endpoint.post { border-left-color: var(--green); }
    .endpoint.put { border-left-color: var(--purple); }
    .endpoint.delete { border-left-color: var(--red); }
    .endpoint summary {
      display: grid;
      grid-template-columns: 88px minmax(180px, 1fr) 2fr;
      gap: 12px;
      align-items: center;
      padding: 12px 16px;
      cursor: pointer;
      list-style: none;
    }
    .endpoint summary::-webkit-details-marker { display: none; }
    .method {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-width: 72px;
      height: 32px;
      border-radius: 3px;
      color: white;
      font-weight: 800;
      text-transform: uppercase;
      font-size: .82rem;
    }
    .get .method { background: var(--blue); }
    .post .method { background: var(--green); }
    .put .method { background: var(--purple); }
    .path {
      font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      font-weight: 800;
      overflow-wrap: anywhere;
    }
    .summary-text { color: var(--muted); }
    .details {
      border-top: 1px solid var(--border);
      background: var(--panel-alt);
      padding: 18px 20px 20px;
    }
    .try-grid {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 18px;
      margin-top: 18px;
    }
    .params {
      display: grid;
      gap: 12px;
    }
    .param-description {
      color: var(--muted);
      font-size: .85rem;
      margin-top: 4px;
    }
    button {
      border: 0;
      background: #393939;
      color: #fff;
      min-height: 40px;
      padding: 0 18px;
      font: inherit;
      font-weight: 700;
      cursor: pointer;
    }
    button:hover { background: #4c4c4c; }
    button.secondary {
      border: 1px solid var(--border);
      background: var(--panel);
      color: var(--text);
    }
    button.secondary:hover {
      background: var(--panel-alt);
    }
    .button-row {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      margin-top: 14px;
    }
    .result-stack {
      display: grid;
      gap: 14px;
    }
    .result-block {
      display: grid;
      gap: 6px;
    }
    .result-label {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 10px;
      color: var(--muted);
      font-size: .86rem;
      font-weight: 800;
    }
    .copy-button {
      min-height: 28px;
      padding: 0 10px;
      font-size: .78rem;
    }
    pre {
      min-height: 44px;
      max-height: 440px;
      overflow: auto;
      margin: 0;
      padding: 14px;
      border: 1px solid var(--border);
      background: #0f1419;
      color: #e6edf3;
      font-size: .88rem;
      white-space: pre-wrap;
      overflow-wrap: anywhere;
    }
    .response-body {
      min-height: 170px;
    }
    code {
      font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    }
    .empty {
      color: var(--muted);
      font-style: italic;
    }

    @media (max-width: 760px) {
      .toolbar, .try-grid, .endpoint summary {
        grid-template-columns: 1fr;
      }
      .endpoint summary {
        gap: 8px;
      }
    }
  </style>
</head>
<body>
  <header>
    <section class="hero">
      <div class="title-row">
        <h1>Image Roundup API</h1>
        <span class="badge secondary">1.0.0</span>
        <span class="badge">OAS 3.1</span>
      </div>
      <p class="subtitle">Explore image inventory, update detection, scan status, and runtime settings. Requests run against this Image Roundup deployment.</p>
      <p><a href="/api/v1/openapi.json">/api/v1/openapi.json</a> · <a href="/">Open app</a></p>
    </section>
    <section class="toolbar">
      <div class="field">
        <label for="server">Server</label>
        <select id="server">
          <option value="/api/v1">/api/v1</option>
        </select>
      </div>
      <div class="field">
        <label for="filter">Filter endpoints</label>
        <input id="filter" placeholder="summary, images, scan, settings">
      </div>
    </section>
  </header>
  <main id="docs">
    <p class="empty">Loading API docs...</p>
  </main>

  <script>
    const docsEl = document.getElementById('docs');
    const filterEl = document.getElementById('filter');
    const serverEl = document.getElementById('server');
    let spec = null;

    const methodOrder = ['get', 'post', 'put', 'patch', 'delete'];

    function escapeHtml(value) {
      return String(value ?? '')
        .replaceAll('&', '&amp;')
        .replaceAll('<', '&lt;')
        .replaceAll('>', '&gt;')
        .replaceAll('"', '&quot;')
        .replaceAll("'", '&#039;');
    }

    function resolveRef(ref) {
      if (!ref || !ref.startsWith('#/components/schemas/')) return null;
      return spec.components.schemas[ref.replace('#/components/schemas/', '')];
    }

    function schemaExample(schema) {
      if (!schema) return {};
      if (schema.$ref) schema = resolveRef(schema.$ref) || schema;
      if (schema.example !== undefined) return schema.example;
      if (schema.type === 'array') return [];
      const props = schema.properties || {};
      const result = {};
      Object.entries(props).forEach(([key, prop]) => {
        if (prop.$ref) prop = resolveRef(prop.$ref) || prop;
        if (prop.enum) result[key] = prop.enum[0];
        else if (prop.type === 'integer' || prop.type === 'number') result[key] = 0;
        else if (prop.type === 'boolean') result[key] = false;
        else if (prop.type === 'array') result[key] = [];
        else if (Array.isArray(prop.type) && prop.type.includes('null')) result[key] = null;
        else result[key] = '';
      });
      return result;
    }

    function requestExample(op) {
      const content = op.requestBody?.content?.['application/json'];
      const examples = content?.examples;
      if (examples) {
        const first = Object.values(examples)[0];
        return first.value ?? {};
      }
      return schemaExample(content?.schema);
    }

    function render() {
      const filter = filterEl.value.trim().toLowerCase();
      const grouped = new Map();

      Object.entries(spec.paths).forEach(([path, pathItem]) => {
        methodOrder.forEach((method) => {
          const op = pathItem[method];
          if (!op) return;
          const haystack = [method, path, op.summary, op.description, ...(op.tags || [])].join(' ').toLowerCase();
          if (filter && !haystack.includes(filter)) return;
          const tag = (op.tags && op.tags[0]) || 'default';
          if (!grouped.has(tag)) grouped.set(tag, []);
          grouped.get(tag).push({ method, path, op });
        });
      });

      if (grouped.size === 0) {
        docsEl.innerHTML = '<p class="empty">No endpoints match that filter.</p>';
        return;
      }

      docsEl.innerHTML = Array.from(grouped.entries()).map(([tag, endpoints]) => {
        return '<section class="tag-section"><div class="tag-title"><h2>' + escapeHtml(tag) + '</h2></div>' +
          endpoints.map(renderEndpoint).join('') +
          '</section>';
      }).join('');

      docsEl.querySelectorAll('[data-try]').forEach((button) => {
        button.addEventListener('click', () => runRequest(button.dataset.try));
      });
      docsEl.querySelectorAll('[data-copy]').forEach((button) => {
        button.addEventListener('click', () => copyBlock(button));
      });
      docsEl.querySelectorAll('[data-clear]').forEach((button) => {
        button.addEventListener('click', () => clearResult(button.dataset.clear));
      });
    }

    function renderEndpoint(endpoint) {
      const id = endpoint.method + ':' + endpoint.path;
      const params = endpoint.op.parameters || [];
      const body = endpoint.op.requestBody ? JSON.stringify(requestExample(endpoint.op), null, 2) : '';
      return '<details class="endpoint ' + endpoint.method + '">' +
        '<summary>' +
        '<span class="method">' + endpoint.method + '</span>' +
        '<span class="path">' + escapeHtml(endpoint.path) + '</span>' +
        '<span class="summary-text">' + escapeHtml(endpoint.op.summary || '') + '</span>' +
        '</summary>' +
        '<div class="details">' +
        '<p>' + escapeHtml(endpoint.op.description || endpoint.op.summary || '') + '</p>' +
        '<div class="try-grid">' +
        '<div>' +
        renderParams(id, params) +
        renderBody(id, body) +
        '<div class="button-row"><button data-try="' + escapeHtml(id) + '">Execute</button><button class="secondary" type="button" data-clear="' + escapeHtml(id) + '">Clear</button></div>' +
        '</div>' +
        '<div class="result-stack">' +
        resultBlock('Curl', 'curl-' + id, 'curl -X ' + endpoint.method.toUpperCase() + ' ' + shellQuote(location.origin + serverEl.value + endpoint.path) + ' \\\n  -H ' + shellQuote('accept: application/json')) +
        resultBlock('Request URL', 'url-' + id, location.origin + serverEl.value + endpoint.path) +
        '<div class="result-block"><div class="result-label"><span>Server response</span></div><pre id="response-' + escapeHtml(id) + '" class="response-body">Response will appear here.</pre></div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</details>';
    }

    function resultBlock(label, id, value) {
      return '<div class="result-block">' +
        '<div class="result-label"><span>' + escapeHtml(label) + '</span><button class="secondary copy-button" type="button" data-copy="' + escapeHtml(id) + '">Copy</button></div>' +
        '<pre id="' + escapeHtml(id) + '">' + escapeHtml(value) + '</pre>' +
        '</div>';
    }

    function renderParams(id, params) {
      if (!params.length) return '<p class="empty">No parameters.</p>';
      return '<div class="params">' + params.map((param) => {
        const label = param.name + (param.required ? ' *' : '');
        return '<div class="field">' +
          '<label for="' + escapeHtml(id + ':' + param.name) + '">' + escapeHtml(label) + '</label>' +
          '<input id="' + escapeHtml(id + ':' + param.name) + '" data-param="' + escapeHtml(param.name) + '" data-in="' + escapeHtml(param.in) + '" placeholder="' + escapeHtml(param.schema?.enum ? param.schema.enum.join(', ') : '') + '">' +
          '<div class="param-description">' + escapeHtml(param.description || '') + '</div>' +
          '</div>';
      }).join('') + '</div>';
    }

    function renderBody(id, body) {
      if (!body) return '';
      return '<div class="field" style="margin-top:14px">' +
        '<label for="body-' + escapeHtml(id) + '">JSON body</label>' +
        '<textarea id="body-' + escapeHtml(id) + '">' + escapeHtml(body) + '</textarea>' +
        '</div>';
    }

    async function runRequest(id) {
      const request = buildRequest(id);
      const url = request.url;
      const options = request.options;

      const responseEl = document.getElementById('response-' + id);
      document.getElementById('curl-' + id).textContent = request.curl;
      document.getElementById('url-' + id).textContent = request.absoluteUrl;
      responseEl.textContent = 'Loading ' + options.method + ' ' + url + ' ...';

      try {
        const response = await fetch(url, options);
        const text = await response.text();
        let formatted = text;
        try { formatted = JSON.stringify(JSON.parse(text), null, 2); } catch (_) {}
        responseEl.textContent = options.method + ' ' + url + '\nHTTP ' + response.status + '\n\n' + formatted;
      } catch (error) {
        responseEl.textContent = options.method + ' ' + url + '\n\n' + error;
      }
    }

    function buildRequest(id) {
      const [method, rawPath] = id.split(':');
      let path = rawPath;
      const details = Array.from(docsEl.querySelectorAll('[data-param]')).filter((el) => el.id.startsWith(id + ':'));
      const query = new URLSearchParams();

      details.forEach((input) => {
        const value = input.value.trim();
        if (!value) return;
        if (input.dataset.in === 'path') {
          path = path.replace('{' + input.dataset.param + '}', encodeURIComponent(value));
        } else {
          query.set(input.dataset.param, value);
        }
      });

      const queryString = query.toString();
      const url = serverEl.value + path + (queryString ? '?' + queryString : '');
      const absoluteUrl = new URL(url, location.origin).toString();
      const options = { method: method.toUpperCase(), headers: { Accept: 'application/json' } };
      const bodyEl = document.getElementById('body-' + id);
      if (bodyEl) {
        options.headers['Content-Type'] = 'application/json';
        options.body = bodyEl.value.trim() || '{}';
      }

      const curlParts = [
        'curl -X ' + shellQuote(options.method),
        '  ' + shellQuote(absoluteUrl),
        '  -H ' + shellQuote('accept: application/json')
      ];
      if (options.body !== undefined) {
        curlParts.push('  -H ' + shellQuote('Content-Type: application/json'));
        curlParts.push('  -d ' + shellQuote(options.body));
      }

      return {
        url,
        absoluteUrl,
        options,
        curl: curlParts.join(' \\\n')
      };
    }

    function shellQuote(value) {
      return "'" + String(value).replaceAll("'", "'\\''") + "'";
    }

    async function copyBlock(button) {
      const target = document.getElementById(button.dataset.copy);
      if (!target) return;
      const text = target.textContent;
      try {
        await navigator.clipboard.writeText(text);
        const oldText = button.textContent;
        button.textContent = 'Copied';
        setTimeout(() => { button.textContent = oldText; }, 1200);
      } catch (_) {
        window.prompt('Copy this value', text);
      }
    }

    function clearResult(id) {
      const request = buildRequest(id);
      document.getElementById('curl-' + id).textContent = request.curl;
      document.getElementById('url-' + id).textContent = request.absoluteUrl;
      document.getElementById('response-' + id).textContent = 'Response will appear here.';
    }

    fetch('/api/v1/openapi.json')
      .then((response) => response.json())
      .then((json) => {
        spec = json;
        filterEl.addEventListener('input', render);
        render();
      })
      .catch((error) => {
        docsEl.innerHTML = '<p class="empty">Failed to load OpenAPI document: ' + escapeHtml(error) + '</p>';
      });
  </script>
</body>
</html>`
