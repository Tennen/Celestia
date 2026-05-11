import fs from "node:fs/promises";
import path from "node:path";
import { chromium } from "playwright";

const stdin = await readStdin();
const request = JSON.parse(stdin || "{}");
const url = String(request.url || "").trim();
if (!url) {
  throw new Error("url is required");
}

const width = positiveInt(request.width, 1440);
const height = positiveInt(request.height, 1000);
const waitMS = positiveInt(request.wait_ms, 500);
const outputDir = String(request.output_dir || "data/agent/screenshots");
await fs.mkdir(outputDir, { recursive: true });

const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage({ viewport: { width, height }, deviceScaleFactor: 1 });
  await page.goto(url, { waitUntil: "networkidle", timeout: positiveInt(request.timeout_ms, 30000) });
  if (waitMS > 0) {
    await page.waitForTimeout(waitMS);
  }
  const filename = `screenshot-${new Date().toISOString().replace(/[:.]/g, "-")}.png`;
  const filepath = path.join(outputDir, filename);
  await page.screenshot({ path: filepath, fullPage: request.full_page === true, type: "png" });
  const stat = await fs.stat(filepath);
  const result = {
    url,
    image: {
      path: filepath,
      content_type: "image/png",
      size_bytes: stat.size,
      width,
      height,
    },
    viewport: `${width}x${height}`,
    captured_at: new Date().toISOString(),
  };
  process.stdout.write(JSON.stringify(result));
} finally {
  await browser.close();
}

function positiveInt(value, fallback) {
  const parsed = Number.parseInt(String(value ?? ""), 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

function readStdin() {
  return new Promise((resolve, reject) => {
    let data = "";
    process.stdin.setEncoding("utf8");
    process.stdin.on("data", (chunk) => {
      data += chunk;
    });
    process.stdin.on("end", () => resolve(data));
    process.stdin.on("error", reject);
  });
}
