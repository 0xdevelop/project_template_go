var l=(i)=>{let x=document.getElementById(i);if(!x)throw Error(`Missing element: ${i}`);return x},z=l("endpoint"),Di=l("transport-method"),R=l("api-domain"),C=l("api-group"),S=l("api-function"),j=l("request-id"),Y=l("request-body"),F=l("request-headers"),Yi=l("request-error"),gi=l("test-email"),ri=l("test-password"),Fi=l("catalog-count"),Ui=l("code-method-label"),Ei=l("code-language-tag"),P=l("invocation-code"),wi=l("headers-count"),T=l("send-request"),Wi=l("response-meta"),Mi=l("response-empty"),W=l("response-code"),Xi=l("copy-response"),ki=l("history-list"),e=l("toast"),J=l("root-workspace"),Oi=l("catalog-columns"),xi=new WeakMap,b="jsonrpc",f={jsonRpcEndpoint:"http://127.0.0.1:13001",mcpEndpoint:"http://127.0.0.1:13002",webSocketEndpoint:"ws://127.0.0.1:13004",grpcEndpoint:"grpc://127.0.0.1:13005",methods:[],project:{name:"project_template_go",version:"v0.0.2",bundleId:"com.project_template_go.project_template_go",runMode:"Debug"}},s=[],h=null,L=null,M="decoded",pi="go",O=[],Zi="2026-07-28",Ri={jsonrpc:"RPC",mcp:"MCP",websocket:"WS",grpc:"gRPC"};function oi(){while(!0){let x=crypto.getRandomValues(new Uint8Array(4)),p=Array.from(x,(o)=>"abcdefghijklmnopqrstuvwxyz0123456789"[o%36]).join("");if(/[a-z]/.test(p)&&/\d/.test(p))return`${Date.now()}${p}`}}function q(i){return[...new Set(i)]}function ui(i,x,p){let o=i.split(/[./:]+/).filter(Boolean);if(o.length>=3)return{domain:o[0],group:o[1],functionName:o.slice(2).join(".")};if(o.length===2)return{domain:o[0],group:p,functionName:o[1]};return{domain:x,group:p,functionName:o[0]||i}}function Ti(){return f.methods.map((i)=>{let x=ui(i.name,"Core","General");return{id:`api:${i.name}`,...x,rpcMethod:"tools/call",toolName:i.name,description:i.description||"已注册 API 方法",inputSchema:i.inputSchema??{type:"object"},kind:Ni()}})}function Ni(){switch(b){case"mcp":return"MCP TOOL";case"websocket":return"WEBSOCKET";case"grpc":return"gRPC";default:return"JSON-RPC"}}function ai(i,x,p){i.replaceChildren(...x.map(({value:n,label:t})=>{let a=document.createElement("option");return a.value=n,a.textContent=t,a}));let o=p&&x.some((n)=>n.value===p)?p:x[0]?.value;if(o)i.value=o}function ni(i){s=Ti(),Fi.textContent=`${s.length} functions`,wx();let x=s.find((p)=>p.rpcMethod===i||p.toolName===i)??s[0];ai(R,q(s.map((p)=>p.domain)).map((p)=>({value:p,label:p})),x?.domain),_i(x?.group,x?.id)}function _i(i,x){let p=s.filter((o)=>o.domain===R.value);ai(C,q(p.map((o)=>o.group)).map((o)=>({value:o,label:o})),i),$i(x)}function $i(i){let x=s.filter((p)=>p.domain===R.value&&p.group===C.value);ai(S,x.map((p)=>({value:p.id,label:p.functionName})),i),Ki()}function Ki(){if(h=s.find((i)=>i.id===S.value)??s[0]??null,!h){Ui.textContent="no function",P.textContent="";return}Ai()}function Pi(){return{name:h?.toolName??"",arguments:qi(h?.inputSchema)}}function qi(i){let x=ji(i);if(x!==null&&!Array.isArray(x)&&typeof x==="object"){let p=x;if(Object.prototype.hasOwnProperty.call(p,"email"))p.email=gi.value;if(Object.prototype.hasOwnProperty.call(p,"password"))p.password=ri.value;return p}return{}}function ji(i){if(!i)return{};if("default"in i)return i.default;if("example"in i)return i.example;if(Array.isArray(i.examples)&&i.examples.length>0)return i.examples[0];if("const"in i)return i.const;if(Array.isArray(i.enum)&&i.enum.length>0)return i.enum[0];let x=Array.isArray(i.type)?i.type.find((p)=>p!=="null"):i.type;if(x==="object"||x===void 0&&i.properties!==null&&typeof i.properties==="object"){let p=i.properties!==null&&typeof i.properties==="object"?i.properties:{};return Object.fromEntries(Object.entries(p).map(([o,n])=>[o,ji(n!==null&&typeof n==="object"?n:void 0)]))}if(x==="array")return[];if(x==="boolean")return!1;if(x==="integer"||x==="number")return 0;if(x==="string")return"";return null}function Ci(){if(b==="websocket"||b==="grpc")return{};let i={"Content-Type":"application/json",Accept:"application/json"};if(b==="mcp")i.Accept="application/json, text/event-stream",i["Mcp-Protocol-Version"]=Zi,i["Mcp-Method"]="tools/call",i["Mcp-Name"]=h?.toolName??"";return i}function Si(i){let x=h?.rpcMethod??"";if(b==="grpc")return{requestId:j.value,method:x,params:i};if(b==="mcp")i={...i,_meta:{"io.modelcontextprotocol/protocolVersion":Zi,"io.modelcontextprotocol/clientInfo":{name:f.project.name,version:f.project.version},"io.modelcontextprotocol/clientCapabilities":{extensions:{}}}};let p={jsonrpc:"2.0",id:j.value,method:x,params:i};if(x==="notifications/initialized")delete p.id;return p}function Ai(){if(!h)return;z.value=Ii(),Di.textContent=b==="websocket"?"WS":b==="grpc"?"RPC":"POST",Y.value=JSON.stringify(Si(Pi()),null,2),F.value=JSON.stringify(Ci(),null,2),Ui.textContent=h.toolName??h.rpcMethod,Ji(),A(),fi()}function Ii(){switch(b){case"mcp":return f.mcpEndpoint;case"websocket":return f.webSocketEndpoint;case"grpc":return f.grpcEndpoint;default:return f.jsonRpcEndpoint}}function ei(i){let x=h?.toolName??h?.rpcMethod;b=i;for(let p of document.querySelectorAll(".protocol-tab")){let o=p.dataset.protocol===b;p.classList.toggle("is-active",o),p.setAttribute("aria-pressed",String(o))}ni(x)}function y(i,x){let p=JSON.parse(i);if(p===null||Array.isArray(p)||typeof p!=="object")throw Error(`${x} 必须是 JSON object`);return p}function Qi(){try{let i=y(Y.value,"Request body"),x=i.params;if(x===null||Array.isArray(x)||typeof x!=="object")return;let p=x.arguments;if(p===null||Array.isArray(p)||typeof p!=="object")return;let o=p;if(Object.prototype.hasOwnProperty.call(o,"email"))o.email=gi.value;if(Object.prototype.hasOwnProperty.call(o,"password"))o.password=ri.value;Y.value=JSON.stringify(i,null,2),A()}catch(i){I(D(i))}}function ix(i,x){try{i.value=JSON.stringify(JSON.parse(i.value),null,2),fi()}catch(p){I(`${x} JSON 无效：${D(p)}`)}}function Ji(){try{let i=y(F.value,"Headers");wi.textContent=`${Object.keys(i).length} 项`}catch{wi.textContent="格式错误"}}function I(i){Yi.textContent=i}function fi(){Yi.textContent=""}function D(i){return i instanceof Error?i.message:String(i)}async function Gi(){fi();let i,x;try{i=y(Y.value,"Request body"),x=y(F.value,"Headers")}catch(n){I(D(n));return}if(b==="grpc")j.value=oi(),i.requestId=j.value,delete i.id;else if(i.method==="notifications/initialized")delete i.id;else j.value=oi(),i.id=j.value;Y.value=JSON.stringify(i,null,2),A();let p={};for(let[n,t]of Object.entries(x))p[n]=String(t);vi(!0);let o=performance.now();try{let n=await fetch("/api/proxy",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({transport:b,url:z.value,method:"POST",headers:p,body:JSON.stringify(i)})}),t=await n.json();if(!n.ok||"error"in t){mi(n.status,n.statusText,"error"in t?t.error:`HTTP ${n.status}`,Math.round(performance.now()-o)),ti("通信失败");return}L=t,Li(t),Bi(t)}catch(n){mi(0,"Communication Error",D(n),Math.round(performance.now()-o)),ti("通信失败")}finally{vi(!1)}}function vi(i){T.disabled=i,T.classList.toggle("is-loading",i);let x=T.querySelector("span");if(x)x.textContent=i?"请求中…":"请求"}function Li(i){Mi.classList.add("is-hidden"),W.classList.remove("is-hidden"),Xi.disabled=!1;let x=i.errorKind==="communication",p=i.protocolError===!0,o=!x&&!p&&px(i.body),n=!x&&!p&&xx(i.status)&&!o,t=p?i.statusText:x?`${i.status||"—"} COMM`:`${i.status}${o?" RPC":""}`;Wi.innerHTML=`
    <span class="status-pill ${n?"is-success":"is-error"}">
      ${Vi(t)}
    </span>
    <span>${i.durationMs} ms</span>
    <span>${ox(i.size)}${i.wasTruncated?" · truncated":""}</span>
  `,yi()}function mi(i,x,p,o){let n=JSON.stringify({error:p},null,2),t={status:i,statusText:x,headers:{},body:n,durationMs:o,size:new TextEncoder().encode(n).length,wasTruncated:!1,errorKind:"communication"};M="decoded";for(let a of document.querySelectorAll(".response-tab")){let c=a.dataset.responseTab===M;a.classList.toggle("is-active",c),a.setAttribute("aria-selected",String(c))}L=t,Li(t),Bi(t)}function xx(i){return i>=200&&i<300||b==="websocket"&&i===101}function px(i){try{let x=JSON.parse(i);return x!==null&&typeof x==="object"&&x.error!=null}catch{return!1}}function yi(){if(!L)return;if(M==="headers"){W.textContent=JSON.stringify(L.headers,null,2);return}try{let i=JSON.parse(L.body);W.textContent=JSON.stringify(M==="decoded"?N(i):i,null,2)}catch{W.textContent=L.body||"(empty response body)"}}function N(i){if(Array.isArray(i))return i.map(N);if(i!==null&&typeof i==="object")return Object.fromEntries(Object.entries(i).map(([n,t])=>[n,N(t)]));if(typeof i!=="string")return i;let x=i.trim(),p=x.startsWith("{")&&x.endsWith("}"),o=x.startsWith("[")&&x.endsWith("]");if(!p&&!o)return i;try{return N(JSON.parse(x))}catch{return i}}function ox(i){if(i<1024)return`${i} B`;return`${(i/1024).toFixed(1)} KB`}function Bi(i){O.unshift({method:h?.toolName??h?.rpcMethod??"unknown",protocol:b,status:i.status,duration:i.durationMs}),O.splice(6),bi()}function bi(){if(O.length===0){let i=document.createElement("li");i.className="history-empty",i.textContent="暂无请求记录",ki.replaceChildren(i);return}ki.replaceChildren(...O.map((i)=>{let x=document.createElement("li");return x.innerHTML=`
        <span class="history-protocol">${Ri[i.protocol]}</span>
        <strong>${Vi(i.method)}</strong>
        <span class="${i.status>=200&&i.status<300||i.protocol==="websocket"&&i.status===101?"history-ok":"history-failed"}">${i.status}</span>
        <small>${i.duration} ms</small>
      `,x}))}function Vi(i){return i.replace(/[&<>"']/g,(x)=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#039;"})[x]??x)}function nx(){let i={};try{i=y(F.value,"Headers")}catch{}let x=Object.entries(i).map(([o,n])=>`-H ${G(`${o}: ${String(n)}`)}`).join(" \\\n  ");if(b==="websocket")return[`printf '%s' ${G(Y.value)}`,`websocat -1 ${G(z.value)}`].join(" | ");if(b==="grpc"){let o=z.value.replace(/^grpc:\/\//,"");return["grpcurl -plaintext","-import-path api/api_grpc/proto","-proto api.proto",`-d ${G(Y.value)}`,G(o),"project_template_go.api.v1.APIService/Call"].join(" ")}return[`curl -X POST ${G(z.value)}`,x,`--data-raw ${G(Y.value)}`].filter(Boolean).join(" \\\n  ")}function G(i){return`'${i.replaceAll("'","'\\''")}'`}function Hi(){let p=(h?.toolName??h?.functionName??"api").split(/[^a-zA-Z0-9]+/).filter(Boolean).map((o)=>o.charAt(0).toUpperCase()+o.slice(1)).join("");return/^\d/.test(p)?`Method${p}`:p||"API"}function li(i,x){let p=" ".repeat(x);return i.split(`
`).map((o)=>`${p}${o}`).join(`
`)}function Z(i){return JSON.stringify(i)}function lx(i){return Object.fromEntries(Object.entries(i).map(([x,p])=>[x,String(p)]))}function tx(i,x,p){let o=Object.entries(p).map(([n,t])=>`	request.Header.Set(${Z(n)}, ${Z(t)})`).join(`
`);return`// imports: bytes, context, fmt, io, net/http
func Call${i}(ctx context.Context) ([]byte, error) {
	body := []byte(${Z(JSON.stringify(x))})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, ${Z(z.value)}, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
${o}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed: %s: %s", response.Status, responseBody)
	}
	return responseBody, nil
}`}function gx(i,x){return`// import: context, github.com/coder/websocket
func Call${i}(ctx context.Context) ([]byte, error) {
	connection, _, err := websocket.Dial(ctx, ${Z(z.value)}, nil)
	if err != nil {
		return nil, err
	}
	defer connection.CloseNow()

	if err := connection.Write(ctx, websocket.MessageText, []byte(${Z(JSON.stringify(x))})); err != nil {
		return nil, err
	}
	_, response, err := connection.Read(ctx)
	if err != nil {
		return nil, err
	}
	return response, connection.Close(websocket.StatusNormalClosure, "")
}`}function rx(i,x){let p=x.params!==null&&!Array.isArray(x.params)&&typeof x.params==="object"?x.params:{},o=z.value.replace(/^grpc:\/\//,""),n=String(x.requestId??j.value),t=String(x.method??h?.rpcMethod??"tools/call");return`// imports: context, encoding/json, api_grpc_protobuf, grpc, insecure, structpb
func Call${i}(ctx context.Context) (*api_grpc_protobuf.CallResponse, error) {
	connection, err := grpc.NewClient(${Z(o)}, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer connection.Close()

	var params map[string]any
	if err := json.Unmarshal([]byte(${Z(JSON.stringify(p))}), &params); err != nil {
		return nil, err
	}
	paramsValue, err := structpb.NewStruct(params)
	if err != nil {
		return nil, err
	}

	return api_grpc_protobuf.NewAPIServiceClient(connection).Call(ctx, &api_grpc_protobuf.CallRequest{
		RequestId: ${Z(n)},
		Method:    ${Z(t)},
		Params:    paramsValue,
	})
}`}function ax(i,x){let p=Hi();switch(b){case"grpc":return rx(p,i);case"websocket":return gx(p,i);default:return tx(p,i,x)}}function fx(i,x,p){return`export async function call${i}(): Promise<unknown> {
  const response = await fetch(${JSON.stringify(z.value)}, {
    method: "POST",
    headers: ${li(JSON.stringify(p,null,2),4).trimStart()},
    body: JSON.stringify(${li(JSON.stringify(x,null,2),4).trimStart()}),
  });
  if (!response.ok) {
    throw new Error(\`request failed: \${response.status} \${await response.text()}\`);
  }
  return response.json();
}`}function bx(i,x){return`export function call${i}(): Promise<unknown> {
  return new Promise((resolve, reject) => {
    const socket = new WebSocket(${JSON.stringify(z.value)});
    socket.addEventListener("open", () => socket.send(${JSON.stringify(JSON.stringify(x))}));
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
}`}function dx(i,x){let p=z.value.replace(/^grpc:\/\//,"");return`// npm i @grpc/grpc-js @grpc/proto-loader
import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";

export async function call${i}(): Promise<unknown> {
  const definition = protoLoader.loadSync("api/api_grpc/proto/api.proto", {
    keepCase: false,
    longs: String,
    enums: String,
    defaults: true,
    oneofs: true,
  });
  const grpcObject = grpc.loadPackageDefinition(definition) as any;
  const Client = grpcObject.project_template_go.api.v1.APIService;
  const client = new Client(${JSON.stringify(p)}, grpc.credentials.createInsecure());
  const request = ${li(JSON.stringify(x,null,2),2).trimStart()};

  return new Promise((resolve, reject) => {
    client.call(request, (error: Error | null, response: unknown) => {
      client.close();
      if (error) reject(error);
      else resolve(response);
    });
  });
}`}function hx(i,x){let p=Hi();switch(b){case"grpc":return dx(p,i);case"websocket":return bx(p,i);default:return fx(p,i,x)}}function A(){Ei.textContent=pi.toUpperCase();try{let i=y(Y.value,"Request body"),x=lx(y(F.value,"Headers"));P.textContent=pi==="go"?ax(i,x):hx(i,x)}catch(i){P.textContent=`// ${D(i)}`}}async function di(i,x){await navigator.clipboard.writeText(i),ti(x)}var si=0;function ti(i){window.clearTimeout(si),e.textContent=i,e.classList.add("is-visible"),si=window.setTimeout(()=>e.classList.remove("is-visible"),1800)}function u(i){try{return`:${new URL(i).port}`}catch{return"—"}}function zi(){l("project-name").textContent=f.project.name,l("project-version").textContent=f.project.version,l("project-bundle").textContent=f.project.bundleId,l("project-mode").textContent=f.project.runMode,l("jsonrpc-port").textContent=u(f.jsonRpcEndpoint),l("mcp-port").textContent=u(f.mcpEndpoint),l("websocket-port").textContent=u(f.webSocketEndpoint),l("grpc-port").textContent=u(f.grpcEndpoint),document.title=`${f.project.name} · API Lab`}async function cx(){try{let i=await fetch("/api/config",{cache:"no-store"});if(!i.ok)throw Error(`HTTP ${i.status}`);let x=await i.json();f={...f,...x,methods:x.methods??f.methods,project:{...f.project,...x.project}},zi(),ni()}catch(i){I(`加载 API 配置失败：${D(i)}`),zi(),ni()}}function V(i,x){return i.querySelector(`:scope > ${x}`)}function H(i,x,p){return Math.min(Math.max(x,i),Math.max(x,p))}function ii(i,x,p,o){let n=window.getComputedStyle(i),t=document.createElement("canvas").getContext("2d"),a=p.length*10;if(t)t.font=n.font,a=Math.max(t.measureText(p).width,...x.map((c)=>t.measureText(c).width));else a=Math.max(p.length,...x.map((c)=>c.length))*10;return Math.ceil(Math.max(o,a+42))}function wx(){window.requestAnimationFrame(()=>{let i=ii(R,q(s.map((d)=>d.domain)),"功能域",92),x=ii(C,q(s.map((d)=>d.group)),"功能组",92),p=ii(S,s.map((d)=>d.functionName),"方法",140),o=xi.get(J);if(!o)return;let n=J.getBoundingClientRect(),t=J.querySelectorAll(":scope > [data-triple-separator]"),a=Array.from(t).reduce((d,w)=>d+w.getBoundingClientRect().width,0),c=Math.max(0,n.width-a),U=Number(J.dataset.minMiddle??390),m=Number(J.dataset.minThird??360),B=i+x+p+14,g=H(B,Number(J.dataset.minFirst??330),c-U-m),k=Math.max(0,c-g),_=Number(J.dataset.middleRatio??0.52),r=H(k*_,U,k-m);o.setDefaultSizes(g,r),xi.get(Oi)?.setDefaultSizes(i,x)})}function kx(){for(let i of document.querySelectorAll("[data-triple-split]")){let x=V(i,'[data-triple-pane="first"]'),p=V(i,'[data-triple-pane="middle"]'),o=V(i,'[data-triple-separator="first"]'),n=V(i,'[data-triple-separator="second"]');if(!x||!p||!o||!n)continue;let t=Number(i.dataset.minFirst??120),a=Number(i.dataset.minMiddle??120),c=Number(i.dataset.minThird??120),U=x.getBoundingClientRect().width,m=p.getBoundingClientRect().width,B=()=>{let r=o.getBoundingClientRect().width+n.getBoundingClientRect().width;return Math.max(0,i.getBoundingClientRect().width-r)},g=(r,d)=>{let w=B(),X=H(r,t,w-a-c),Q=H(d,a,w-X-c);return i.style.setProperty("--triple-first",`${X}px`),i.style.setProperty("--triple-middle",`${Q}px`),o.setAttribute("aria-valuenow",String(Math.round(X/Math.max(w,1)*100))),n.setAttribute("aria-valuenow",String(Math.round((X+Q)/Math.max(w,1)*100))),{first:X,middle:Q}},k=()=>({first:x.getBoundingClientRect().width,middle:p.getBoundingClientRect().width}),_=(r,d)=>{let w=0,X=0,Q=0;r.addEventListener("pointerdown",(v)=>{let $=k();w=v.clientX,X=$.first,Q=$.middle,r.setPointerCapture(v.pointerId),r.classList.add("is-dragging")}),r.addEventListener("pointermove",(v)=>{if(!r.hasPointerCapture(v.pointerId))return;let $=v.clientX-w;if(d==="first"){let K=X+Q,E=H(X+$,t,K-a);g(E,K-E);return}g(X,Q+$)});let hi=(v)=>{if(r.hasPointerCapture(v.pointerId))r.releasePointerCapture(v.pointerId);r.classList.remove("is-dragging")};r.addEventListener("pointerup",hi),r.addEventListener("pointercancel",hi),r.addEventListener("dblclick",()=>g(U,m)),r.addEventListener("keydown",(v)=>{if(v.key!=="ArrowLeft"&&v.key!=="ArrowRight")return;v.preventDefault();let $=v.key==="ArrowRight"?12:-12,K=k();if(d==="first"){let E=K.first+K.middle,ci=H(K.first+$,t,E-a);g(ci,E-ci);return}g(K.first,K.middle+$)})};_(o,"first"),_(n,"second"),xi.set(i,{setDefaultSizes:(r,d)=>{let w=g(r,d);U=w.first,m=w.middle}}),window.requestAnimationFrame(()=>{let r=k();U=r.first,m=r.middle,g(U,m)})}}function vx(){for(let i of document.querySelectorAll("[data-split]")){let x=V(i,'[data-pane="first"]'),p=V(i,'[role="separator"]');if(!x||!p)continue;let o=i.dataset.orientation==="horizontal",n=Number(i.dataset.minFirst??120),t=Number(i.dataset.minSecond??120),a=o?x.getBoundingClientRect().width:x.getBoundingClientRect().height,c=0,U=0,m=(g)=>{let k=i.getBoundingClientRect(),_=o?p.getBoundingClientRect().width:p.getBoundingClientRect().height,r=o?k.width:k.height,d=Math.max(n,r-_-t),w=Math.min(d,Math.max(n,g));i.style.setProperty("--split-first",`${w}px`),p.setAttribute("aria-valuenow",String(Math.round(w/Math.max(r,1)*100)))};p.addEventListener("pointerdown",(g)=>{if(window.matchMedia("(max-width: 760px)").matches)return;c=o?g.clientX:g.clientY,U=o?x.getBoundingClientRect().width:x.getBoundingClientRect().height,p.setPointerCapture(g.pointerId),p.classList.add("is-dragging")}),p.addEventListener("pointermove",(g)=>{if(!p.hasPointerCapture(g.pointerId))return;let k=o?g.clientX:g.clientY;m(U+k-c)});let B=(g)=>{if(p.hasPointerCapture(g.pointerId))p.releasePointerCapture(g.pointerId);p.classList.remove("is-dragging")};p.addEventListener("pointerup",B),p.addEventListener("pointercancel",B),p.addEventListener("dblclick",()=>m(a)),p.addEventListener("keydown",(g)=>{let k=o?x.getBoundingClientRect().width:x.getBoundingClientRect().height,_=o&&g.key==="ArrowLeft"||!o&&g.key==="ArrowUp",r=o&&g.key==="ArrowRight"||!o&&g.key==="ArrowDown";if(!_&&!r)return;g.preventDefault(),m(k+(r?12:-12))}),requestAnimationFrame(()=>{a=o?x.getBoundingClientRect().width:x.getBoundingClientRect().height,m(a)})}}for(let i of document.querySelectorAll(".protocol-tab"))i.addEventListener("click",()=>{ei(i.dataset.protocol)});R.addEventListener("change",()=>_i());C.addEventListener("change",()=>$i());S.addEventListener("change",Ki);Y.addEventListener("input",A);F.addEventListener("input",()=>{Ji(),A()});z.addEventListener("input",A);gi.addEventListener("input",Qi);ri.addEventListener("input",Qi);l("format-request").addEventListener("click",()=>{ix(Y,"Request body"),A()});l("reset-request").addEventListener("click",Ai);T.addEventListener("click",()=>void Gi());l("copy-command").addEventListener("click",()=>{di(nx(),"调用命令已复制")});l("copy-invocation").addEventListener("click",()=>{di(P.textContent??"","调用代码已复制")});for(let i of document.querySelectorAll(".code-language-tab"))i.addEventListener("click",()=>{pi=i.dataset.codeLanguage;for(let x of document.querySelectorAll(".code-language-tab")){let p=x===i;x.classList.toggle("is-active",p),x.setAttribute("aria-selected",String(p))}A()});Xi.addEventListener("click",()=>{if(L)di(W.textContent??"","响应已复制")});l("clear-history").addEventListener("click",()=>{O.length=0,bi()});for(let i of document.querySelectorAll(".response-tab"))i.addEventListener("click",()=>{M=i.dataset.responseTab;for(let x of document.querySelectorAll(".response-tab")){let p=x===i;x.classList.toggle("is-active",p),x.setAttribute("aria-selected",String(p))}yi()});document.addEventListener("keydown",(i)=>{if((i.metaKey||i.ctrlKey)&&i.key==="Enter")i.preventDefault(),Gi()});kx();vx();bi();j.value=oi();cx();
