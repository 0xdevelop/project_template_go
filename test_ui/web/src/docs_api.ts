// docs_api 子功能页：渲染由 test_ui 服务在 /docs_api 页面内注入的 API 方法文档；
// 文档唯一事实源在仓库 docs/ 目录，对外不提供任何原始文件路由。
import "./docs_api.css";

interface TocItem {
  level: number;
  id: string;
  text: string;
}

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function renderInline(value: string): string {
  let out = "";
  // 先切出行内 code span，避免其内容被加粗/链接规则误伤
  const parts = value.split(/(`[^`]+`)/);
  for (const part of parts) {
    if (part.length > 1 && part.startsWith("`") && part.endsWith("`")) {
      out += `<code>${escapeHtml(part.slice(1, -1))}</code>`;
    } else {
      let text = escapeHtml(part);
      text = text.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
      text = text.replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, '<a href="$2">$1</a>');
      out += text;
    }
  }
  return out;
}

function slugify(text: string, used: Record<string, boolean>): string {
  let id = text
    .trim()
    .replace(/`/g, "")
    .replace(/\s+/g, "-")
    .replace(/[#?&%"'<>]/g, "");
  const base = id || "section";
  let sequence = 1;
  id = base;
  while (used[id]) {
    sequence += 1;
    id = `${base}-${sequence}`;
  }
  used[id] = true;
  return id;
}

function renderMarkdown(markdown: string): { html: string; toc: TocItem[] } {
  const lines = markdown.replace(/\r\n/g, "\n").split("\n");
  const html: string[] = [];
  const toc: TocItem[] = [];
  const usedIds: Record<string, boolean> = {};
  let index = 0;
  const total = lines.length;
  const paragraph: string[] = [];

  const flushParagraph = () => {
    if (paragraph.length) {
      html.push(`<p>${paragraph.map(renderInline).join("<br>")}</p>`);
      paragraph.length = 0;
    }
  };

  while (index < total) {
    const line = lines[index];

    const fence = line.match(/^```(\w*)\s*$/);
    if (fence) {
      flushParagraph();
      const code: string[] = [];
      index += 1;
      while (index < total && !/^```\s*$/.test(lines[index])) {
        code.push(lines[index]);
        index += 1;
      }
      index += 1;
      html.push(
        `<pre><code class="lang-${fence[1]}">${escapeHtml(code.join("\n"))}</code></pre>`,
      );
      continue;
    }

    const heading = line.match(/^(#{1,4})\s+(.*)$/);
    if (heading) {
      flushParagraph();
      const level = heading[1].length;
      const text = heading[2].trim();
      const id = slugify(text.replace(/[*_]/g, ""), usedIds);
      html.push(`<h${level} id="${id}">${renderInline(text)}</h${level}>`);
      // H1 = 功能域/顶层节，H2 = 域内方法：两级构成左侧分类树
      if (level === 1 || level === 2) {
        toc.push({ level, id, text: text.replace(/`/g, "") });
      }
      index += 1;
      continue;
    }

    if (
      line.includes("|") &&
      index + 1 < total &&
      /^\s*\|?[\s:|-]+\|[\s:|-]*$/.test(lines[index + 1])
    ) {
      flushParagraph();
      const cells = (row: string): string[] =>
        row
          .replace(/^\s*\|/, "")
          .replace(/\|\s*$/, "")
          .split("|")
          .map((cell) => renderInline(cell.trim()));
      const thead = `<thead><tr>${cells(line)
        .map((cell) => `<th>${cell}</th>`)
        .join("")}</tr></thead>`;
      index += 2;
      const body: string[] = [];
      while (
        index < total &&
        lines[index].includes("|") &&
        lines[index].trim() !== ""
      ) {
        body.push(
          `<tr>${cells(lines[index])
            .map((cell) => `<td>${cell}</td>`)
            .join("")}</tr>`,
        );
        index += 1;
      }
      html.push(`<table>${thead}<tbody>${body.join("")}</tbody></table>`);
      continue;
    }

    const listItem = line.match(/^(\s*)([-*]|\d+\.)\s+(.*)$/);
    if (listItem) {
      flushParagraph();
      const ordered = /^\d+\.$/.test(listItem[2]);
      const items: string[] = [];
      while (index < total) {
        const matched = lines[index].match(/^(\s*)([-*]|\d+\.)\s+(.*)$/);
        if (matched) {
          items.push(matched[3]);
          index += 1;
          continue;
        }
        if (/^\s{2,}\S/.test(lines[index]) && items.length) {
          items[items.length - 1] += ` ${lines[index].trim()}`;
          index += 1;
          continue;
        }
        break;
      }
      const tag = ordered ? "ol" : "ul";
      html.push(
        `<${tag}>${items.map((item) => `<li>${renderInline(item)}</li>`).join("")}</${tag}>`,
      );
      continue;
    }

    if (/^>\s?/.test(line)) {
      flushParagraph();
      const quoted: string[] = [];
      while (index < total && /^>\s?/.test(lines[index])) {
        quoted.push(lines[index].replace(/^>\s?/, ""));
        index += 1;
      }
      html.push(
        `<blockquote><p>${quoted.map(renderInline).join("<br>")}</p></blockquote>`,
      );
      continue;
    }

    if (line.trim() === "") {
      flushParagraph();
      index += 1;
      continue;
    }

    paragraph.push(line);
    index += 1;
  }
  flushParagraph();
  return { html: html.join("\n"), toc };
}

// buildToc 构建左侧分类树：H1 为父节点（有方法子节点时可折叠，点击开合；无子节点时直接跳转），
// H2 方法为缩进子节点。
function buildToc(toc: TocItem[]): void {
  const nav = document.getElementById("toc");
  if (!nav) {
    return;
  }
  const out = ['<div class="toc-title">目录</div>'];
  let groupOpen = false;
  const closeGroup = () => {
    if (groupOpen) {
      out.push("</details>");
      groupOpen = false;
    }
  };
  for (let index = 0; index < toc.length; index += 1) {
    const item = toc[index];
    if (item.level === 1) {
      closeGroup();
      const hasChildren = index + 1 < toc.length && toc[index + 1].level === 2;
      if (hasChildren) {
        out.push(
          `<details class="toc-group" open><summary>${escapeHtml(item.text)}</summary>`,
        );
        groupOpen = true;
      } else {
        out.push(
          `<a class="toc-root" href="#${item.id}">${escapeHtml(item.text)}</a>`,
        );
      }
    } else {
      out.push(
        `<a class="toc-child" href="#${item.id}">${escapeHtml(item.text)}</a>`,
      );
    }
  }
  closeGroup();
  nav.innerHTML = out.join("");
}

function loadDoc(): void {
  const markdown = (window as { __API_DOC_SOURCE__?: unknown }).__API_DOC_SOURCE__;
  // 无注入内容时保持空白页，与 API 侧 Home 空响应同一姿态，不输出任何提示。
  if (typeof markdown !== "string" || markdown === "") {
    return;
  }
  const rendered = renderMarkdown(markdown);
  const doc = document.getElementById("doc");
  if (doc) {
    doc.innerHTML = rendered.html;
  }
  buildToc(rendered.toc);
  if (location.hash) {
    document.getElementById(location.hash.slice(1))?.scrollIntoView();
  }
}

loadDoc();
