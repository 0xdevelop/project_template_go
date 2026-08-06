import "./styles.css";

type Protocol = "jsonrpc" | "mcp" | "websocket" | "grpc";

type APIMethod = {
  name: string;
  description?: string;
  inputSchema?: Record<string, unknown>;
};

type ProjectInfo = {
  name: string;
  version: string;
  bundleId: string;
  runMode: string;
};

type WebConfig = {
  jsonRpcEndpoint: string;
  mcpEndpoint: string;
  webSocketEndpoint: string;
  grpcEndpoint: string;
  methods: APIMethod[];
  project: ProjectInfo;
};

type ProxyResponse = {
  status: number;
  statusText: string;
  headers: Record<string, string[]>;
  body: string;
  durationMs: number;
  size: number;
  wasTruncated: boolean;
  errorKind?: "communication";
  protocolError?: boolean;
};

type HistoryItem = {
  method: string;
  protocol: Protocol;
  status: number;
  duration: number;
};

type CatalogItem = {
  id: string;
  domain: string;
  group: string;
  functionName: string;
  rpcMethod: string;
  description: string;
  inputSchema: Record<string, unknown>;
  kind: "JSON-RPC" | "MCP TOOL" | "WEBSOCKET" | "gRPC";
  toolName?: string;
};

type TripleSplitController = {
  setDefaultSizes: (first: number, middle: number) => void;
};

const byId = <T extends HTMLElement>(id: string): T => {
  const element = document.getElementById(id);
  if (!element) {
    throw new Error(`Missing element: ${id}`);
  }
  return element as T;
};

const endpointInput = byId<HTMLInputElement>("endpoint");
const transportMethodBadge = byId<HTMLElement>("transport-method");
const domainSelect = byId<HTMLSelectElement>("api-domain");
const groupSelect = byId<HTMLSelectElement>("api-group");
const functionSelect = byId<HTMLSelectElement>("api-function");
const requestIDInput = byId<HTMLInputElement>("request-id");
const requestBody = byId<HTMLTextAreaElement>("request-body");
const requestHeaders = byId<HTMLTextAreaElement>("request-headers");
const requestError = byId<HTMLParagraphElement>("request-error");
const testEmailInput = byId<HTMLInputElement>("test-email");
const testPasswordInput = byId<HTMLInputElement>("test-password");
const catalogCount = byId<HTMLElement>("catalog-count");
const codeMethodLabel = byId<HTMLElement>("code-method-label");
const codeLanguageTag = byId<HTMLElement>("code-language-tag");
const invocationCode = byId<HTMLPreElement>("invocation-code");
const headersCount = byId<HTMLElement>("headers-count");
const sendButton = byId<HTMLButtonElement>("send-request");
const responseMeta = byId<HTMLDivElement>("response-meta");
const responseEmpty = byId<HTMLDivElement>("response-empty");
const responseCode = byId<HTMLPreElement>("response-code");
const copyResponseButton = byId<HTMLButtonElement>("copy-response");
const historyList = byId<HTMLOListElement>("history-list");
const toast = byId<HTMLDivElement>("toast");
const rootWorkspace = byId<HTMLElement>("root-workspace");
const catalogColumns = byId<HTMLElement>("catalog-columns");
const tripleSplitControllers = new WeakMap<
  HTMLElement,
  TripleSplitController
>();

let protocol: Protocol = "jsonrpc";
let config: WebConfig = {
  jsonRpcEndpoint: "http://127.0.0.1:13001",
  mcpEndpoint: "http://127.0.0.1:13002",
  webSocketEndpoint: "ws://127.0.0.1:13004",
  grpcEndpoint: "grpc://127.0.0.1:13005",
  methods: [],
  project: {
    name: "project_template_go",
    version: "v0.0.2",
    bundleId: "com.project_template_go.project_template_go",
    runMode: "Debug",
  },
};
let catalogItems: CatalogItem[] = [];
let selectedItem: CatalogItem | null = null;
let lastResponse: ProxyResponse | null = null;
let responseView: "decoded" | "raw" | "headers" = "decoded";
let codeLanguage: "go" | "ts" = "go";
const history: HistoryItem[] = [];

const mcpProtocolVersion = "2026-07-28";
const protocolLabels: Record<Protocol, string> = {
  jsonrpc: "RPC",
  mcp: "MCP",
  websocket: "WS",
  grpc: "gRPC",
};

function generateRequestID(): string {
  const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789";
  while (true) {
    const randomValues = crypto.getRandomValues(new Uint8Array(4));
    const suffix = Array.from(
      randomValues,
      (value) => alphabet[value % alphabet.length],
    ).join("");
    if (/[a-z]/.test(suffix) && /\d/.test(suffix)) {
      return `${Date.now()}${suffix}`;
    }
  }
}

function unique(values: string[]): string[] {
  return [...new Set(values)];
}

function classifyName(
  name: string,
  fallbackDomain: string,
  fallbackGroup: string,
): Pick<CatalogItem, "domain" | "group" | "functionName"> {
  const parts = name.split(/[./:]+/).filter(Boolean);
  if (parts.length >= 3) {
    return {
      domain: parts[0],
      group: parts[1],
      functionName: parts.slice(2).join("."),
    };
  }
  if (parts.length === 2) {
    return {
      domain: parts[0],
      group: fallbackGroup,
      functionName: parts[1],
    };
  }
  return {
    domain: fallbackDomain,
    group: fallbackGroup,
    functionName: parts[0] || name,
  };
}

function buildCatalogItems(): CatalogItem[] {
  return config.methods
    .map((method) => {
      const classified = classifyName(method.name, "Core", "General");
      return {
        id: `api:${method.name}`,
        ...classified,
        rpcMethod: "tools/call",
        toolName: method.name,
        description: method.description || "已注册 API 方法",
        inputSchema: method.inputSchema ?? { type: "object" },
        kind: protocolCatalogKind(),
      };
    });
}

function protocolCatalogKind(): CatalogItem["kind"] {
  switch (protocol) {
    case "mcp":
      return "MCP TOOL";
    case "websocket":
      return "WEBSOCKET";
    case "grpc":
      return "gRPC";
    default:
      return "JSON-RPC";
  }
}

function replaceOptions(
  select: HTMLSelectElement,
  values: Array<{ value: string; label: string }>,
  preferred?: string,
): void {
  select.replaceChildren(
    ...values.map(({ value, label }) => {
      const option = document.createElement("option");
      option.value = value;
      option.textContent = label;
      return option;
    }),
  );
  const selectedValue =
    preferred && values.some((item) => item.value === preferred)
      ? preferred
      : values[0]?.value;
  if (selectedValue) {
    select.value = selectedValue;
  }
}

function rebuildCatalog(preferredMethod?: string): void {
  catalogItems = buildCatalogItems();
  catalogCount.textContent = `${catalogItems.length} functions`;
  syncCatalogSizing();

  const preferred =
    catalogItems.find(
      (item) =>
        item.rpcMethod === preferredMethod || item.toolName === preferredMethod,
    ) ?? catalogItems[0];
  replaceOptions(
    domainSelect,
    unique(catalogItems.map((item) => item.domain)).map((value) => ({
      value,
      label: value,
    })),
    preferred?.domain,
  );
  refreshGroups(preferred?.group, preferred?.id);
}

function refreshGroups(preferredGroup?: string, preferredID?: string): void {
  const domainItems = catalogItems.filter(
    (item) => item.domain === domainSelect.value,
  );
  replaceOptions(
    groupSelect,
    unique(domainItems.map((item) => item.group)).map((value) => ({
      value,
      label: value,
    })),
    preferredGroup,
  );
  refreshFunctions(preferredID);
}

function refreshFunctions(preferredID?: string): void {
  const groupItems = catalogItems.filter(
    (item) =>
      item.domain === domainSelect.value && item.group === groupSelect.value,
  );
  replaceOptions(
    functionSelect,
    groupItems.map((item) => ({
      value: item.id,
      label: item.functionName,
    })),
    preferredID,
  );
  applyCatalogSelection();
}

function applyCatalogSelection(): void {
  selectedItem =
    catalogItems.find((item) => item.id === functionSelect.value) ??
    catalogItems[0] ??
    null;
  if (!selectedItem) {
    codeMethodLabel.textContent = "no function";
    invocationCode.textContent = "";
    return;
  }

  applyPreset();
}

function paramsForSelection(): Record<string, unknown> {
  return {
    name: selectedItem?.toolName ?? "",
    arguments: paramsFromSchema(selectedItem?.inputSchema),
  };
}

function paramsFromSchema(
  schema: Record<string, unknown> | undefined,
): Record<string, unknown> {
  const value = valueFromSchema(schema);
  if (value !== null && !Array.isArray(value) && typeof value === "object") {
    const params = value as Record<string, unknown>;
    if (Object.prototype.hasOwnProperty.call(params, "email")) {
      params.email = testEmailInput.value;
    }
    if (Object.prototype.hasOwnProperty.call(params, "password")) {
      params.password = testPasswordInput.value;
    }
    return params;
  }
  return {};
}

function valueFromSchema(
  schema: Record<string, unknown> | undefined,
): unknown {
  if (!schema) {
    return {};
  }
  if ("default" in schema) {
    return schema.default;
  }
  if ("example" in schema) {
    return schema.example;
  }
  if (Array.isArray(schema.examples) && schema.examples.length > 0) {
    return schema.examples[0];
  }
  if ("const" in schema) {
    return schema.const;
  }
  if (Array.isArray(schema.enum) && schema.enum.length > 0) {
    return schema.enum[0];
  }

  const schemaType = Array.isArray(schema.type)
    ? schema.type.find((item) => item !== "null")
    : schema.type;
  if (
    schemaType === "object" ||
    (schemaType === undefined &&
      schema.properties !== null &&
      typeof schema.properties === "object")
  ) {
    const properties =
      schema.properties !== null && typeof schema.properties === "object"
        ? (schema.properties as Record<string, unknown>)
        : {};
    return Object.fromEntries(
      Object.entries(properties).map(([name, propertySchema]) => [
        name,
        valueFromSchema(
          propertySchema !== null && typeof propertySchema === "object"
            ? (propertySchema as Record<string, unknown>)
            : undefined,
        ),
      ]),
    );
  }
  if (schemaType === "array") {
    return [];
  }
  if (schemaType === "boolean") {
    return false;
  }
  if (schemaType === "integer" || schemaType === "number") {
    return 0;
  }
  if (schemaType === "string") {
    return "";
  }
  return null;
}

function defaultHeaders(): Record<string, string> {
  if (protocol === "websocket" || protocol === "grpc") {
    return {};
  }
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    Accept: "application/json",
  };
  if (protocol === "mcp") {
    headers.Accept = "application/json, text/event-stream";
    headers["Mcp-Protocol-Version"] = mcpProtocolVersion;
    headers["Mcp-Method"] = "tools/call";
    headers["Mcp-Name"] = selectedItem?.toolName ?? "";
  }
  return headers;
}

function makeBody(params: Record<string, unknown>): Record<string, unknown> {
  const method = selectedItem?.rpcMethod ?? "";
  if (protocol === "grpc") {
    return {
      requestId: requestIDInput.value,
      method,
      params,
    };
  }
  if (protocol === "mcp") {
    params = {
      ...params,
      _meta: {
        "io.modelcontextprotocol/protocolVersion": mcpProtocolVersion,
        "io.modelcontextprotocol/clientInfo": {
          name: config.project.name,
          version: config.project.version,
        },
        "io.modelcontextprotocol/clientCapabilities": {
          extensions: {},
        },
      },
    };
  }
  const body: Record<string, unknown> = {
    jsonrpc: "2.0",
    id: requestIDInput.value,
    method,
    params,
  };
  if (method === "notifications/initialized") {
    delete body.id;
  }
  return body;
}

function applyPreset(): void {
  if (!selectedItem) {
    return;
  }
  endpointInput.value = protocolEndpoint();
  transportMethodBadge.textContent =
    protocol === "websocket" ? "WS" : protocol === "grpc" ? "RPC" : "POST";
  requestBody.value = JSON.stringify(makeBody(paramsForSelection()), null, 2);
  requestHeaders.value = JSON.stringify(defaultHeaders(), null, 2);
  codeMethodLabel.textContent =
    selectedItem.toolName ?? selectedItem.rpcMethod;
  updateHeadersCount();
  renderInvocationCode();
  clearRequestError();
}

function protocolEndpoint(): string {
  switch (protocol) {
    case "mcp":
      return config.mcpEndpoint;
    case "websocket":
      return config.webSocketEndpoint;
    case "grpc":
      return config.grpcEndpoint;
    default:
      return config.jsonRpcEndpoint;
  }
}

function setProtocol(nextProtocol: Protocol): void {
  const selectedMethod =
    selectedItem?.toolName ?? selectedItem?.rpcMethod;
  protocol = nextProtocol;
  for (const tab of document.querySelectorAll<HTMLButtonElement>(".protocol-tab")) {
    const active = tab.dataset.protocol === protocol;
    tab.classList.toggle("is-active", active);
    tab.setAttribute("aria-pressed", String(active));
  }
  rebuildCatalog(selectedMethod);
}

function parseJSONObject(
  value: string,
  label: string,
): Record<string, unknown> {
  const parsed: unknown = JSON.parse(value);
  if (parsed === null || Array.isArray(parsed) || typeof parsed !== "object") {
    throw new Error(`${label} 必须是 JSON object`);
  }
  return parsed as Record<string, unknown>;
}

function syncTestAccountToRequest(): void {
  try {
    const body = parseJSONObject(requestBody.value, "Request body");
    const params = body.params;
    if (params === null || Array.isArray(params) || typeof params !== "object") {
      return;
    }
    const argumentsValue = (params as Record<string, unknown>).arguments;
    if (
      argumentsValue === null ||
      Array.isArray(argumentsValue) ||
      typeof argumentsValue !== "object"
    ) {
      return;
    }
    const argumentsObject = argumentsValue as Record<string, unknown>;
    if (Object.prototype.hasOwnProperty.call(argumentsObject, "email")) {
      argumentsObject.email = testEmailInput.value;
    }
    if (Object.prototype.hasOwnProperty.call(argumentsObject, "password")) {
      argumentsObject.password = testPasswordInput.value;
    }
    requestBody.value = JSON.stringify(body, null, 2);
    renderInvocationCode();
  } catch (error) {
    showRequestError(errorMessage(error));
  }
}

function formatJSON(textarea: HTMLTextAreaElement, label: string): void {
  try {
    textarea.value = JSON.stringify(JSON.parse(textarea.value), null, 2);
    clearRequestError();
  } catch (error) {
    showRequestError(`${label} JSON 无效：${errorMessage(error)}`);
  }
}

function updateHeadersCount(): void {
  try {
    const value = parseJSONObject(requestHeaders.value, "Headers");
    headersCount.textContent = `${Object.keys(value).length} 项`;
  } catch {
    headersCount.textContent = "格式错误";
  }
}

function showRequestError(message: string): void {
  requestError.textContent = message;
}

function clearRequestError(): void {
  requestError.textContent = "";
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

async function sendRequest(): Promise<void> {
  clearRequestError();
  let body: Record<string, unknown>;
  let headers: Record<string, unknown>;
  try {
    body = parseJSONObject(requestBody.value, "Request body");
    headers = parseJSONObject(requestHeaders.value, "Headers");
  } catch (error) {
    showRequestError(errorMessage(error));
    return;
  }
  if (protocol === "grpc") {
    requestIDInput.value = generateRequestID();
    body.requestId = requestIDInput.value;
    delete body.id;
  } else if (body.method === "notifications/initialized") {
    delete body.id;
  } else {
    requestIDInput.value = generateRequestID();
    body.id = requestIDInput.value;
  }
  requestBody.value = JSON.stringify(body, null, 2);
  renderInvocationCode();

  const headerStrings: Record<string, string> = {};
  for (const [name, value] of Object.entries(headers)) {
    headerStrings[name] = String(value);
  }

  setLoading(true);
  const startedAt = performance.now();
  try {
    const response = await fetch("/api/proxy", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        transport: protocol,
        url: endpointInput.value,
        method: "POST",
        headers: headerStrings,
        body: JSON.stringify(body),
      }),
    });
    const result = (await response.json()) as ProxyResponse | { error: string };
    if (!response.ok || "error" in result) {
      showCommunicationError(
        response.status,
        response.statusText,
        "error" in result ? result.error : `HTTP ${response.status}`,
        Math.round(performance.now() - startedAt),
      );
      showToast("通信失败");
      return;
    }
    lastResponse = result;
    showResponse(result);
    addHistory(result);
  } catch (error) {
    showCommunicationError(
      0,
      "Communication Error",
      errorMessage(error),
      Math.round(performance.now() - startedAt),
    );
    showToast("通信失败");
  } finally {
    setLoading(false);
  }
}

function setLoading(loading: boolean): void {
  sendButton.disabled = loading;
  sendButton.classList.toggle("is-loading", loading);
  const label = sendButton.querySelector("span");
  if (label) {
    label.textContent = loading ? "请求中…" : "请求";
  }
}

function showResponse(response: ProxyResponse): void {
  responseEmpty.classList.add("is-hidden");
  responseCode.classList.remove("is-hidden");
  copyResponseButton.disabled = false;

  const communicationError = response.errorKind === "communication";
  const protocolError = response.protocolError === true;
  const rpcError =
    !communicationError && !protocolError && hasRPCError(response.body);
  const successful =
    !communicationError &&
    !protocolError &&
    isSuccessfulStatus(response.status) &&
    !rpcError;
  const statusLabel = protocolError
    ? response.statusText
    : communicationError
      ? `${response.status || "—"} COMM`
      : `${response.status}${rpcError ? " RPC" : ""}`;
  responseMeta.innerHTML = `
    <span class="status-pill ${successful ? "is-success" : "is-error"}">
      ${escapeHTML(statusLabel)}
    </span>
    <span>${response.durationMs} ms</span>
    <span>${formatBytes(response.size)}${response.wasTruncated ? " · truncated" : ""}</span>
  `;
  renderResponseView();
}

function showCommunicationError(
  status: number,
  statusText: string,
  message: string,
  durationMs: number,
): void {
  const body = JSON.stringify({ error: message }, null, 2);
  const response: ProxyResponse = {
    status,
    statusText,
    headers: {},
    body,
    durationMs,
    size: new TextEncoder().encode(body).length,
    wasTruncated: false,
    errorKind: "communication",
  };
  responseView = "decoded";
  for (const tab of document.querySelectorAll<HTMLButtonElement>(
    ".response-tab",
  )) {
    const active = tab.dataset.responseTab === responseView;
    tab.classList.toggle("is-active", active);
    tab.setAttribute("aria-selected", String(active));
  }
  lastResponse = response;
  showResponse(response);
  addHistory(response);
}

function isSuccessfulStatus(status: number): boolean {
  return (
    (status >= 200 && status < 300) ||
    (protocol === "websocket" && status === 101)
  );
}

function hasRPCError(body: string): boolean {
  try {
    const parsed = JSON.parse(body) as { error?: unknown };
    return parsed !== null && typeof parsed === "object" && parsed.error != null;
  } catch {
    return false;
  }
}

function renderResponseView(): void {
  if (!lastResponse) {
    return;
  }
  if (responseView === "headers") {
    responseCode.textContent = JSON.stringify(lastResponse.headers, null, 2);
    return;
  }
  try {
    const body = JSON.parse(lastResponse.body) as unknown;
    responseCode.textContent = JSON.stringify(
      responseView === "decoded" ? decodeNestedJSON(body) : body,
      null,
      2,
    );
  } catch {
    responseCode.textContent = lastResponse.body || "(empty response body)";
  }
}

function decodeNestedJSON(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(decodeNestedJSON);
  }
  if (value !== null && typeof value === "object") {
    return Object.fromEntries(
      Object.entries(value).map(([key, child]) => [key, decodeNestedJSON(child)]),
    );
  }
  if (typeof value !== "string") {
    return value;
  }

  const candidate = value.trim();
  const isJSONObject = candidate.startsWith("{") && candidate.endsWith("}");
  const isJSONArray = candidate.startsWith("[") && candidate.endsWith("]");
  if (!isJSONObject && !isJSONArray) {
    return value;
  }
  try {
    return decodeNestedJSON(JSON.parse(candidate) as unknown);
  } catch {
    return value;
  }
}

function formatBytes(size: number): string {
  if (size < 1024) {
    return `${size} B`;
  }
  return `${(size / 1024).toFixed(1)} KB`;
}

function addHistory(response: ProxyResponse): void {
  history.unshift({
    method: selectedItem?.toolName ?? selectedItem?.rpcMethod ?? "unknown",
    protocol,
    status: response.status,
    duration: response.durationMs,
  });
  history.splice(6);
  renderHistory();
}

function renderHistory(): void {
  if (history.length === 0) {
    const item = document.createElement("li");
    item.className = "history-empty";
    item.textContent = "暂无请求记录";
    historyList.replaceChildren(item);
    return;
  }
  historyList.replaceChildren(
    ...history.map((entry) => {
      const item = document.createElement("li");
      item.innerHTML = `
        <span class="history-protocol">${protocolLabels[entry.protocol]}</span>
        <strong>${escapeHTML(entry.method)}</strong>
        <span class="${entry.status >= 200 && entry.status < 300 || entry.protocol === "websocket" && entry.status === 101 ? "history-ok" : "history-failed"}">${entry.status}</span>
        <small>${entry.duration} ms</small>
      `;
      return item;
    }),
  );
}

function escapeHTML(value: string): string {
  return value.replace(
    /[&<>"']/g,
    (character) =>
      ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&#039;",
      })[character] ?? character,
  );
}

function buildCommand(): string {
  let headers: Record<string, unknown> = {};
  try {
    headers = parseJSONObject(requestHeaders.value, "Headers");
  } catch {
    // Copy the usable portion even while the header editor is invalid.
  }
  const headerArgs = Object.entries(headers)
    .map(([name, value]) => `-H ${shellQuote(`${name}: ${String(value)}`)}`)
    .join(" \\\n  ");
  if (protocol === "websocket") {
    return [
      `printf '%s' ${shellQuote(requestBody.value)}`,
      `websocat -1 ${shellQuote(endpointInput.value)}`,
    ].join(" | ");
  }
  if (protocol === "grpc") {
    const endpoint = endpointInput.value.replace(/^grpc:\/\//, "");
    return [
      "grpcurl -plaintext",
      "-import-path api/api_grpc/proto",
      "-proto api.proto",
      `-d ${shellQuote(requestBody.value)}`,
      shellQuote(endpoint),
      "project_template_go.api.v1.APIService/Call",
    ].join(" ");
  }
  const parts = [
    `curl -X POST ${shellQuote(endpointInput.value)}`,
    headerArgs,
    `--data-raw ${shellQuote(requestBody.value)}`,
  ].filter(Boolean);
  return parts.join(" \\\n  ");
}

function shellQuote(value: string): string {
  return `'${value.replaceAll("'", `'\\''`)}'`;
}

function invocationFunctionName(): string {
  const source = selectedItem?.toolName ?? selectedItem?.functionName ?? "api";
  const words = source.split(/[^a-zA-Z0-9]+/).filter(Boolean);
  const pascal = words
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join("");
  return /^\d/.test(pascal) ? `Method${pascal}` : pascal || "API";
}

function indentCode(value: string, spaces: number): string {
  const indent = " ".repeat(spaces);
  return value
    .split("\n")
    .map((line) => `${indent}${line}`)
    .join("\n");
}

function goString(value: string): string {
  return JSON.stringify(value);
}

function stringHeaders(
  headers: Record<string, unknown>,
): Record<string, string> {
  return Object.fromEntries(
    Object.entries(headers).map(([name, value]) => [name, String(value)]),
  );
}

function buildGoHTTPInvocation(
  functionName: string,
  body: Record<string, unknown>,
  headers: Record<string, string>,
): string {
  const headerLines = Object.entries(headers)
    .map(
      ([name, value]) =>
        `\trequest.Header.Set(${goString(name)}, ${goString(value)})`,
    )
    .join("\n");
  return `// imports: bytes, context, fmt, io, net/http
func Call${functionName}(ctx context.Context) ([]byte, error) {
\tbody := []byte(${goString(JSON.stringify(body))})
\trequest, err := http.NewRequestWithContext(ctx, http.MethodPost, ${goString(endpointInput.value)}, bytes.NewReader(body))
\tif err != nil {
\t\treturn nil, err
\t}
${headerLines}

\tresponse, err := http.DefaultClient.Do(request)
\tif err != nil {
\t\treturn nil, err
\t}
\tdefer response.Body.Close()

\tresponseBody, err := io.ReadAll(response.Body)
\tif err != nil {
\t\treturn nil, err
\t}
\tif response.StatusCode < 200 || response.StatusCode >= 300 {
\t\treturn nil, fmt.Errorf("request failed: %s: %s", response.Status, responseBody)
\t}
\treturn responseBody, nil
}`;
}

function buildGoWebSocketInvocation(
  functionName: string,
  body: Record<string, unknown>,
): string {
  return `// import: context, github.com/coder/websocket
func Call${functionName}(ctx context.Context) ([]byte, error) {
\tconnection, _, err := websocket.Dial(ctx, ${goString(endpointInput.value)}, nil)
\tif err != nil {
\t\treturn nil, err
\t}
\tdefer connection.CloseNow()

\tif err := connection.Write(ctx, websocket.MessageText, []byte(${goString(JSON.stringify(body))})); err != nil {
\t\treturn nil, err
\t}
\t_, response, err := connection.Read(ctx)
\tif err != nil {
\t\treturn nil, err
\t}
\treturn response, connection.Close(websocket.StatusNormalClosure, "")
}`;
}

function buildGoGRPCInvocation(
  functionName: string,
  body: Record<string, unknown>,
): string {
  const params =
    body.params !== null &&
    !Array.isArray(body.params) &&
    typeof body.params === "object"
      ? (body.params as Record<string, unknown>)
      : {};
  const endpoint = endpointInput.value.replace(/^grpc:\/\//, "");
  const requestID = String(body.requestId ?? requestIDInput.value);
  const method = String(body.method ?? selectedItem?.rpcMethod ?? "tools/call");
  return `// imports: context, encoding/json, api_grpc_protobuf, grpc, insecure, structpb
func Call${functionName}(ctx context.Context) (*api_grpc_protobuf.CallResponse, error) {
\tconnection, err := grpc.NewClient(${goString(endpoint)}, grpc.WithTransportCredentials(insecure.NewCredentials()))
\tif err != nil {
\t\treturn nil, err
\t}
\tdefer connection.Close()

\tvar params map[string]any
\tif err := json.Unmarshal([]byte(${goString(JSON.stringify(params))}), &params); err != nil {
\t\treturn nil, err
\t}
\tparamsValue, err := structpb.NewStruct(params)
\tif err != nil {
\t\treturn nil, err
\t}

\treturn api_grpc_protobuf.NewAPIServiceClient(connection).Call(ctx, &api_grpc_protobuf.CallRequest{
\t\tRequestId: ${goString(requestID)},
\t\tMethod:    ${goString(method)},
\t\tParams:    paramsValue,
\t})
}`;
}

function buildGoInvocation(
  body: Record<string, unknown>,
  headers: Record<string, string>,
): string {
  const functionName = invocationFunctionName();
  switch (protocol) {
    case "grpc":
      return buildGoGRPCInvocation(functionName, body);
    case "websocket":
      return buildGoWebSocketInvocation(functionName, body);
    default:
      return buildGoHTTPInvocation(functionName, body, headers);
  }
}

function buildTSHTTPInvocation(
  functionName: string,
  body: Record<string, unknown>,
  headers: Record<string, string>,
): string {
  return `export async function call${functionName}(): Promise<unknown> {
  const response = await fetch(${JSON.stringify(endpointInput.value)}, {
    method: "POST",
    headers: ${indentCode(JSON.stringify(headers, null, 2), 4).trimStart()},
    body: JSON.stringify(${indentCode(JSON.stringify(body, null, 2), 4).trimStart()}),
  });
  if (!response.ok) {
    throw new Error(\`request failed: \${response.status} \${await response.text()}\`);
  }
  return response.json();
}`;
}

function buildTSWebSocketInvocation(
  functionName: string,
  body: Record<string, unknown>,
): string {
  return `export function call${functionName}(): Promise<unknown> {
  return new Promise((resolve, reject) => {
    const socket = new WebSocket(${JSON.stringify(endpointInput.value)});
    socket.addEventListener("open", () => socket.send(${JSON.stringify(JSON.stringify(body))}));
    socket.addEventListener("error", () => reject(new Error("WebSocket request failed")));
    socket.addEventListener("message", (event) => {
      socket.close();
      try {
        resolve(JSON.parse(String(event.data)));
      } catch {
        resolve(event.data);
      }
    });
  });
}`;
}

function buildTSGRPCInvocation(
  functionName: string,
  body: Record<string, unknown>,
): string {
  const endpoint = endpointInput.value.replace(/^grpc:\/\//, "");
  return `// npm i @grpc/grpc-js @grpc/proto-loader
import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";

export async function call${functionName}(): Promise<unknown> {
  const definition = protoLoader.loadSync("api/api_grpc/proto/api.proto", {
    keepCase: false,
    longs: String,
    enums: String,
    defaults: true,
    oneofs: true,
  });
  const grpcObject = grpc.loadPackageDefinition(definition) as any;
  const Client = grpcObject.project_template_go.api.v1.APIService;
  const client = new Client(${JSON.stringify(endpoint)}, grpc.credentials.createInsecure());
  const request = ${indentCode(JSON.stringify(body, null, 2), 2).trimStart()};

  return new Promise((resolve, reject) => {
    client.call(request, (error: Error | null, response: unknown) => {
      client.close();
      if (error) reject(error);
      else resolve(response);
    });
  });
}`;
}

function buildTSInvocation(
  body: Record<string, unknown>,
  headers: Record<string, string>,
): string {
  const functionName = invocationFunctionName();
  switch (protocol) {
    case "grpc":
      return buildTSGRPCInvocation(functionName, body);
    case "websocket":
      return buildTSWebSocketInvocation(functionName, body);
    default:
      return buildTSHTTPInvocation(functionName, body, headers);
  }
}

function renderInvocationCode(): void {
  codeLanguageTag.textContent = codeLanguage.toUpperCase();
  try {
    const body = parseJSONObject(requestBody.value, "Request body");
    const headers = stringHeaders(
      parseJSONObject(requestHeaders.value, "Headers"),
    );
    invocationCode.textContent =
      codeLanguage === "go"
        ? buildGoInvocation(body, headers)
        : buildTSInvocation(body, headers);
  } catch (error) {
    invocationCode.textContent = `// ${errorMessage(error)}`;
  }
}

async function copyText(value: string, message: string): Promise<void> {
  await navigator.clipboard.writeText(value);
  showToast(message);
}

let toastTimer = 0;
function showToast(message: string): void {
  window.clearTimeout(toastTimer);
  toast.textContent = message;
  toast.classList.add("is-visible");
  toastTimer = window.setTimeout(() => toast.classList.remove("is-visible"), 1800);
}

function endpointPort(endpoint: string): string {
  try {
    return `:${new URL(endpoint).port}`;
  } catch {
    return "—";
  }
}

function applyProjectInfo(): void {
  byId<HTMLElement>("project-name").textContent = config.project.name;
  byId<HTMLElement>("project-version").textContent = config.project.version;
  byId<HTMLElement>("project-bundle").textContent = config.project.bundleId;
  byId<HTMLElement>("project-mode").textContent = config.project.runMode;
  byId<HTMLElement>("jsonrpc-port").textContent = endpointPort(
    config.jsonRpcEndpoint,
  );
  byId<HTMLElement>("mcp-port").textContent = endpointPort(config.mcpEndpoint);
  byId<HTMLElement>("websocket-port").textContent = endpointPort(
    config.webSocketEndpoint,
  );
  byId<HTMLElement>("grpc-port").textContent = endpointPort(
    config.grpcEndpoint,
  );
  document.title = `${config.project.name} · API Lab`;
}

async function loadConfig(): Promise<void> {
  try {
    const response = await fetch("/api/config", { cache: "no-store" });
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`);
    }
    const loadedConfig = (await response.json()) as Partial<WebConfig>;
    config = {
      ...config,
      ...loadedConfig,
      methods: loadedConfig.methods ?? config.methods,
      project: {
        ...config.project,
        ...loadedConfig.project,
      },
    };
    applyProjectInfo();
    rebuildCatalog();
  } catch (error) {
    showRequestError(`加载 API 配置失败：${errorMessage(error)}`);
    applyProjectInfo();
    rebuildCatalog();
  }
}

function directChild<T extends HTMLElement>(
  parent: HTMLElement,
  selector: string,
): T | null {
  return parent.querySelector<T>(`:scope > ${selector}`);
}

function clamp(value: number, minimum: number, maximum: number): number {
  return Math.min(Math.max(minimum, value), Math.max(minimum, maximum));
}

function catalogWidthFor(
  select: HTMLSelectElement,
  labels: string[],
  heading: string,
  minimum: number,
): number {
  const style = window.getComputedStyle(select);
  const context = document.createElement("canvas").getContext("2d");
  let longest = heading.length * 10;
  if (context) {
    context.font = style.font;
    longest = Math.max(
      context.measureText(heading).width,
      ...labels.map((label) => context.measureText(label).width),
    );
  } else {
    longest = Math.max(
      heading.length,
      ...labels.map((label) => label.length),
    ) * 10;
  }

  // 34px is the select arrow/padding; 8px is the compact column inset.
  return Math.ceil(Math.max(minimum, longest + 42));
}

function syncCatalogSizing(): void {
  window.requestAnimationFrame(() => {
    const domainWidth = catalogWidthFor(
      domainSelect,
      unique(catalogItems.map((item) => item.domain)),
      "功能域",
      92,
    );
    const groupWidth = catalogWidthFor(
      groupSelect,
      unique(catalogItems.map((item) => item.group)),
      "功能组",
      92,
    );
    const methodWidth = catalogWidthFor(
      functionSelect,
      catalogItems.map((item) => item.functionName),
      "方法",
      140,
    );

    const rootController = tripleSplitControllers.get(rootWorkspace);
    if (!rootController) {
      return;
    }
    const rootRect = rootWorkspace.getBoundingClientRect();
    const rootSeparators = rootWorkspace.querySelectorAll<HTMLElement>(
      ":scope > [data-triple-separator]",
    );
    const separatorWidth = Array.from(rootSeparators).reduce(
      (total, separator) => total + separator.getBoundingClientRect().width,
      0,
    );
    const usableWidth = Math.max(0, rootRect.width - separatorWidth);
    const minimumRequest = Number(rootWorkspace.dataset.minMiddle ?? 390);
    const minimumResponse = Number(rootWorkspace.dataset.minThird ?? 360);
    const catalogWidth = domainWidth + groupWidth + methodWidth + 14;
    const first = clamp(
      catalogWidth,
      Number(rootWorkspace.dataset.minFirst ?? 330),
      usableWidth - minimumRequest - minimumResponse,
    );
    const remaining = Math.max(0, usableWidth - first);
    const ratio = Number(rootWorkspace.dataset.middleRatio ?? 0.52);
    const middle = clamp(
      remaining * ratio,
      minimumRequest,
      remaining - minimumResponse,
    );
    rootController.setDefaultSizes(first, middle);
    tripleSplitControllers
      .get(catalogColumns)
      ?.setDefaultSizes(domainWidth, groupWidth);
  });
}

function initializeTripleSplitViews(): void {
  for (const view of document.querySelectorAll<HTMLElement>(
    "[data-triple-split]",
  )) {
    const firstPane = directChild<HTMLElement>(
      view,
      '[data-triple-pane="first"]',
    );
    const middlePane = directChild<HTMLElement>(
      view,
      '[data-triple-pane="middle"]',
    );
    const firstSeparator = directChild<HTMLElement>(
      view,
      '[data-triple-separator="first"]',
    );
    const secondSeparator = directChild<HTMLElement>(
      view,
      '[data-triple-separator="second"]',
    );
    if (!firstPane || !middlePane || !firstSeparator || !secondSeparator) {
      continue;
    }

    const minFirst = Number(view.dataset.minFirst ?? 120);
    const minMiddle = Number(view.dataset.minMiddle ?? 120);
    const minThird = Number(view.dataset.minThird ?? 120);
    let defaultFirst = firstPane.getBoundingClientRect().width;
    let defaultMiddle = middlePane.getBoundingClientRect().width;

    const usableWidth = (): number => {
      const separatorWidth =
        firstSeparator.getBoundingClientRect().width +
        secondSeparator.getBoundingClientRect().width;
      return Math.max(0, view.getBoundingClientRect().width - separatorWidth);
    };

    const setSizes = (
      requestedFirst: number,
      requestedMiddle: number,
    ): { first: number; middle: number } => {
      const usable = usableWidth();
      const first = clamp(
        requestedFirst,
        minFirst,
        usable - minMiddle - minThird,
      );
      const middle = clamp(
        requestedMiddle,
        minMiddle,
        usable - first - minThird,
      );
      view.style.setProperty("--triple-first", `${first}px`);
      view.style.setProperty("--triple-middle", `${middle}px`);
      firstSeparator.setAttribute(
        "aria-valuenow",
        String(Math.round((first / Math.max(usable, 1)) * 100)),
      );
      secondSeparator.setAttribute(
        "aria-valuenow",
        String(Math.round(((first + middle) / Math.max(usable, 1)) * 100)),
      );
      return { first, middle };
    };

    const currentSizes = (): { first: number; middle: number } => ({
      first: firstPane.getBoundingClientRect().width,
      middle: middlePane.getBoundingClientRect().width,
    });

    const bindSeparator = (
      separator: HTMLElement,
      boundary: "first" | "second",
    ): void => {
      let startPointer = 0;
      let startFirst = 0;
      let startMiddle = 0;

      separator.addEventListener("pointerdown", (event) => {
        const current = currentSizes();
        startPointer = event.clientX;
        startFirst = current.first;
        startMiddle = current.middle;
        separator.setPointerCapture(event.pointerId);
        separator.classList.add("is-dragging");
      });
      separator.addEventListener("pointermove", (event) => {
        if (!separator.hasPointerCapture(event.pointerId)) {
          return;
        }
        const delta = event.clientX - startPointer;
        if (boundary === "first") {
          const adjacentWidth = startFirst + startMiddle;
          const first = clamp(
            startFirst + delta,
            minFirst,
            adjacentWidth - minMiddle,
          );
          setSizes(first, adjacentWidth - first);
          return;
        }
        setSizes(startFirst, startMiddle + delta);
      });
      const finishDrag = (event: PointerEvent): void => {
        if (separator.hasPointerCapture(event.pointerId)) {
          separator.releasePointerCapture(event.pointerId);
        }
        separator.classList.remove("is-dragging");
      };
      separator.addEventListener("pointerup", finishDrag);
      separator.addEventListener("pointercancel", finishDrag);
      separator.addEventListener("dblclick", () =>
        setSizes(defaultFirst, defaultMiddle),
      );
      separator.addEventListener("keydown", (event) => {
        if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") {
          return;
        }
        event.preventDefault();
        const delta = event.key === "ArrowRight" ? 12 : -12;
        const current = currentSizes();
        if (boundary === "first") {
          const adjacentWidth = current.first + current.middle;
          const first = clamp(
            current.first + delta,
            minFirst,
            adjacentWidth - minMiddle,
          );
          setSizes(first, adjacentWidth - first);
          return;
        }
        setSizes(current.first, current.middle + delta);
      });
    };

    bindSeparator(firstSeparator, "first");
    bindSeparator(secondSeparator, "second");
    tripleSplitControllers.set(view, {
      setDefaultSizes: (first, middle) => {
        const sizes = setSizes(first, middle);
        defaultFirst = sizes.first;
        defaultMiddle = sizes.middle;
      },
    });

    window.requestAnimationFrame(() => {
      const initial = currentSizes();
      defaultFirst = initial.first;
      defaultMiddle = initial.middle;
      setSizes(defaultFirst, defaultMiddle);
    });
  }
}

function initializeSplitViews(): void {
  for (const view of document.querySelectorAll<HTMLElement>("[data-split]")) {
    const firstPane = directChild<HTMLElement>(view, '[data-pane="first"]');
    const separator = directChild<HTMLElement>(view, '[role="separator"]');
    if (!firstPane || !separator) {
      continue;
    }

    const horizontal = view.dataset.orientation === "horizontal";
    const minFirst = Number(view.dataset.minFirst ?? 120);
    const minSecond = Number(view.dataset.minSecond ?? 120);
    let defaultSize = horizontal
      ? firstPane.getBoundingClientRect().width
      : firstPane.getBoundingClientRect().height;
    let startPointer = 0;
    let startSize = 0;

    const setSize = (requestedSize: number): void => {
      const rect = view.getBoundingClientRect();
      const separatorSize = horizontal
        ? separator.getBoundingClientRect().width
        : separator.getBoundingClientRect().height;
      const total = horizontal ? rect.width : rect.height;
      const maximum = Math.max(minFirst, total - separatorSize - minSecond);
      const size = Math.min(maximum, Math.max(minFirst, requestedSize));
      view.style.setProperty("--split-first", `${size}px`);
      separator.setAttribute(
        "aria-valuenow",
        String(Math.round((size / Math.max(total, 1)) * 100)),
      );
    };

    separator.addEventListener("pointerdown", (event) => {
      if (window.matchMedia("(max-width: 760px)").matches) {
        return;
      }
      startPointer = horizontal ? event.clientX : event.clientY;
      startSize = horizontal
        ? firstPane.getBoundingClientRect().width
        : firstPane.getBoundingClientRect().height;
      separator.setPointerCapture(event.pointerId);
      separator.classList.add("is-dragging");
    });
    separator.addEventListener("pointermove", (event) => {
      if (!separator.hasPointerCapture(event.pointerId)) {
        return;
      }
      const pointer = horizontal ? event.clientX : event.clientY;
      setSize(startSize + pointer - startPointer);
    });
    const finishDrag = (event: PointerEvent): void => {
      if (separator.hasPointerCapture(event.pointerId)) {
        separator.releasePointerCapture(event.pointerId);
      }
      separator.classList.remove("is-dragging");
    };
    separator.addEventListener("pointerup", finishDrag);
    separator.addEventListener("pointercancel", finishDrag);
    separator.addEventListener("dblclick", () => setSize(defaultSize));
    separator.addEventListener("keydown", (event) => {
      const currentSize = horizontal
        ? firstPane.getBoundingClientRect().width
        : firstPane.getBoundingClientRect().height;
      const decrease =
        (horizontal && event.key === "ArrowLeft") ||
        (!horizontal && event.key === "ArrowUp");
      const increase =
        (horizontal && event.key === "ArrowRight") ||
        (!horizontal && event.key === "ArrowDown");
      if (!decrease && !increase) {
        return;
      }
      event.preventDefault();
      setSize(currentSize + (increase ? 12 : -12));
    });

    requestAnimationFrame(() => {
      defaultSize = horizontal
        ? firstPane.getBoundingClientRect().width
        : firstPane.getBoundingClientRect().height;
      setSize(defaultSize);
    });
  }
}

for (const tab of document.querySelectorAll<HTMLButtonElement>(".protocol-tab")) {
  tab.addEventListener("click", () => {
    setProtocol(tab.dataset.protocol as Protocol);
  });
}

domainSelect.addEventListener("change", () => refreshGroups());
groupSelect.addEventListener("change", () => refreshFunctions());
functionSelect.addEventListener("change", applyCatalogSelection);
requestBody.addEventListener("input", renderInvocationCode);
requestHeaders.addEventListener("input", () => {
  updateHeadersCount();
  renderInvocationCode();
});
endpointInput.addEventListener("input", renderInvocationCode);
testEmailInput.addEventListener("input", syncTestAccountToRequest);
testPasswordInput.addEventListener("input", syncTestAccountToRequest);
byId<HTMLButtonElement>("format-request").addEventListener("click", () => {
  formatJSON(requestBody, "Request body");
  renderInvocationCode();
});
byId<HTMLButtonElement>("reset-request").addEventListener("click", applyPreset);
sendButton.addEventListener("click", () => void sendRequest());
byId<HTMLButtonElement>("copy-command").addEventListener("click", () => {
  void copyText(buildCommand(), "调用命令已复制");
});
byId<HTMLButtonElement>("copy-invocation").addEventListener("click", () => {
  void copyText(invocationCode.textContent ?? "", "调用代码已复制");
});
for (const tab of document.querySelectorAll<HTMLButtonElement>(
  ".code-language-tab",
)) {
  tab.addEventListener("click", () => {
    codeLanguage = tab.dataset.codeLanguage as "go" | "ts";
    for (const item of document.querySelectorAll<HTMLButtonElement>(
      ".code-language-tab",
    )) {
      const active = item === tab;
      item.classList.toggle("is-active", active);
      item.setAttribute("aria-selected", String(active));
    }
    renderInvocationCode();
  });
}
copyResponseButton.addEventListener("click", () => {
  if (lastResponse) {
    void copyText(responseCode.textContent ?? "", "响应已复制");
  }
});
byId<HTMLButtonElement>("clear-history").addEventListener("click", () => {
  history.length = 0;
  renderHistory();
});

for (const tab of document.querySelectorAll<HTMLButtonElement>(".response-tab")) {
  tab.addEventListener("click", () => {
    responseView = tab.dataset.responseTab as "decoded" | "raw" | "headers";
    for (const item of document.querySelectorAll<HTMLButtonElement>(".response-tab")) {
      const active = item === tab;
      item.classList.toggle("is-active", active);
      item.setAttribute("aria-selected", String(active));
    }
    renderResponseView();
  });
}

document.addEventListener("keydown", (event) => {
  if ((event.metaKey || event.ctrlKey) && event.key === "Enter") {
    event.preventDefault();
    void sendRequest();
  }
});

initializeTripleSplitViews();
initializeSplitViews();
renderHistory();
requestIDInput.value = generateRequestID();
void loadConfig();
