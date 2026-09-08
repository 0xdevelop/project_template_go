import "./styles.css";

// 开发期 API 控制台（各 Go 服务共用同一份源码）。目录全部来自 /api/config 运行期投影：
// 本服务方法表 + 可选的聚合后端工具 + 可选的 REST 路由；来源标签、端点、gRPC 服务名与文档页签都由
// Go 侧注入，页面不写死任何方法、任何项目名。
// 请求编辑器是一份 JSON：{ headers, body }（REST 再带 method / path / query），发送前拆开交 proxy。

type Protocol = "jsonrpc" | "mcp" | "websocket" | "grpc" | "rest";

type APIMethod = {
  name: string;
  description?: string;
  inputSchema?: Record<string, unknown>;
  source: string;
  protected: boolean;
  acceptsOrgId: boolean;
  public: boolean;
};

type RESTParameter = {
  name: string;
  in: string;
  required: boolean;
  description?: string;
  schema?: Record<string, unknown>;
};

type RESTOperation = {
  method: string;
  path: string;
  operationId: string;
  summary?: string;
  description?: string;
  tags?: string[];
  origin?: string;
  backendMethod?: string;
  protected: boolean;
  parameters?: RESTParameter[];
  requestSchema?: Record<string, unknown>;
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
  /** 为空 = 本服务没有 REST 面，页签与端口徽标隐藏。 */
  restEndpoint: string;
  /** 为空 = 不显示健康端口徽标。 */
  healthEndpoint: string;
  /** gRPC 服务全名（如 <pkg>.APIService），调用代码片段用；由 Go 侧从 protobuf 描述符取。 */
  grpcService: string;
  /** grpcurl 的 -import-path，即本仓 proto 目录。 */
  grpcProtoImportPath: string;
  /** 来源标识 -> 导航里的功能域名；缺项按标识原样显示，空标识显示「本服务」。 */
  sourceLabels: Record<string, string>;
  methods: APIMethod[];
  restOperations: RESTOperation[];
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
  kind: "JSON-RPC" | "MCP TOOL" | "WEBSOCKET" | "gRPC" | "REST";
  toolName?: string;
  source: string;
  protected: boolean;
  acceptsOrgId: boolean;
  rest?: RESTOperation;
};

// RequestEnvelope 是编辑器里那一份 JSON 的形状：headers 与 body 合一；REST 再带 method / path / query。
type RequestEnvelope = {
  method?: string;
  path?: string;
  query?: Record<string, unknown>;
  headers: Record<string, unknown>;
  body: Record<string, unknown>;
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
const requestShapeHint = byId<HTMLElement>("request-shape-hint");
const requestError = byId<HTMLParagraphElement>("request-error");
const apiDescription = byId<HTMLDivElement>("api-description");
const apiDescriptionSource = byId<HTMLElement>("api-description-source");
const testUserNameInput = byId<HTMLInputElement>("test-user-name");
const testEmailInput = byId<HTMLInputElement>("test-email");
const testPhoneInput = byId<HTMLInputElement>("test-phone");
const testPasswordInput = byId<HTMLInputElement>("test-password");
const testAccessTokenInput = byId<HTMLInputElement>("test-access-token");
const testOrgIDInput = byId<HTMLInputElement>("test-org-id");
const catalogCount = byId<HTMLElement>("catalog-count");
const codeMethodLabel = byId<HTMLElement>("code-method-label");
const codeLanguageTag = byId<HTMLElement>("code-language-tag");
const invocationCode = byId<HTMLPreElement>("invocation-code");
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

let protocol: Protocol = "mcp";
let config: WebConfig = {
  jsonRpcEndpoint: "",
  mcpEndpoint: "",
  webSocketEndpoint: "",
  grpcEndpoint: "",
  restEndpoint: "",
  healthEndpoint: "",
  grpcService: "",
  grpcProtoImportPath: "",
  sourceLabels: {},
  methods: [],
  restOperations: [],
  project: {
    name: "",
    version: "",
    bundleId: "",
    runMode: "",
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
  rest: "REST",
};
const jwtTokenPlaceholder = "<access_token>";
const orgIDPlaceholder = "<org_id>";

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

// sourceLabel 把来源标识翻成导航里的功能域名：标签表由 Go 侧注入，缺项按标识原样显示，
// 空标识（单一来源的服务）显示「本服务」。
function sourceLabel(source: string | undefined): string {
  const key = (source ?? "").trim();
  if (key === "") {
    return "本服务";
  }
  return config.sourceLabels[key] ?? config.sourceLabels[key.toLowerCase()] ?? key;
}

function classifyName(
  name: string,
  fallbackGroup: string,
): Pick<CatalogItem, "group" | "functionName"> {
  const parts = name.split(/[./:]+/).filter(Boolean);
  if (parts.length >= 2) {
    return {
      group: parts[0],
      functionName: parts.slice(1).join("."),
    };
  }
  return {
    group: fallbackGroup,
    functionName: parts[0] || name,
  };
}

function buildCatalogItems(): CatalogItem[] {
  if (protocol === "rest") {
    return config.restOperations.map((operation) => {
      const firstSegment = operation.path.split("/").filter(Boolean)[0] ?? "root";
      return {
        id: `rest:${operation.method} ${operation.path}`,
        domain: sourceLabel(operation.origin),
        group: operation.tags?.[0] ?? firstSegment,
        functionName: `${operation.method} ${operation.path}`,
        rpcMethod: operation.operationId,
        description: operation.summary || operation.description || "已注册 REST 路由",
        inputSchema: operation.requestSchema ?? { type: "object" },
        kind: "REST",
        toolName: operation.backendMethod,
        source: operation.origin ?? "",
        protected: operation.protected,
        acceptsOrgId: operation.protected,
        rest: operation,
      };
    });
  }
  return config.methods.map((method) => {
    const classified = classifyName(method.name, "General");
    return {
      id: `api:${method.name}`,
      domain: sourceLabel(method.source),
      ...classified,
      rpcMethod: "tools/call",
      toolName: method.name,
      description: method.description || "已注册 API 方法",
      inputSchema: method.inputSchema ?? { type: "object" },
      kind: protocolCatalogKind(),
      source: method.source,
      protected: method.protected,
      acceptsOrgId: method.acceptsOrgId,
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
    case "rest":
      return "REST";
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
    renderApiDescription(null);
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

// paramsFromSchema 按入参 schema 生成示例参数；测试账号与令牌 / 组织按字段名回填。
function paramsFromSchema(
  schema: Record<string, unknown> | undefined,
): Record<string, unknown> {
  const value = valueFromSchema(schema);
  if (value !== null && !Array.isArray(value) && typeof value === "object") {
    const params = value as Record<string, unknown>;
    fillTestAccount(params);
    fillGateArguments(params);
    return params;
  }
  return {};
}

function fillTestAccount(params: Record<string, unknown>): void {
  if (Object.prototype.hasOwnProperty.call(params, "user_name")) {
    params.user_name = testUserNameInput.value;
  }
  if (Object.prototype.hasOwnProperty.call(params, "email")) {
    params.email = testEmailInput.value;
  }
  if (Object.prototype.hasOwnProperty.call(params, "phone")) {
    params.phone = testPhoneInput.value;
  }
  if (Object.prototype.hasOwnProperty.call(params, "password")) {
    params.password = testPasswordInput.value;
  }
}

// fillGateArguments 只在 schema 声明了门禁字段时回填：jwt_token / org_id 由注册表按方法注入，
// 页面不替方法多加一个键（后端 schema 严格键集，多一个即拒）。
function fillGateArguments(params: Record<string, unknown>): void {
  if (Object.prototype.hasOwnProperty.call(params, "jwt_token")) {
    params.jwt_token = testAccessTokenInput.value || jwtTokenPlaceholder;
  }
  if (Object.prototype.hasOwnProperty.call(params, "org_id")) {
    params.org_id = testOrgIDInput.value || orgIDPlaceholder;
  }
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

// defaultHeaders 按协议出默认头：REST 受保护路由带 Authorization / X-Org-Id 占位；
// MCP 带协议版本与 tools/call 提示头；WS / gRPC 无 HTTP 头。
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
  if (protocol === "rest") {
    if (!selectedItem?.rest?.requestSchema) {
      delete headers["Content-Type"];
    }
    if (selectedItem?.protected) {
      headers.Authorization = `Bearer ${testAccessTokenInput.value || jwtTokenPlaceholder}`;
      headers["X-Org-Id"] = testOrgIDInput.value || orgIDPlaceholder;
    }
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

// restEnvelope 给 REST 路由出默认信封：路径参数留占位，查询参数按 schema 出示例，请求体按 schema 出示例。
function restEnvelope(operation: RESTOperation): RequestEnvelope {
  const query: Record<string, unknown> = {};
  for (const parameter of operation.parameters ?? []) {
    if (parameter.in === "query") {
      query[parameter.name] = valueFromSchema(parameter.schema);
    }
  }
  const body =
    operation.requestSchema && ["POST", "PUT", "PATCH", "DELETE"].includes(operation.method)
      ? paramsFromSchema(operation.requestSchema)
      : {};
  return {
    method: operation.method,
    path: operation.path,
    query,
    headers: defaultHeaders(),
    body,
  };
}

function presetEnvelope(): RequestEnvelope {
  if (protocol === "rest" && selectedItem?.rest) {
    return restEnvelope(selectedItem.rest);
  }
  return {
    headers: defaultHeaders(),
    body: makeBody(paramsForSelection()),
  };
}

function applyPreset(): void {
  if (!selectedItem) {
    return;
  }
  endpointInput.value = protocolEndpoint();
  transportMethodBadge.textContent =
    protocol === "websocket"
      ? "WS"
      : protocol === "grpc"
        ? "RPC"
        : protocol === "rest"
          ? selectedItem.rest?.method ?? "GET"
          : "POST";
  requestBody.value = JSON.stringify(presetEnvelope(), null, 2);
  requestShapeHint.textContent =
    protocol === "rest" ? "method + path + query + headers + body" : "headers + body 合一";
  codeMethodLabel.textContent =
    protocol === "rest"
      ? selectedItem.functionName
      : (selectedItem.toolName ?? selectedItem.rpcMethod);
  renderApiDescription(selectedItem);
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
    case "rest":
      return config.restEndpoint;
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

// parseEnvelope 读编辑器里的合一 JSON；headers 缺省为空对象，body 缺省为空对象。
function parseEnvelope(): RequestEnvelope {
  const raw = parseJSONObject(requestBody.value, "实际 JSON");
  const headers =
    raw.headers !== null && !Array.isArray(raw.headers) && typeof raw.headers === "object"
      ? (raw.headers as Record<string, unknown>)
      : {};
  const body =
    raw.body !== null && !Array.isArray(raw.body) && typeof raw.body === "object"
      ? (raw.body as Record<string, unknown>)
      : {};
  const query =
    raw.query !== null && !Array.isArray(raw.query) && typeof raw.query === "object"
      ? (raw.query as Record<string, unknown>)
      : undefined;
  return {
    method: typeof raw.method === "string" ? raw.method : undefined,
    path: typeof raw.path === "string" ? raw.path : undefined,
    query,
    headers,
    body,
  };
}

function writeEnvelope(envelope: RequestEnvelope): void {
  const ordered: Record<string, unknown> = {};
  if (protocol === "rest") {
    ordered.method = envelope.method;
    ordered.path = envelope.path;
    ordered.query = envelope.query ?? {};
  }
  ordered.headers = envelope.headers;
  ordered.body = envelope.body;
  requestBody.value = JSON.stringify(ordered, null, 2);
}

// syncTestAccountToRequest 把顶栏账号 / 令牌 / 组织改动同步进当前请求：只改已存在的键。
function syncTestAccountToRequest(): void {
  try {
    const envelope = parseEnvelope();
    if (protocol === "rest") {
      fillTestAccount(envelope.body);
      if (Object.prototype.hasOwnProperty.call(envelope.headers, "Authorization")) {
        envelope.headers.Authorization = `Bearer ${testAccessTokenInput.value || jwtTokenPlaceholder}`;
      }
      if (Object.prototype.hasOwnProperty.call(envelope.headers, "X-Org-Id")) {
        envelope.headers["X-Org-Id"] = testOrgIDInput.value || orgIDPlaceholder;
      }
    } else {
      const params = envelope.body.params;
      if (params !== null && !Array.isArray(params) && typeof params === "object") {
        const argumentsValue = (params as Record<string, unknown>).arguments;
        if (
          argumentsValue !== null &&
          !Array.isArray(argumentsValue) &&
          typeof argumentsValue === "object"
        ) {
          const argumentsObject = argumentsValue as Record<string, unknown>;
          fillTestAccount(argumentsObject);
          fillGateArguments(argumentsObject);
        }
      }
    }
    writeEnvelope(envelope);
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

function showRequestError(message: string): void {
  requestError.textContent = message;
}

function clearRequestError(): void {
  requestError.textContent = "";
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

// schemaRows 把入参 schema 展开一层成表格行：字段 / 类型 / 必填 / 说明。
function schemaRows(
  schema: Record<string, unknown> | undefined,
): Array<{ name: string; type: string; required: boolean; description: string }> {
  if (!schema) {
    return [];
  }
  const properties =
    schema.properties !== null && typeof schema.properties === "object"
      ? (schema.properties as Record<string, unknown>)
      : {};
  const required = new Set(Array.isArray(schema.required) ? (schema.required as string[]) : []);
  return Object.entries(properties).map(([name, propertySchema]) => {
    const property =
      propertySchema !== null && typeof propertySchema === "object"
        ? (propertySchema as Record<string, unknown>)
        : {};
    let type = Array.isArray(property.type)
      ? property.type.join(" | ")
      : typeof property.type === "string"
        ? property.type
        : property.$ref
          ? "object"
          : "any";
    if (Array.isArray(property.enum)) {
      type = `enum(${(property.enum as unknown[]).map(String).join(" / ")})`;
    }
    if (type === "array" && property.items !== null && typeof property.items === "object") {
      const items = property.items as Record<string, unknown>;
      type = `array<${typeof items.type === "string" ? items.type : "object"}>`;
    }
    return {
      name,
      type,
      required: required.has(name),
      description: typeof property.description === "string" ? property.description : "",
    };
  });
}

// renderApiDescription 渲染「接口说明」面板：方法名、来源、描述、门禁、入参表；REST 项再带 method + path + 参数。
function renderApiDescription(item: CatalogItem | null): void {
  if (!item) {
    apiDescriptionSource.textContent = "—";
    apiDescription.innerHTML = '<p class="description-empty">没有可选方法</p>';
    return;
  }
  apiDescriptionSource.textContent = `来源 · ${item.domain}`;
  const parts: string[] = [];
  const title = item.rest ? `${item.rest.method} ${item.rest.path}` : (item.toolName ?? item.rpcMethod);
  parts.push(`<div class="description-title"><code>${escapeHTML(title)}</code></div>`);
  const badges: string[] = [`<span class="description-badge">${escapeHTML(item.kind)}</span>`];
  badges.push(
    item.protected
      ? '<span class="description-badge is-protected">受保护 · 需令牌</span>'
      : '<span class="description-badge is-public">公开</span>',
  );
  if (item.acceptsOrgId && !item.rest) {
    badges.push('<span class="description-badge">按活动组织注入 org_id</span>');
  }
  if (item.rest?.backendMethod) {
    badges.push(`<span class="description-badge">后端方法 · ${escapeHTML(item.rest.backendMethod)}</span>`);
  }
  if (item.rest?.operationId) {
    badges.push(`<span class="description-badge">operationId · ${escapeHTML(item.rest.operationId)}</span>`);
  }
  parts.push(`<div class="description-badges">${badges.join("")}</div>`);
  parts.push(`<p class="description-text">${escapeHTML(item.description)}</p>`);
  if (item.rest?.description && item.rest.description !== item.description) {
    parts.push(`<p class="description-text">${escapeHTML(item.rest.description)}</p>`);
  }
  if (item.rest && (item.rest.parameters?.length ?? 0) > 0) {
    parts.push('<div class="description-section">参数</div>');
    parts.push(
      `<table class="description-table"><thead><tr><th>名称</th><th>位置</th><th>必填</th><th>说明</th></tr></thead><tbody>${(item.rest.parameters ?? [])
        .map(
          (parameter) =>
            `<tr><td><code>${escapeHTML(parameter.name)}</code></td><td>${escapeHTML(parameter.in)}</td><td>${parameter.required ? "是" : "否"}</td><td>${escapeHTML(parameter.description ?? "")}</td></tr>`,
        )
        .join("")}</tbody></table>`,
    );
  }
  const rows = schemaRows(item.inputSchema);
  parts.push(`<div class="description-section">${item.rest ? "请求体" : "入参"}</div>`);
  if (rows.length === 0) {
    parts.push('<p class="description-empty">无入参</p>');
  } else {
    parts.push(
      `<table class="description-table"><thead><tr><th>字段</th><th>类型</th><th>必填</th><th>说明</th></tr></thead><tbody>${rows
        .map(
          (row) =>
            `<tr><td><code>${escapeHTML(row.name)}</code></td><td>${escapeHTML(row.type)}</td><td>${row.required ? "是" : "否"}</td><td>${escapeHTML(row.description)}</td></tr>`,
        )
        .join("")}</tbody></table>`,
    );
  }
  apiDescription.innerHTML = parts.join("");
}

// restTargetURL 把信封里的 path / query 拼到 REST 端点上；路径参数占位 {name} 原样保留给用户改。
function restTargetURL(envelope: RequestEnvelope): string {
  const base = endpointInput.value.replace(/\/+$/, "");
  const path = (envelope.path ?? "/").startsWith("/") ? (envelope.path ?? "/") : `/${envelope.path}`;
  const query = Object.entries(envelope.query ?? {})
    .filter(([, value]) => value !== null && value !== undefined && value !== "")
    .map(([name, value]) => `${encodeURIComponent(name)}=${encodeURIComponent(String(value))}`)
    .join("&");
  return `${base}${path}${query ? `?${query}` : ""}`;
}

async function sendRequest(): Promise<void> {
  clearRequestError();
  let envelope: RequestEnvelope;
  try {
    envelope = parseEnvelope();
  } catch (error) {
    showRequestError(errorMessage(error));
    return;
  }
  const body = envelope.body;
  if (protocol === "grpc") {
    requestIDInput.value = generateRequestID();
    body.requestId = requestIDInput.value;
    delete body.id;
  } else if (protocol !== "rest") {
    if (body.method === "notifications/initialized") {
      delete body.id;
    } else {
      requestIDInput.value = generateRequestID();
      body.id = requestIDInput.value;
    }
  }
  writeEnvelope(envelope);
  renderInvocationCode();

  const headerStrings: Record<string, string> = {};
  for (const [name, value] of Object.entries(envelope.headers)) {
    headerStrings[name] = String(value);
  }
  const isREST = protocol === "rest";
  const method = isREST ? (envelope.method ?? "GET").toUpperCase() : "POST";
  const url = isREST ? restTargetURL(envelope) : endpointInput.value;
  const hasBody = !isREST || !["GET", "HEAD"].includes(method);

  setLoading(true);
  const startedAt = performance.now();
  try {
    const response = await fetch("/api/proxy", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        transport: protocol,
        url,
        method,
        headers: headerStrings,
        body: hasBody ? JSON.stringify(body) : "",
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


function shellQuote(value: string): string {
  return `'${value.replaceAll("'", `'\\''`)}'`;
}

function buildCommand(): string {
  let envelope: RequestEnvelope = { headers: {}, body: {} };
  try {
    envelope = parseEnvelope();
  } catch {
    // Copy the usable portion even while the editor is invalid.
  }
  const headerArgs = Object.entries(envelope.headers)
    .map(([name, value]) => `-H ${shellQuote(`${name}: ${String(value)}`)}`)
    .join(" \\\n  ");
  const bodyJSON = JSON.stringify(envelope.body);
  if (protocol === "websocket") {
    return [
      `printf '%s' ${shellQuote(bodyJSON)}`,
      `websocat -1 ${shellQuote(endpointInput.value)}`,
    ].join(" | ");
  }
  if (protocol === "grpc") {
    const endpoint = endpointInput.value.replace(/^grpc:\/\//, "");
    return [
      "grpcurl -plaintext",
      `-import-path ${config.grpcProtoImportPath || "<proto-dir>"}`,
      "-proto api.proto",
      `-d ${shellQuote(bodyJSON)}`,
      shellQuote(endpoint),
      `${config.grpcService || "<pkg>.APIService"}/Call`,
    ].join(" ");
  }
  if (protocol === "rest") {
    const method = (envelope.method ?? "GET").toUpperCase();
    const parts = [
      `curl -X ${method} ${shellQuote(restTargetURL(envelope))}`,
      headerArgs,
      ["GET", "HEAD"].includes(method) ? "" : `--data-raw ${shellQuote(bodyJSON)}`,
    ].filter(Boolean);
    return parts.join(" \\\n  ");
  }
  const parts = [
    `curl -X POST ${shellQuote(endpointInput.value)}`,
    headerArgs,
    `--data-raw ${shellQuote(bodyJSON)}`,
  ].filter(Boolean);
  return parts.join(" \\\n  ");
}

function invocationFunctionName(): string {
  const source =
    selectedItem?.toolName ?? selectedItem?.rest?.operationId ?? selectedItem?.functionName ?? "api";
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
  method: string,
  url: string,
  body: Record<string, unknown>,
  headers: Record<string, string>,
  withBody: boolean,
): string {
  const headerLines = Object.entries(headers)
    .map(
      ([name, value]) =>
        `\trequest.Header.Set(${goString(name)}, ${goString(value)})`,
    )
    .join("\n");
  const bodyLine = withBody
    ? `\tbody := []byte(${goString(JSON.stringify(body))})\n\trequest, err := http.NewRequestWithContext(ctx, ${goString(method)}, ${goString(url)}, bytes.NewReader(body))`
    : `\trequest, err := http.NewRequestWithContext(ctx, ${goString(method)}, ${goString(url)}, nil)`;
  return `// imports: ${withBody ? "bytes, " : ""}context, fmt, io, net/http
func Call${functionName}(ctx context.Context) ([]byte, error) {
${bodyLine}
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

function buildGoInvocation(envelope: RequestEnvelope): string {
  const functionName = invocationFunctionName();
  const headers = stringHeaders(envelope.headers);
  switch (protocol) {
    case "grpc":
      return buildGoGRPCInvocation(functionName, envelope.body);
    case "websocket":
      return buildGoWebSocketInvocation(functionName, envelope.body);
    case "rest": {
      const method = (envelope.method ?? "GET").toUpperCase();
      return buildGoHTTPInvocation(
        functionName,
        method,
        restTargetURL(envelope),
        envelope.body,
        headers,
        !["GET", "HEAD"].includes(method),
      );
    }
    default:
      return buildGoHTTPInvocation(functionName, "POST", endpointInput.value, envelope.body, headers, true);
  }
}

function buildTSHTTPInvocation(
  functionName: string,
  method: string,
  url: string,
  body: Record<string, unknown>,
  headers: Record<string, string>,
  withBody: boolean,
): string {
  const bodyLine = withBody
    ? `\n    body: JSON.stringify(${indentCode(JSON.stringify(body, null, 2), 4).trimStart()}),`
    : "";
  return `export async function call${functionName}(): Promise<unknown> {
  const response = await fetch(${JSON.stringify(url)}, {
    method: ${JSON.stringify(method)},
    headers: ${indentCode(JSON.stringify(headers, null, 2), 4).trimStart()},${bodyLine}
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
  const definition = protoLoader.loadSync("cg_api/api_grpc/proto/api.proto", {
    keepCase: false,
    longs: String,
    enums: String,
    defaults: true,
    oneofs: true,
  });
  const grpcObject = grpc.loadPackageDefinition(definition) as any;
  const Client = grpcObject.${config.grpcService || "<pkg>.APIService"};
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

function buildTSInvocation(envelope: RequestEnvelope): string {
  const functionName = invocationFunctionName();
  const headers = stringHeaders(envelope.headers);
  switch (protocol) {
    case "grpc":
      return buildTSGRPCInvocation(functionName, envelope.body);
    case "websocket":
      return buildTSWebSocketInvocation(functionName, envelope.body);
    case "rest": {
      const method = (envelope.method ?? "GET").toUpperCase();
      return buildTSHTTPInvocation(
        functionName,
        method,
        restTargetURL(envelope),
        envelope.body,
        headers,
        !["GET", "HEAD"].includes(method),
      );
    }
    default:
      return buildTSHTTPInvocation(functionName, "POST", endpointInput.value, envelope.body, headers, true);
  }
}

function renderInvocationCode(): void {
  codeLanguageTag.textContent = codeLanguage.toUpperCase();
  try {
    const envelope = parseEnvelope();
    invocationCode.textContent =
      codeLanguage === "go"
        ? buildGoInvocation(envelope)
        : buildTSInvocation(envelope);
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
  byId<HTMLElement>("jsonrpc-port").textContent = endpointPort(config.jsonRpcEndpoint);
  byId<HTMLElement>("mcp-port").textContent = endpointPort(config.mcpEndpoint);
  byId<HTMLElement>("websocket-port").textContent = endpointPort(config.webSocketEndpoint);
  byId<HTMLElement>("grpc-port").textContent = endpointPort(config.grpcEndpoint);
  byId<HTMLElement>("rest-port").textContent = endpointPort(config.restEndpoint);
  byId<HTMLElement>("health-port").textContent = endpointPort(config.healthEndpoint);
  byId<HTMLElement>("rest-fact").hidden = config.restEndpoint === "";
  byId<HTMLElement>("health-fact").hidden = config.healthEndpoint === "";
  // 端点为空的协议面不显示页签；当前选中的协议被隐藏时退回 MCP。
  const endpointOf: Record<Protocol, string> = {
    jsonrpc: config.jsonRpcEndpoint,
    mcp: config.mcpEndpoint,
    websocket: config.webSocketEndpoint,
    grpc: config.grpcEndpoint,
    rest: config.restEndpoint,
  };
  for (const tab of document.querySelectorAll<HTMLButtonElement>(".protocol-tab")) {
    const key = tab.dataset.protocol as Protocol;
    tab.hidden = endpointOf[key] === "";
  }
  if (endpointOf[protocol] === "" && config.mcpEndpoint !== "") {
    protocol = "mcp";
  }
  const labels = unique(
    config.methods.map((method) => sourceLabel(method.source)).concat(
      config.restOperations.map((operation) => sourceLabel(operation.origin)),
    ),
  );
  byId<HTMLElement>("source-summary").textContent =
    labels.length > 1 ? `${labels.join(" · ")} 多路真源` : `${labels[0] ?? "本服务"} 方法表`;
  document.title = `${config.project.name || "API"} · API Lab`;
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
      restOperations: loadedConfig.restOperations ?? config.restOperations,
      sourceLabels: loadedConfig.sourceLabels ?? config.sourceLabels,
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
endpointInput.addEventListener("input", renderInvocationCode);
testUserNameInput.addEventListener("input", syncTestAccountToRequest);
testEmailInput.addEventListener("input", syncTestAccountToRequest);
testPhoneInput.addEventListener("input", syncTestAccountToRequest);
testPasswordInput.addEventListener("input", syncTestAccountToRequest);
testAccessTokenInput.addEventListener("input", syncTestAccountToRequest);
testOrgIDInput.addEventListener("input", syncTestAccountToRequest);
byId<HTMLButtonElement>("format-request").addEventListener("click", () => {
  formatJSON(requestBody, "实际 JSON");
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

for (const tab of document.querySelectorAll<HTMLButtonElement>(".protocol-tab")) {
  const active = tab.dataset.protocol === protocol;
  tab.classList.toggle("is-active", active);
  tab.setAttribute("aria-pressed", String(active));
}
initializeTripleSplitViews();
initializeSplitViews();
renderHistory();
requestIDInput.value = generateRequestID();
void loadConfig();
