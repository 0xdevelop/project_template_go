var n=(i)=>{let x=document.getElementById(i);if(!x)throw Error(`Missing element: ${i}`);return x},s=n("endpoint"),Ei=n("transport-method"),R=n("api-domain"),q=n("api-group"),S=n("api-function"),j=n("request-id"),Y=n("request-body"),F=n("request-headers"),_i=n("request-error"),ti=n("test-user-name"),ai=n("test-email"),fi=n("test-phone"),bi=n("test-password"),Wi=n("catalog-count"),$i=n("code-method-label"),Mi=n("code-language-tag"),u=n("invocation-code"),mi=n("headers-count"),N=n("send-request"),Oi=n("response-meta"),Ri=n("response-empty"),W=n("response-code"),Ki=n("copy-response"),zi=n("history-list"),ii=n("toast"),J=n("root-workspace"),Ti=n("catalog-columns"),pi=new WeakMap,b="jsonrpc",f={jsonRpcEndpoint:"http://127.0.0.1:13001",mcpEndpoint:"http://127.0.0.1:13002",webSocketEndpoint:"ws://127.0.0.1:13004",grpcEndpoint:"grpc://127.0.0.1:13005",methods:[],project:{name:"project_template_go",version:"v0.0.2",bundleId:"com.project_template_go.project_template_go",runMode:"Debug"}},z=[],h=null,L=null,M="decoded",oi="go",O=[],Ui="2026-07-28",Ni={jsonrpc:"RPC",mcp:"MCP",websocket:"WS",grpc:"gRPC"};function li(){while(!0){let x=crypto.getRandomValues(new Uint8Array(4)),p=Array.from(x,(o)=>"abcdefghijklmnopqrstuvwxyz0123456789"[o%36]).join("");if(/[a-z]/.test(p)&&/\d/.test(p))return`${Date.now()}${p}`}}function C(i){return[...new Set(i)]}function Pi(i,x,p){let o=i.split(/[./:]+/).filter(Boolean);if(o.length>=3)return{domain:o[0],group:o[1],functionName:o.slice(2).join(".")};if(o.length===2)return{domain:o[0],group:p,functionName:o[1]};return{domain:x,group:p,functionName:o[0]||i}}function ui(){return f.methods.map((i)=>{let x=Pi(i.name,"Core","General");return{id:`api:${i.name}`,...x,rpcMethod:"tools/call",toolName:i.name,description:i.description||"已注册 API 方法",inputSchema:i.inputSchema??{type:"object"},kind:Ci()}})}function Ci(){switch(b){case"mcp":return"MCP TOOL";case"websocket":return"WEBSOCKET";case"grpc":return"gRPC";default:return"JSON-RPC"}}function di(i,x,p){i.replaceChildren(...x.map(({value:l,label:g})=>{let a=document.createElement("option");return a.value=l,a.textContent=g,a}));let o=p&&x.some((l)=>l.value===p)?p:x[0]?.value;if(o)i.value=o}function ni(i){z=ui(),Wi.textContent=`${z.length} functions`,vx();let x=z.find((p)=>p.rpcMethod===i||p.toolName===i)??z[0];di(R,C(z.map((p)=>p.domain)).map((p)=>({value:p,label:p})),x?.domain),ji(x?.group,x?.id)}function ji(i,x){let p=z.filter((o)=>o.domain===R.value);di(q,C(p.map((o)=>o.group)).map((o)=>({value:o,label:o})),i),Qi(x)}function Qi(i){let x=z.filter((p)=>p.domain===R.value&&p.group===q.value);di(S,x.map((p)=>({value:p.id,label:p.functionName})),i),Ai()}function Ai(){if(h=z.find((i)=>i.id===S.value)??z[0]??null,!h){$i.textContent="no function",u.textContent="";return}Gi()}function qi(){return{name:h?.toolName??"",arguments:Si(h?.inputSchema)}}function Si(i){let x=Ji(i);if(x!==null&&!Array.isArray(x)&&typeof x==="object"){let p=x;if(Object.prototype.hasOwnProperty.call(p,"user_name"))p.user_name=ti.value;if(Object.prototype.hasOwnProperty.call(p,"email"))p.email=ai.value;if(Object.prototype.hasOwnProperty.call(p,"phone"))p.phone=fi.value;if(Object.prototype.hasOwnProperty.call(p,"password"))p.password=bi.value;return p}return{}}function Ji(i){if(!i)return{};if("default"in i)return i.default;if("example"in i)return i.example;if(Array.isArray(i.examples)&&i.examples.length>0)return i.examples[0];if("const"in i)return i.const;if(Array.isArray(i.enum)&&i.enum.length>0)return i.enum[0];let x=Array.isArray(i.type)?i.type.find((p)=>p!=="null"):i.type;if(x==="object"||x===void 0&&i.properties!==null&&typeof i.properties==="object"){let p=i.properties!==null&&typeof i.properties==="object"?i.properties:{};return Object.fromEntries(Object.entries(p).map(([o,l])=>[o,Ji(l!==null&&typeof l==="object"?l:void 0)]))}if(x==="array")return[];if(x==="boolean")return!1;if(x==="integer"||x==="number")return 0;if(x==="string")return"";return null}function Ii(){if(b==="websocket"||b==="grpc")return{};let i={"Content-Type":"application/json",Accept:"application/json"};if(b==="mcp")i.Accept="application/json, text/event-stream",i["Mcp-Protocol-Version"]=Ui,i["Mcp-Method"]="tools/call",i["Mcp-Name"]=h?.toolName??"";return i}function ei(i){let x=h?.rpcMethod??"";if(b==="grpc")return{requestId:j.value,method:x,params:i};if(b==="mcp")i={...i,_meta:{"io.modelcontextprotocol/protocolVersion":Ui,"io.modelcontextprotocol/clientInfo":{name:f.project.name,version:f.project.version},"io.modelcontextprotocol/clientCapabilities":{extensions:{}}}};let p={jsonrpc:"2.0",id:j.value,method:x,params:i};if(x==="notifications/initialized")delete p.id;return p}function Gi(){if(!h)return;s.value=ix(),Ei.textContent=b==="websocket"?"WS":b==="grpc"?"RPC":"POST",Y.value=JSON.stringify(ei(qi()),null,2),F.value=JSON.stringify(Ii(),null,2),$i.textContent=h.toolName??h.rpcMethod,Li(),Q(),hi()}function ix(){switch(b){case"mcp":return f.mcpEndpoint;case"websocket":return f.webSocketEndpoint;case"grpc":return f.grpcEndpoint;default:return f.jsonRpcEndpoint}}function xx(i){let x=h?.toolName??h?.rpcMethod;b=i;for(let p of document.querySelectorAll(".protocol-tab")){let o=p.dataset.protocol===b;p.classList.toggle("is-active",o),p.setAttribute("aria-pressed",String(o))}ni(x)}function B(i,x){let p=JSON.parse(i);if(p===null||Array.isArray(p)||typeof p!=="object")throw Error(`${x} 必须是 JSON object`);return p}function I(){try{let i=B(Y.value,"Request body"),x=i.params;if(x===null||Array.isArray(x)||typeof x!=="object")return;let p=x.arguments;if(p===null||Array.isArray(p)||typeof p!=="object")return;let o=p;if(Object.prototype.hasOwnProperty.call(o,"user_name"))o.user_name=ti.value;if(Object.prototype.hasOwnProperty.call(o,"email"))o.email=ai.value;if(Object.prototype.hasOwnProperty.call(o,"phone"))o.phone=fi.value;if(Object.prototype.hasOwnProperty.call(o,"password"))o.password=bi.value;Y.value=JSON.stringify(i,null,2),Q()}catch(i){e(D(i))}}function px(i,x){try{i.value=JSON.stringify(JSON.parse(i.value),null,2),hi()}catch(p){e(`${x} JSON 无效：${D(p)}`)}}function Li(){try{let i=B(F.value,"Headers");mi.textContent=`${Object.keys(i).length} 项`}catch{mi.textContent="格式错误"}}function e(i){_i.textContent=i}function hi(){_i.textContent=""}function D(i){return i instanceof Error?i.message:String(i)}async function Bi(){hi();let i,x;try{i=B(Y.value,"Request body"),x=B(F.value,"Headers")}catch(l){e(D(l));return}if(b==="grpc")j.value=li(),i.requestId=j.value,delete i.id;else if(i.method==="notifications/initialized")delete i.id;else j.value=li(),i.id=j.value;Y.value=JSON.stringify(i,null,2),Q();let p={};for(let[l,g]of Object.entries(x))p[l]=String(g);si(!0);let o=performance.now();try{let l=await fetch("/api/proxy",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({transport:b,url:s.value,method:"POST",headers:p,body:JSON.stringify(i)})}),g=await l.json();if(!l.ok||"error"in g){Yi(l.status,l.statusText,"error"in g?g.error:`HTTP ${l.status}`,Math.round(performance.now()-o)),ri("通信失败");return}L=g,Vi(g),yi(g)}catch(l){Yi(0,"Communication Error",D(l),Math.round(performance.now()-o)),ri("通信失败")}finally{si(!1)}}function si(i){N.disabled=i,N.classList.toggle("is-loading",i);let x=N.querySelector("span");if(x)x.textContent=i?"请求中…":"请求"}function Vi(i){Ri.classList.add("is-hidden"),W.classList.remove("is-hidden"),Ki.disabled=!1;let x=i.errorKind==="communication",p=i.protocolError===!0,o=!x&&!p&&lx(i.body),l=!x&&!p&&ox(i.status)&&!o,g=p?i.statusText:x?`${i.status||"—"} COMM`:`${i.status}${o?" RPC":""}`;Oi.innerHTML=`
    <span class="status-pill ${l?"is-success":"is-error"}">
      ${Di(g)}
    </span>
    <span>${i.durationMs} ms</span>
    <span>${nx(i.size)}${i.wasTruncated?" · truncated":""}</span>
  `,Hi()}function Yi(i,x,p,o){let l=JSON.stringify({error:p},null,2),g={status:i,statusText:x,headers:{},body:l,durationMs:o,size:new TextEncoder().encode(l).length,wasTruncated:!1,errorKind:"communication"};M="decoded";for(let a of document.querySelectorAll(".response-tab")){let w=a.dataset.responseTab===M;a.classList.toggle("is-active",w),a.setAttribute("aria-selected",String(w))}L=g,Vi(g),yi(g)}function ox(i){return i>=200&&i<300||b==="websocket"&&i===101}function lx(i){try{let x=JSON.parse(i);return x!==null&&typeof x==="object"&&x.error!=null}catch{return!1}}function Hi(){if(!L)return;if(M==="headers"){W.textContent=JSON.stringify(L.headers,null,2);return}try{let i=JSON.parse(L.body);W.textContent=JSON.stringify(M==="decoded"?P(i):i,null,2)}catch{W.textContent=L.body||"(empty response body)"}}function P(i){if(Array.isArray(i))return i.map(P);if(i!==null&&typeof i==="object")return Object.fromEntries(Object.entries(i).map(([l,g])=>[l,P(g)]));if(typeof i!=="string")return i;let x=i.trim(),p=x.startsWith("{")&&x.endsWith("}"),o=x.startsWith("[")&&x.endsWith("]");if(!p&&!o)return i;try{return P(JSON.parse(x))}catch{return i}}function nx(i){if(i<1024)return`${i} B`;return`${(i/1024).toFixed(1)} KB`}function yi(i){O.unshift({method:h?.toolName??h?.rpcMethod??"unknown",protocol:b,status:i.status,duration:i.durationMs}),O.splice(6),wi()}function wi(){if(O.length===0){let i=document.createElement("li");i.className="history-empty",i.textContent="暂无请求记录",zi.replaceChildren(i);return}zi.replaceChildren(...O.map((i)=>{let x=document.createElement("li");return x.innerHTML=`
        <span class="history-protocol">${Ni[i.protocol]}</span>
        <strong>${Di(i.method)}</strong>
        <span class="${i.status>=200&&i.status<300||i.protocol==="websocket"&&i.status===101?"history-ok":"history-failed"}">${i.status}</span>
        <small>${i.duration} ms</small>
      `,x}))}function Di(i){return i.replace(/[&<>"']/g,(x)=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#039;"})[x]??x)}function gx(){let i={};try{i=B(F.value,"Headers")}catch{}let x=Object.entries(i).map(([o,l])=>`-H ${G(`${o}: ${String(l)}`)}`).join(" \\\n  ");if(b==="websocket")return[`printf '%s' ${G(Y.value)}`,`websocat -1 ${G(s.value)}`].join(" | ");if(b==="grpc"){let o=s.value.replace(/^grpc:\/\//,"");return["grpcurl -plaintext","-import-path gcs_api/api_grpc/proto","-proto api.proto",`-d ${G(Y.value)}`,G(o),"project_template_go.api.v1.APIService/Call"].join(" ")}return[`curl -X POST ${G(s.value)}`,x,`--data-raw ${G(Y.value)}`].filter(Boolean).join(" \\\n  ")}function G(i){return`'${i.replaceAll("'","'\\''")}'`}function Fi(){let p=(h?.toolName??h?.functionName??"api").split(/[^a-zA-Z0-9]+/).filter(Boolean).map((o)=>o.charAt(0).toUpperCase()+o.slice(1)).join("");return/^\d/.test(p)?`Method${p}`:p||"API"}function gi(i,x){let p=" ".repeat(x);return i.split(`
`).map((o)=>`${p}${o}`).join(`
`)}function _(i){return JSON.stringify(i)}function rx(i){return Object.fromEntries(Object.entries(i).map(([x,p])=>[x,String(p)]))}function tx(i,x,p){let o=Object.entries(p).map(([l,g])=>`	request.Header.Set(${_(l)}, ${_(g)})`).join(`
`);return`// imports: bytes, context, fmt, io, net/http
func Call${i}(ctx context.Context) ([]byte, error) {
	body := []byte(${_(JSON.stringify(x))})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, ${_(s.value)}, bytes.NewReader(body))
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
}`}function ax(i,x){return`// import: context, github.com/coder/websocket
func Call${i}(ctx context.Context) ([]byte, error) {
	connection, _, err := websocket.Dial(ctx, ${_(s.value)}, nil)
	if err != nil {
		return nil, err
	}
	defer connection.CloseNow()

	if err := connection.Write(ctx, websocket.MessageText, []byte(${_(JSON.stringify(x))})); err != nil {
		return nil, err
	}
	_, response, err := connection.Read(ctx)
	if err != nil {
		return nil, err
	}
	return response, connection.Close(websocket.StatusNormalClosure, "")
}`}function fx(i,x){let p=x.params!==null&&!Array.isArray(x.params)&&typeof x.params==="object"?x.params:{},o=s.value.replace(/^grpc:\/\//,""),l=String(x.requestId??j.value),g=String(x.method??h?.rpcMethod??"tools/call");return`// imports: context, encoding/json, api_grpc_protobuf, grpc, insecure, structpb
func Call${i}(ctx context.Context) (*api_grpc_protobuf.CallResponse, error) {
	connection, err := grpc.NewClient(${_(o)}, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer connection.Close()

	var params map[string]any
	if err := json.Unmarshal([]byte(${_(JSON.stringify(p))}), &params); err != nil {
		return nil, err
	}
	paramsValue, err := structpb.NewStruct(params)
	if err != nil {
		return nil, err
	}

	return api_grpc_protobuf.NewAPIServiceClient(connection).Call(ctx, &api_grpc_protobuf.CallRequest{
		RequestId: ${_(l)},
		Method:    ${_(g)},
		Params:    paramsValue,
	})
}`}function bx(i,x){let p=Fi();switch(b){case"grpc":return fx(p,i);case"websocket":return ax(p,i);default:return tx(p,i,x)}}function dx(i,x,p){return`export async function call${i}(): Promise<unknown> {
  const response = await fetch(${JSON.stringify(s.value)}, {
    method: "POST",
    headers: ${gi(JSON.stringify(p,null,2),4).trimStart()},
    body: JSON.stringify(${gi(JSON.stringify(x,null,2),4).trimStart()}),
  });
  if (!response.ok) {
    throw new Error(\`request failed: \${response.status} \${await response.text()}\`);
  }
  return response.json();
}`}function hx(i,x){return`export function call${i}(): Promise<unknown> {
  return new Promise((resolve, reject) => {
    const socket = new WebSocket(${JSON.stringify(s.value)});
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
}`}function wx(i,x){let p=s.value.replace(/^grpc:\/\//,"");return`// npm i @grpc/grpc-js @grpc/proto-loader
import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";

export async function call${i}(): Promise<unknown> {
  const definition = protoLoader.loadSync("gcs_api/api_grpc/proto/api.proto", {
    keepCase: false,
    longs: String,
    enums: String,
    defaults: true,
    oneofs: true,
  });
  const grpcObject = grpc.loadPackageDefinition(definition) as any;
  const Client = grpcObject.project_template_go.api.v1.APIService;
  const client = new Client(${JSON.stringify(p)}, grpc.credentials.createInsecure());
  const request = ${gi(JSON.stringify(x,null,2),2).trimStart()};

  return new Promise((resolve, reject) => {
    client.call(request, (error: Error | null, response: unknown) => {
      client.close();
      if (error) reject(error);
      else resolve(response);
    });
  });
}`}function cx(i,x){let p=Fi();switch(b){case"grpc":return wx(p,i);case"websocket":return hx(p,i);default:return dx(p,i,x)}}function Q(){Mi.textContent=oi.toUpperCase();try{let i=B(Y.value,"Request body"),x=rx(B(F.value,"Headers"));u.textContent=oi==="go"?bx(i,x):cx(i,x)}catch(i){u.textContent=`// ${D(i)}`}}async function ci(i,x){await navigator.clipboard.writeText(i),ri(x)}var Xi=0;function ri(i){window.clearTimeout(Xi),ii.textContent=i,ii.classList.add("is-visible"),Xi=window.setTimeout(()=>ii.classList.remove("is-visible"),1800)}function T(i){try{return`:${new URL(i).port}`}catch{return"—"}}function Zi(){n("project-name").textContent=f.project.name,n("project-version").textContent=f.project.version,n("project-bundle").textContent=f.project.bundleId,n("project-mode").textContent=f.project.runMode,n("jsonrpc-port").textContent=T(f.jsonRpcEndpoint),n("mcp-port").textContent=T(f.mcpEndpoint),n("websocket-port").textContent=T(f.webSocketEndpoint),n("grpc-port").textContent=T(f.grpcEndpoint),document.title=`${f.project.name} · API Lab`}async function kx(){try{let i=await fetch("/api/config",{cache:"no-store"});if(!i.ok)throw Error(`HTTP ${i.status}`);let x=await i.json();f={...f,...x,methods:x.methods??f.methods,project:{...f.project,...x.project}},Zi(),ni()}catch(i){e(`加载 API 配置失败：${D(i)}`),Zi(),ni()}}function H(i,x){return i.querySelector(`:scope > ${x}`)}function y(i,x,p){return Math.min(Math.max(x,i),Math.max(x,p))}function xi(i,x,p,o){let l=window.getComputedStyle(i),g=document.createElement("canvas").getContext("2d"),a=p.length*10;if(g)g.font=l.font,a=Math.max(g.measureText(p).width,...x.map((w)=>g.measureText(w).width));else a=Math.max(p.length,...x.map((w)=>w.length))*10;return Math.ceil(Math.max(o,a+42))}function vx(){window.requestAnimationFrame(()=>{let i=xi(R,C(z.map((d)=>d.domain)),"功能域",92),x=xi(q,C(z.map((d)=>d.group)),"功能组",92),p=xi(S,z.map((d)=>d.functionName),"方法",140),o=pi.get(J);if(!o)return;let l=J.getBoundingClientRect(),g=J.querySelectorAll(":scope > [data-triple-separator]"),a=Array.from(g).reduce((d,c)=>d+c.getBoundingClientRect().width,0),w=Math.max(0,l.width-a),X=Number(J.dataset.minMiddle??390),m=Number(J.dataset.minThird??360),V=i+x+p+14,r=y(V,Number(J.dataset.minFirst??330),w-X-m),k=Math.max(0,w-r),$=Number(J.dataset.middleRatio??0.52),t=y(k*$,X,k-m);o.setDefaultSizes(r,t),pi.get(Ti)?.setDefaultSizes(i,x)})}function mx(){for(let i of document.querySelectorAll("[data-triple-split]")){let x=H(i,'[data-triple-pane="first"]'),p=H(i,'[data-triple-pane="middle"]'),o=H(i,'[data-triple-separator="first"]'),l=H(i,'[data-triple-separator="second"]');if(!x||!p||!o||!l)continue;let g=Number(i.dataset.minFirst??120),a=Number(i.dataset.minMiddle??120),w=Number(i.dataset.minThird??120),X=x.getBoundingClientRect().width,m=p.getBoundingClientRect().width,V=()=>{let t=o.getBoundingClientRect().width+l.getBoundingClientRect().width;return Math.max(0,i.getBoundingClientRect().width-t)},r=(t,d)=>{let c=V(),Z=y(t,g,c-a-w),A=y(d,a,c-Z-w);return i.style.setProperty("--triple-first",`${Z}px`),i.style.setProperty("--triple-middle",`${A}px`),o.setAttribute("aria-valuenow",String(Math.round(Z/Math.max(c,1)*100))),l.setAttribute("aria-valuenow",String(Math.round((Z+A)/Math.max(c,1)*100))),{first:Z,middle:A}},k=()=>({first:x.getBoundingClientRect().width,middle:p.getBoundingClientRect().width}),$=(t,d)=>{let c=0,Z=0,A=0;t.addEventListener("pointerdown",(v)=>{let K=k();c=v.clientX,Z=K.first,A=K.middle,t.setPointerCapture(v.pointerId),t.classList.add("is-dragging")}),t.addEventListener("pointermove",(v)=>{if(!t.hasPointerCapture(v.pointerId))return;let K=v.clientX-c;if(d==="first"){let U=Z+A,E=y(Z+K,g,U-a);r(E,U-E);return}r(Z,A+K)});let ki=(v)=>{if(t.hasPointerCapture(v.pointerId))t.releasePointerCapture(v.pointerId);t.classList.remove("is-dragging")};t.addEventListener("pointerup",ki),t.addEventListener("pointercancel",ki),t.addEventListener("dblclick",()=>r(X,m)),t.addEventListener("keydown",(v)=>{if(v.key!=="ArrowLeft"&&v.key!=="ArrowRight")return;v.preventDefault();let K=v.key==="ArrowRight"?12:-12,U=k();if(d==="first"){let E=U.first+U.middle,vi=y(U.first+K,g,E-a);r(vi,E-vi);return}r(U.first,U.middle+K)})};$(o,"first"),$(l,"second"),pi.set(i,{setDefaultSizes:(t,d)=>{let c=r(t,d);X=c.first,m=c.middle}}),window.requestAnimationFrame(()=>{let t=k();X=t.first,m=t.middle,r(X,m)})}}function zx(){for(let i of document.querySelectorAll("[data-split]")){let x=H(i,'[data-pane="first"]'),p=H(i,'[role="separator"]');if(!x||!p)continue;let o=i.dataset.orientation==="horizontal",l=Number(i.dataset.minFirst??120),g=Number(i.dataset.minSecond??120),a=o?x.getBoundingClientRect().width:x.getBoundingClientRect().height,w=0,X=0,m=(r)=>{let k=i.getBoundingClientRect(),$=o?p.getBoundingClientRect().width:p.getBoundingClientRect().height,t=o?k.width:k.height,d=Math.max(l,t-$-g),c=Math.min(d,Math.max(l,r));i.style.setProperty("--split-first",`${c}px`),p.setAttribute("aria-valuenow",String(Math.round(c/Math.max(t,1)*100)))};p.addEventListener("pointerdown",(r)=>{if(window.matchMedia("(max-width: 760px)").matches)return;w=o?r.clientX:r.clientY,X=o?x.getBoundingClientRect().width:x.getBoundingClientRect().height,p.setPointerCapture(r.pointerId),p.classList.add("is-dragging")}),p.addEventListener("pointermove",(r)=>{if(!p.hasPointerCapture(r.pointerId))return;let k=o?r.clientX:r.clientY;m(X+k-w)});let V=(r)=>{if(p.hasPointerCapture(r.pointerId))p.releasePointerCapture(r.pointerId);p.classList.remove("is-dragging")};p.addEventListener("pointerup",V),p.addEventListener("pointercancel",V),p.addEventListener("dblclick",()=>m(a)),p.addEventListener("keydown",(r)=>{let k=o?x.getBoundingClientRect().width:x.getBoundingClientRect().height,$=o&&r.key==="ArrowLeft"||!o&&r.key==="ArrowUp",t=o&&r.key==="ArrowRight"||!o&&r.key==="ArrowDown";if(!$&&!t)return;r.preventDefault(),m(k+(t?12:-12))}),requestAnimationFrame(()=>{a=o?x.getBoundingClientRect().width:x.getBoundingClientRect().height,m(a)})}}for(let i of document.querySelectorAll(".protocol-tab"))i.addEventListener("click",()=>{xx(i.dataset.protocol)});R.addEventListener("change",()=>ji());q.addEventListener("change",()=>Qi());S.addEventListener("change",Ai);Y.addEventListener("input",Q);F.addEventListener("input",()=>{Li(),Q()});s.addEventListener("input",Q);ti.addEventListener("input",I);ai.addEventListener("input",I);fi.addEventListener("input",I);bi.addEventListener("input",I);n("format-request").addEventListener("click",()=>{px(Y,"Request body"),Q()});n("reset-request").addEventListener("click",Gi);N.addEventListener("click",()=>void Bi());n("copy-command").addEventListener("click",()=>{ci(gx(),"调用命令已复制")});n("copy-invocation").addEventListener("click",()=>{ci(u.textContent??"","调用代码已复制")});for(let i of document.querySelectorAll(".code-language-tab"))i.addEventListener("click",()=>{oi=i.dataset.codeLanguage;for(let x of document.querySelectorAll(".code-language-tab")){let p=x===i;x.classList.toggle("is-active",p),x.setAttribute("aria-selected",String(p))}Q()});Ki.addEventListener("click",()=>{if(L)ci(W.textContent??"","响应已复制")});n("clear-history").addEventListener("click",()=>{O.length=0,wi()});for(let i of document.querySelectorAll(".response-tab"))i.addEventListener("click",()=>{M=i.dataset.responseTab;for(let x of document.querySelectorAll(".response-tab")){let p=x===i;x.classList.toggle("is-active",p),x.setAttribute("aria-selected",String(p))}Hi()});document.addEventListener("keydown",(i)=>{if((i.metaKey||i.ctrlKey)&&i.key==="Enter")i.preventDefault(),Bi()});mx();zx();wi();j.value=li();kx();
