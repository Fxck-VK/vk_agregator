import {existsSync, mkdirSync, readFileSync, readdirSync, writeFileSync} from "node:fs";
import {dirname, relative, resolve} from "node:path";
import {fileURLToPath} from "node:url";
import {sourceLoader} from "./content-source-loader.mjs";

export const platformRoot = fileURLToPath(new URL("..", import.meta.url));
const groupOrder = ["Платформа", "Авторизация", "Основные страницы", "Юридические документы", "Академия", "Справка", "Публичные страницы", "Разработка"];
const localizedPath = path => "/ru" + (path === "/" ? "" : path);
const readJson = path => JSON.parse(readFileSync(path, "utf8"));

function discoverPages(root) {
  const pages = [];
  function visit(directory, segments, preview) {
    for (const entry of readdirSync(directory, {withFileTypes: true})) {
      const file = resolve(directory, entry.name);
      if (entry.isDirectory() && !entry.name.startsWith("_") && !entry.name.startsWith("@")) {
        const group = /^\(.+\)$/.test(entry.name);
        visit(file, group ? segments : [...segments, entry.name], preview || entry.name === "(landing-preview)");
      } else if (entry.isFile() && /^page\.[jt]sx?$/.test(entry.name)) {
        pages.push({path: "/" + segments.join("/"), source: relative(root, file).replaceAll("\\", "/"), preview});
      }
    }
  }
  visit(resolve(root, "src/app"), [], false);
  return pages.sort((a, b) => a.path.localeCompare(b.path, "en"));
}

function matches(pattern, path) {
  const patternParts = pattern.split("/");
  const parts = path.split("/");
  return patternParts.length === parts.length && patternParts.every((part, index) =>
    part === parts[index] || /^\[[a-zA-Z0-9_]+\]$/.test(part));
}

export function collectPageInventory(root = platformRoot, now = Date.now()) {
  const load = sourceLoader(root);
  const {allowsReleasePath, releaseFeatures} = load(resolve(root, "src/features/release/policy.ts"));
  const {contentPages, expandContentPages, canReadContent} = load(resolve(root, "src/content/templates/catalog.ts"));
  const labels = readJson(resolve(root, "release/page-labels.json"));
  const sourcePages = discoverPages(root);
  const sourcePaths = new Set(sourcePages.map(page => page.path));
  if (sourcePaths.size !== sourcePages.length) throw new Error("Duplicate implemented route.");
  for (const path of Object.keys(labels)) {
    if (!sourcePaths.has(path)) throw new Error("Page label has no implemented route: " + path);
  }

  const pages = [];
  const covered = new Set();
  const redirects = [];
  const excluded = [];
  function add(page, production) {
    pages.push({...page, view: production ? "prod" : "dev", parameter: /\[/.test(page.path)});
  }
  for (const page of expandContentPages(contentPages)) {
    const owner = sourcePages.find(source => source.path === page.path)
      ?? sourcePages.find(source => matches(source.path, page.path))
      ?? sourcePages.find(source => source.path === "/[...missing]");
    if (!owner) throw new Error("Content has no implemented route: " + page.path);
    covered.add(owner.path);
    const parent = contentPages.find(parent => parent.id === page.id.split("--")[0]) ?? page;
    if (!canReadContent(parent, {preview: true, now})) {
      excluded.push({...owner, reason: "Запись контента сейчас закрыта правилами публикации"});
      continue;
    }
    const production = canReadContent(parent, {preview: false, now})
      && allowsReleasePath(page.path, "prod", now)
      && (page.template === "T23" || page.status === "published");
    const group = {T01: "Основные страницы", T10: "Академия", T11: "Справка", T23: "Юридические документы"}[page.template] ?? "Публичные страницы";
    add({
      path: page.path,
      title: page.path === "/" ? "Главная" : page.seo.title,
      group,
      source: owner.source,
      reason: production ? page.template === "T23" ? "Публичный правовой раздел" : "Опубликована"
        : page.status === "draft" ? "Черновик" : "Ограничение публикации",
    }, production);
  }

  const legalOwner = sourcePages.find(page => page.path === "/legal/[slug]");
  if (!legalOwner) throw new Error("Legal document route is missing.");
  const legal = readJson(resolve(root, "legal/catalog.json"));
  for (const document of legal.documents) {
    covered.add(legalOwner.path);
    add({
      path: "/legal/" + document.slug,
      title: document.title,
      group: "Юридические документы",
      source: legalOwner.source,
      reason: "Публичный правовой документ",
    }, allowsReleasePath("/legal/" + document.slug, "prod", now));
  }

  for (const source of sourcePages) {
    if (covered.has(source.path)) continue;
    const label = labels[source.path] ?? {};
    if (label.omit) {
      excluded.push({...source, reason: label.omit});
      continue;
    }
    if (label.redirects) {
      redirects.push({...source, title: label.title, targets: label.redirects});
      continue;
    }
    const production = !source.preview && allowsReleasePath(source.path, "prod", now);
    const blockedFeatures = releaseFeatures.filter(feature => !feature.production
      && (source.path === feature.path || source.path.startsWith(feature.path + "/")));
    add({
      ...source,
      title: label.title ?? localizedPath(source.path),
      group: label.group ?? (source.path.startsWith("/app") ? "Платформа" : "Основные страницы"),
      note: label.note,
      reason: production ? "Допущена правилами релиза"
        : source.preview ? "Защищённый предпросмотр DEV"
        : blockedFeatures.length ? "Функция только в DEV" : "Страница разработки",
    }, production);
  }

  const paths = new Set();
  for (const page of pages) {
    if (paths.has(page.path)) throw new Error("Duplicate page in release inventory: " + page.path);
    paths.add(page.path);
  }
  pages.sort((a, b) => groupOrder.indexOf(a.group) - groupOrder.indexOf(b.group) || a.path.localeCompare(b.path, "en"));
  return {prod: pages.filter(page => page.view === "prod"), dev: pages.filter(page => page.view === "dev"), redirects, excluded};
}

function counts(pages) {
  const parameter = pages.filter(page => page.parameter).length;
  return `Адресов без параметров — ${pages.length - parameter}, с параметрами — ${parameter}`;
}

export function renderMarkdown(inventory, view) {
  const pages = inventory[view];
  const title = view === "prod" ? "Страницы PROD" : "Страницы только DEV";
  const lines = [
    "<!-- Generated by scripts/release-page-inventory.mjs. Run npm --prefix web/platform run release:pages. -->",
    "# " + title, "",
    counts(pages) + ".", "",
    "Состав определяется текущим кодом и правилами допуска. Страницы PROD доступны также в DEV.", "",
    "[PROD](pages-prod.md) · [Только DEV](pages-dev.md)", "",
  ];
  for (const group of new Set([...groupOrder, ...pages.map(page => page.group)])) {
    const entries = pages.filter(page => page.group === group);
    if (!entries.length) continue;
    lines.push("## " + group, "", "| Страница | Адрес | Основание |", "| --- | --- | --- |");
    for (const page of entries) {
      const cell = value => String(value).replaceAll("|", "\\|").replaceAll("\n", " ");
      lines.push("| " + cell(page.title) + " | `" + localizedPath(page.path) + "` | " + cell(page.note ?? page.reason) + " |");
    }
    lines.push("");
  }
  lines.push(
    "## Обновление списков", "",
    "Изменения маршрутов обнаруживаются в `src/app`. Названия служебных страниц задаются в `release/page-labels.json`.",
    "Допуск берётся из `src/features/release/policy.ts`, `release/features.json` и каталога `src/content/pages`.",
    "Уроки, статьи справки и юридические документы разворачиваются из существующих каталогов.", "",
    "Обновить оба списка и HTML для браузера — `npm --prefix web/platform run release:pages`.",
    "Проверить актуальность — `npm --prefix web/platform run release:pages:check`.",
    "Проверка также входит в `public-pages:generate` перед тестами, проверкой типов и сборкой.", "",
    "Списки отражают допуск в релиз. Состояние выкладки проверяется по отдельной процедуре релиза.", "",
    "## Переадресации", "",
  );
  for (const redirect of inventory.redirects) {
    lines.push("- `" + localizedPath(redirect.path) + "` → " + redirect.targets.map(path => "`" + localizedPath(path) + "`").join(" или ") + ".");
  }
  return lines.join("\n") + "\n";
}

const escapeHtml = value => String(value).replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;");

export function renderHtml(inventory, view) {
  const pages = inventory[view];
  const title = view === "prod" ? "Страницы PROD" : "Страницы только DEV";
  const sections = [...new Set([...groupOrder, ...pages.map(page => page.group)])].map(group => {
    const entries = pages.filter(page => page.group === group);
    if (!entries.length) return "";
    const rows = entries.map(page => {
      const path = localizedPath(page.path);
      const description = page.note ?? page.reason;
      const name = `<span class="name">${escapeHtml(page.title)}</span><span class="path">${escapeHtml(path)}</span>`;
      const body = page.parameter ? `<div class="entry">${name}<span class="path">${escapeHtml(description)}</span></div>`
        : `<a class="entry" href="http://localhost:7159${escapeHtml(path)}" target="_blank" rel="noopener">${name}</a>`;
      return `<li data-search="${escapeHtml((page.title + " " + path).toLocaleLowerCase("ru"))}">${body}<span class="state">${escapeHtml(page.reason)}</span></li>`;
    }).join("");
    return `<section><h2>${escapeHtml(group)} <span>${entries.length}</span></h2><ul>${rows}</ul></section>`;
  }).join("");
  return `<!doctype html>
<html lang="ru"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>${title} · Epifera</title>
<style>
*{box-sizing:border-box}body{margin:0;background:#0d1117;color:#f0f6fc;font:16px/1.5 Inter,Arial,sans-serif}
main{max-width:1160px;margin:auto;padding:40px 28px 64px}h1{font-size:clamp(28px,4vw,40px);line-height:1.2;margin:28px 0 12px}
p{color:#9198a1;margin:0 0 20px}nav{display:flex;gap:12px;flex-wrap:wrap}nav a{padding:10px 18px;border:1px solid #30363d;border-radius:8px;color:inherit;text-decoration:none}
nav a[aria-current]{color:#9a7cf5;border-color:#9a7cf5;background:#151b23}
.tools{display:flex;align-items:center;gap:16px;flex-wrap:wrap;margin-bottom:32px}input{flex:1;min-width:0;padding:13px 16px;border:1px solid #30363d;border-radius:10px;color:inherit;background:#151b23;font:inherit}
a:focus-visible,input:focus-visible{outline:2px solid #9a7cf5;outline-offset:3px}#count{color:#9198a1;font-size:14px}
section{margin:30px 0}h2{font-size:20px;margin:0 0 12px}h2 span{color:#9198a1;font-size:14px;margin-left:8px}
ul{list-style:none;padding:0;margin:0;border:1px solid #30363d;border-radius:12px;overflow:hidden}
li{display:flex;gap:12px;align-items:center;background:#151b23;padding-right:18px}li+li{border-top:1px solid #30363d}
.entry{flex:1;min-width:0;padding:15px 18px;color:inherit;text-decoration:none}.entry:hover .name{color:#9a7cf5}
.name,.path{display:block}.path{font-size:13px;color:#9198a1;overflow-wrap:anywhere;margin-top:3px}.state{max-width:180px;font-size:12px;color:#9198a1}
[hidden]{display:none!important}@media(max-width:600px){main{padding:24px 16px}.tools{gap:10px}input{flex-basis:100%;width:100%}li{gap:4px;padding-right:12px}.entry{padding:13px 12px}.state{max-width:90px;font-size:11px}}
</style>
<main><nav aria-label="Списки релиза"><a href="prod.html"${view === "prod" ? ' aria-current="page"' : ""}>PROD · ${inventory.prod.length}</a><a href="dev.html"${view === "dev" ? ' aria-current="page"' : ""}>Только DEV · ${inventory.dev.length}</a></nav>
<h1>${title}</h1><p>${counts(pages)}. Допуск по текущему коду. Страницы PROD доступны также в DEV.</p>
<p>Ссылки открывают localhost:7159. Для просмотра DEV-страниц включи «Вид DEV» в колбе.</p>
<div class="tools"><input type="search" aria-label="Поиск страницы" placeholder="Поиск по названию или адресу"><span id="count" role="status">Показано ${pages.length} из ${pages.length}</span></div>
${sections}<p id="empty" hidden>Страницы не найдены.</p></main>
<script>
const rows=[...document.querySelectorAll("li")];
document.querySelector("input").addEventListener("input",event=>{
 const query=event.target.value.trim().toLocaleLowerCase("ru");let visible=0;
 for(const row of rows){row.hidden=!row.dataset.search.includes(query);if(!row.hidden)visible++;}
 for(const section of document.querySelectorAll("section"))section.hidden=![...section.querySelectorAll("li")].some(row=>!row.hidden);
 document.getElementById("count").textContent="Показано "+visible+" из "+rows.length;
 document.getElementById("empty").hidden=visible>0;
});
</script></html>`;
}

export function writePageLists({root = platformRoot, check = false, output = resolve(root, "../../.runtime/public-pages")} = {}) {
  const inventory = collectPageInventory(root);
  for (const view of ["prod", "dev"]) {
    const file = resolve(root, "docs/pages-" + view + ".md");
    const expected = renderMarkdown(inventory, view);
    const current = existsSync(file) ? readFileSync(file, "utf8").replaceAll("\r\n", "\n") : "";
    if (current === expected) continue;
    if (check) throw new Error("Release page list is stale. Run npm --prefix web/platform run release:pages: " + relative(root, file));
    mkdirSync(dirname(file), {recursive: true});
    writeFileSync(file, expected);
  }
  if (!check) {
    mkdirSync(output, {recursive: true});
    writeFileSync(resolve(output, "prod.html"), renderHtml(inventory, "prod"));
    writeFileSync(resolve(output, "dev.html"), renderHtml(inventory, "dev"));
    writeFileSync(resolve(output, "index.html"), renderHtml(inventory, "prod"));
    writeFileSync(resolve(output, "inventory.json"), JSON.stringify(inventory, null, 2) + "\n");
  }
  return inventory;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const inventory = writePageLists({check: process.argv.includes("--check")});
    console.log(`Списки страниц актуальны. PROD ${inventory.prod.length}, только DEV ${inventory.dev.length}.`);
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
