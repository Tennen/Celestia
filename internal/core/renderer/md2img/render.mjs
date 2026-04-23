#!/usr/bin/env node
import fs from "node:fs/promises";
import path from "node:path";
import { randomUUID } from "node:crypto";
import { chromium } from "playwright";
import { unified } from "unified";
import remarkParse from "remark-parse";
import remarkGfm from "remark-gfm";
import remarkRehype from "remark-rehype";
import rehypeStringify from "rehype-stringify";

const VIEWPORT_WIDTH = 375;
const VIEWPORT_HEIGHT = 800;
const DEVICE_SCALE_FACTOR = 3;
const PAGE_HEIGHT = 667;
const CANVAS_PADDING_TOP = 24;
const CANVAS_PADDING_RIGHT = 20;
const CANVAS_PADDING_BOTTOM = 32;
const CANVAS_PADDING_LEFT = 20;
const USABLE_HEIGHT = PAGE_HEIGHT - CANVAS_PADDING_TOP - CANVAS_PADDING_BOTTOM;

main().catch((error) => {
  console.error(error instanceof Error ? error.message : String(error));
  process.exit(1);
});

async function main() {
  const input = await readInput();
  const markdown = String(input.markdown || "").trim();
  if (!markdown) throw new Error("markdown is empty");
  const mode = input.mode === "multi-page" ? "multi-page" : "long-image";
  const outputDir = path.resolve(process.cwd(), String(input.output_dir || "data/renderer/md2img"));
  await fs.mkdir(outputDir, { recursive: true });

  const document = await buildHtml(markdown);
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({
    viewport: { width: VIEWPORT_WIDTH, height: VIEWPORT_HEIGHT },
    deviceScaleFactor: DEVICE_SCALE_FACTOR,
  });

  try {
    const images = mode === "multi-page"
      ? await renderMultiPage(page, document, outputDir)
      : await renderLongImage(page, document, outputDir);
    process.stdout.write(JSON.stringify({
      mode,
      images,
      output_dir: outputDir,
      source_chars: [...markdown].length,
      rendered_at: new Date().toISOString(),
    }));
  } finally {
    await browser.close();
  }
}

async function readInput() {
  const chunks = [];
  for await (const chunk of process.stdin) chunks.push(chunk);
  const raw = Buffer.concat(chunks).toString("utf8").trim();
  return raw ? JSON.parse(raw) : {};
}

async function renderLongImage(page, document, outputDir) {
  await page.setContent(document.html, { waitUntil: "load" });
  await waitForStableLayout(page);
  const filepath = path.join(outputDir, `md2img_${Date.now()}_${randomUUID()}.png`);
  await page.locator(".mobile-canvas").screenshot({ path: filepath, type: "png" });
  return [await imageInfo(filepath, VIEWPORT_WIDTH * DEVICE_SCALE_FACTOR, 0)];
}

async function renderMultiPage(page, document, outputDir) {
  await page.setContent(document.html, { waitUntil: "load" });
  await waitForStableLayout(page);
  const measures = await measureBlocks(page);
  const plan = paginateBlocks(measures);
  const pagedHtml = buildPagedHtml(document.blockHtmlById, plan);
  await page.setContent(pagedHtml, { waitUntil: "load" });
  await waitForStableLayout(page);

  const pages = page.locator(".mobile-page");
  const count = await pages.count();
  if (count === 0) throw new Error("md2img multi-page render produced no images");
  const images = [];
  for (let index = 0; index < count; index += 1) {
    const filepath = path.join(outputDir, `md2img_${Date.now()}_${index + 1}_${randomUUID()}.png`);
    await pages.nth(index).screenshot({ path: filepath, type: "png" });
    images.push(await imageInfo(filepath, VIEWPORT_WIDTH * DEVICE_SCALE_FACTOR, PAGE_HEIGHT * DEVICE_SCALE_FACTOR));
  }
  return images;
}

async function imageInfo(filepath, width, height) {
  const stat = await fs.stat(filepath);
  return {
    path: filepath,
    content_type: "image/png",
    size_bytes: stat.size,
    width,
    ...(height > 0 ? { height } : {}),
  };
}

async function waitForStableLayout(page) {
  await page.evaluate(async () => {
    const images = Array.from(document.images);
    await Promise.all(images.map((img) => {
      if (img.complete) return Promise.resolve();
      return new Promise((resolve) => {
        img.onload = () => resolve();
        img.onerror = () => resolve();
      });
    }));
    if (document.fonts?.ready) await document.fonts.ready;
  });
  await page.waitForTimeout(100);
}

async function buildHtml(markdown) {
  const rendered = await unified()
    .use(remarkParse)
    .use(remarkGfm)
    .use(remarkBlockPlugin)
    .use(remarkRehype)
    .use(rehypeBlockAttrPlugin)
    .use(rehypeStringify)
    .process(markdown);
  const blockHtml = String(rendered);
  const blockHtmlById = extractBlockHtmlById(blockHtml);
  if (blockHtmlById.size === 0) throw new Error("md2img produced no block sections");
  return {
    html: buildHtmlDocument(`<main class="render-root"><article class="mobile-canvas">${blockHtml}</article></main>`),
    blockHtmlById,
  };
}

function buildPagedHtml(blockHtmlById, plan) {
  const pageSections = plan.pages.map((page) => {
    const blocks = page.blockIds.map((blockId) => {
      const blockHtml = blockHtmlById.get(blockId);
      if (!blockHtml) throw new Error(`md2img could not find block html for ${blockId}`);
      return blockHtml;
    });
    return `<section class="mobile-page"><article class="page-canvas">${blocks.join("\n")}</article></section>`;
  });
  if (pageSections.length === 0) throw new Error("md2img page plan is empty");
  return buildHtmlDocument(`<main class="pages-root">${pageSections.join("\n")}</main>`);
}

function buildHtmlDocument(bodyHtml) {
  return [
    "<!DOCTYPE html>",
    '<html lang="zh-CN">',
    "<head>",
    '<meta charset="utf-8" />',
    '<meta name="viewport" content="width=device-width, initial-scale=1" />',
    "<style>",
    mobileCss(),
    "</style>",
    "</head>",
    "<body>",
    bodyHtml,
    "</body>",
    "</html>",
  ].join("\n");
}

function extractBlockHtmlById(blockHtml) {
  const sections = new Map();
  const matcher = /<section\b[^>]*data-block-id="([^"]+)"[^>]*>[\s\S]*?<\/section>/g;
  let match = matcher.exec(blockHtml);
  while (match) {
    sections.set(match[1], match[0]);
    match = matcher.exec(blockHtml);
  }
  return sections;
}

function remarkBlockPlugin() {
  return (tree) => {
    let seq = 0;
    visitMdast(tree, (node) => {
      if (!isBlockNode(node)) return;
      seq += 1;
      const meta = {
        id: `b_${seq}`,
        type: inferBlockType(node),
        breakInside: inferBreakInside(node),
        keepWithNext: node.type === "heading",
      };
      node.data = { ...(node.data || {}), hProperties: { ...((node.data || {}).hProperties || {}), __blockMeta: meta } };
    });
  };
}

function visitMdast(node, visitor) {
  visitor(node);
  for (const child of Array.isArray(node.children) ? node.children : []) visitMdast(child, visitor);
}

function isBlockNode(node) {
  return ["heading", "paragraph", "list", "blockquote", "code", "image", "thematicBreak", "table"].includes(String(node.type || ""));
}

function inferBlockType(node) {
  const type = String(node.type || "");
  if (type === "heading") return "heading";
  if (type === "list") return "list";
  if (type === "blockquote") return "blockquote";
  if (type === "code") return "code";
  if (type === "image") return "image";
  if (type === "thematicBreak") return "divider";
  return "paragraph";
}

function inferBreakInside(node) {
  return ["heading", "blockquote", "code", "image", "thematicBreak", "table"].includes(String(node.type || "")) ? "avoid" : "auto";
}

function rehypeBlockAttrPlugin() {
  return (tree) => visitHastParent(tree);
}

function visitHastParent(parent) {
  const children = Array.isArray(parent.children) ? parent.children : [];
  if (children.length === 0) return;
  parent.children = children.map((child) => wrapBlockNode(child));
  for (const child of parent.children) visitHastParent(child);
}

function wrapBlockNode(node) {
  const properties = { ...(node.properties || {}) };
  const meta = properties.__blockMeta;
  if (!meta || node.type !== "element") return { ...node, properties };
  delete properties.__blockMeta;
  return {
    type: "element",
    tagName: "section",
    properties: {
      "data-block-id": meta.id,
      "data-block-type": meta.type,
      "data-break-inside": meta.breakInside,
      "data-keep-with-next": String(meta.keepWithNext),
    },
    children: [{ ...node, properties }],
  };
}

async function measureBlocks(page) {
  const raw = await page.evaluate(() => {
    const canvas = document.querySelector(".mobile-canvas");
    if (!canvas) throw new Error("md2img failed to find .mobile-canvas during block measurement");
    const canvasRect = canvas.getBoundingClientRect();
    return Array.from(document.querySelectorAll("[data-block-id]")).map((node) => {
      const rect = node.getBoundingClientRect();
      return {
        id: node.getAttribute("data-block-id"),
        type: node.getAttribute("data-block-type"),
        breakInside: node.getAttribute("data-break-inside"),
        keepWithNext: node.getAttribute("data-keep-with-next") === "true",
        top: rect.top - canvasRect.top,
        height: rect.height,
      };
    });
  });
  const measures = raw.filter((item) => item.id && item.type && Number.isFinite(item.top) && Number.isFinite(item.height));
  if (measures.length === 0) throw new Error("md2img found no measurable blocks");
  return measures;
}

function paginateBlocks(blocks) {
  const ordered = [...blocks].sort((left, right) => left.top - right.top);
  const pages = [];
  let currentIds = [];
  let currentTop = null;
  let cursor = 0;
  const flush = () => {
    if (currentIds.length === 0) {
      currentTop = null;
      return;
    }
    pages.push({ index: pages.length, blockIds: [...currentIds] });
    currentIds = [];
    currentTop = null;
  };
  while (cursor < ordered.length) {
    const groupEnd = placementGroupEnd(ordered, cursor);
    const groupStart = ordered[cursor];
    const groupLast = ordered[groupEnd];
    const ids = ordered.slice(cursor, groupEnd + 1).map((block) => block.id);
    const groupBottom = groupLast.top + groupLast.height;
    if (groupStart.height > USABLE_HEIGHT) {
      flush();
      pages.push({ index: pages.length, blockIds: [groupStart.id] });
      cursor += 1;
      continue;
    }
    if (currentTop === null) {
      currentIds = ids;
      currentTop = groupStart.top;
      cursor = groupEnd + 1;
      continue;
    }
    if (groupBottom - currentTop <= USABLE_HEIGHT) {
      currentIds.push(...ids);
      cursor = groupEnd + 1;
      continue;
    }
    flush();
  }
  flush();
  return { pages };
}

function placementGroupEnd(blocks, startIndex) {
  let endIndex = startIndex;
  while (endIndex < blocks.length - 1 && blocks[endIndex].keepWithNext) endIndex += 1;
  if (endIndex === startIndex) return endIndex;
  const groupedHeight = blocks[endIndex].top + blocks[endIndex].height - blocks[startIndex].top;
  return groupedHeight > USABLE_HEIGHT ? startIndex : endIndex;
}

function mobileCss() {
  return `
html, body { margin: 0; padding: 0; background: #f5f5f5; }
body { font-family: -apple-system, BlinkMacSystemFont, "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif; color: #222; }
* { box-sizing: border-box; }
.render-root, .pages-root { width: 100%; }
.mobile-canvas, .page-canvas { width: ${VIEWPORT_WIDTH}px; margin: 0 auto; background: #fff; padding: ${CANVAS_PADDING_TOP}px ${CANVAS_PADDING_RIGHT}px ${CANVAS_PADDING_BOTTOM}px ${CANVAS_PADDING_LEFT}px; }
.mobile-page { width: ${VIEWPORT_WIDTH}px; height: ${PAGE_HEIGHT}px; margin: 0 auto; background: #fff; overflow: hidden; }
.mobile-page + .mobile-page { margin-top: 12px; }
.page-canvas { height: ${PAGE_HEIGHT}px; overflow: hidden; }
[data-block-id] { display: block; }
[data-break-inside="avoid"] { break-inside: avoid; page-break-inside: avoid; }
h1, h2, h3, h4, h5, h6 { font-weight: 700; color: #111; }
h1 { font-size: 24px; line-height: 1.4; margin: 0 0 12px; }
h2 { font-size: 20px; line-height: 1.45; margin: 20px 0 10px; }
h3 { font-size: 17px; line-height: 1.5; margin: 18px 0 8px; }
p { font-size: 15px; line-height: 1.75; margin: 0 0 12px; color: #222; }
ul, ol { margin: 0 0 12px 20px; padding: 0; }
li { font-size: 15px; line-height: 1.75; margin: 0 0 6px; }
blockquote { margin: 12px 0; padding: 0 0 0 12px; border-left: 3px solid #ff6b6b; color: #666; }
pre { margin: 12px 0; padding: 12px; background: #f6f8fa; border-radius: 8px; overflow: hidden; white-space: pre-wrap; word-break: break-word; }
code { font-size: 13px; line-height: 1.6; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; }
hr { margin: 20px 0; border: 0; border-top: 1px solid #eee; }
img { display: block; width: 100%; height: auto; margin: 12px 0; border-radius: 12px; }
table { width: 100%; border-collapse: collapse; margin: 12px 0; table-layout: fixed; }
th, td { border: 1px solid #e5e7eb; padding: 8px 10px; font-size: 13px; line-height: 1.6; vertical-align: top; word-break: break-word; }
th { background: #f8fafc; color: #111827; font-weight: 700; }
`.trim();
}
