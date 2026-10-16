// Renders the UI against mocked API responses and saves screenshots.
//
//   cd ui && node --experimental-strip-types scripts/screenshots/capture.ts [outDir]
//
// SHOT_THEME=light renders the light theme; SHOT_WIDTH/SHOT_HEIGHT set the viewport.
//
// Starts a Vite dev server, intercepts the API (REST + gRPC-web) with the
// fixtures from ./fixtures.ts, and captures the Timeline, Graph and Settings views.
// Set CHROMIUM_PATH to use a preinstalled Chromium. In sandboxes without access
// to cdn.tailwindcss.com, set TAILWIND_CSS to a locally compiled Tailwind v3
// stylesheet (same theme as index.html) and it is served in place of the CDN.

import { createServer } from 'vite';
import { chromium } from 'playwright';
import path from 'node:path';
import { mkdirSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { TimelineChunk } from '../../src/generated/timeline.ts';
import { buildTimelineResources, buildNamespaceGraph, NAMESPACES, KINDS } from './fixtures.ts';

const here = path.dirname(fileURLToPath(import.meta.url));
const uiRoot = path.resolve(here, '../..');
const outDir = path.resolve(process.argv[2] ?? path.join(uiRoot, '..', 'assets'));
const viewport = { width: Number(process.env.SHOT_WIDTH ?? 1440), height: Number(process.env.SHOT_HEIGHT ?? 760) };

function grpcWebFrame(flag: number, payload: Uint8Array): Uint8Array {
  const frame = new Uint8Array(5 + payload.length);
  frame[0] = flag;
  new DataView(frame.buffer).setUint32(1, payload.length, false);
  frame.set(payload, 5);
  return frame;
}

// Timestamps travel as int64 nanoseconds on the wire; the fixtures use seconds.
const toNanos = (seconds: number) => (seconds ? (BigInt(seconds) * 1_000_000_000n).toString() : 0) as unknown as number;

function timelineBody(nowSeconds: number): Buffer {
  const resources = buildTimelineResources(nowSeconds).map(r => ({
    ...r,
    createdAt: toNanos(r.createdAt),
    statusSegments: r.statusSegments.map(seg => ({ ...seg, startTime: toNanos(seg.startTime), endTime: toNanos(seg.endTime) })),
    events: r.events.map(evt => ({ ...evt, timestamp: toNanos(evt.timestamp) })),
  }));
  const chunks = [
    TimelineChunk.encode({
      metadata: {
        totalCount: resources.length,
        filesSearched: 1,
        segmentsScanned: 1,
        segmentsSkipped: 0,
        queryExecutionTimeMs: 42,
        nextCursor: '',
        hasMore: false,
        pageSize: 100,
      },
    }).finish(),
    TimelineChunk.encode({ batch: { kind: 'all', resources, isFinalBatch: true } }).finish(),
  ];
  const trailer = new TextEncoder().encode('grpc-status:0\r\n');
  const frames = [...chunks.map(c => grpcWebFrame(0, c)), grpcWebFrame(0x80, trailer)];
  return Buffer.concat(frames.map(f => Buffer.from(f)));
}

async function main() {
  mkdirSync(outDir, { recursive: true });

  const server = await createServer({ root: uiRoot, logLevel: 'error', server: { port: 5199, host: '127.0.0.1' } });
  await server.listen();
  const base = 'http://127.0.0.1:5199';

  const proxy = process.env.HTTPS_PROXY || process.env.https_proxy;
  const browser = await chromium.launch({
    executablePath: process.env.CHROMIUM_PATH || undefined,
    proxy: proxy ? { server: proxy, bypass: '127.0.0.1,localhost' } : undefined,
  });
  const context = await browser.newContext({ viewport, deviceScaleFactor: 2, ignoreHTTPSErrors: true });
  await context.addInitScript((theme: string) => {
    window.localStorage.setItem('spectre-graph-namespace', 'external-secrets');
    if (theme) {
      const settings = JSON.parse(window.localStorage.getItem('spectre-settings') || '{}');
      window.localStorage.setItem('spectre-settings', JSON.stringify({ ...settings, theme }));
    }
  }, process.env.SHOT_THEME ?? '');

  if (process.env.TAILWIND_CSS) {
    const css = readFileSync(process.env.TAILWIND_CSS, 'utf8');
    const script = `window.tailwind = window.tailwind || {}; (() => { const s = document.createElement('style'); s.textContent = ${JSON.stringify(css)}; document.head.appendChild(s); })();`;
    await context.route(/cdn\.tailwindcss\.com/, route => route.fulfill({ contentType: 'application/javascript', body: script }));
  }
  await context.route(/fonts\.(googleapis|gstatic)\.com/, route => route.abort());
  await context.route(/cdnjs\.cloudflare\.com/, route => route.fulfill({ contentType: 'application/javascript', body: '' }));

  const now = () => Math.floor(Date.now() / 1000);
  await context.route(/\/health$/, route => route.fulfill({ json: { status: 'ok', watcherEnabled: true } }));
  await context.route(/\/v1\/metadata/, route =>
    route.fulfill({ json: { namespaces: NAMESPACES, kinds: KINDS, timeRange: { earliest: now() - 86400, latest: now() } } }),
  );
  await context.route(/\/v1\/namespace-graph/, route => route.fulfill({ json: buildNamespaceGraph(now()) }));
  await context.route(/api\.TimelineService\/GetTimeline/, route =>
    route.fulfill({ status: 200, headers: { 'content-type': 'application/grpc-web+proto' }, body: timelineBody(now()) }),
  );

  const page = await context.newPage();
  page.on('pageerror', err => console.error('page error:', err.message));

  // Timeline with a resource selected
  await page.goto(`${base}/?start=now-30m&end=now`);
  await page.waitForSelector('g.label');
  await page.waitForTimeout(500);
  const segment = await page.locator('g.resource-row').nth(3).locator('rect.segment').nth(2).boundingBox();
  if (segment) await page.mouse.click(segment.x + segment.width / 2, segment.y + segment.height / 2);
  await page.waitForTimeout(900);
  await page.mouse.move(viewport.width / 2, viewport.height - 4);
  await page.screenshot({ path: path.join(outDir, 'screenshot-2.png') });
  console.log('saved timeline');

  // Graph with the top causal path selected
  await page.goto(`${base}/graph`);
  await page.waitForSelector('g.node');
  await page.waitForTimeout(500);
  await page.locator('[data-causal-path]').first().click();
  await page.waitForTimeout(900);
  await page.mouse.move(viewport.width / 2, viewport.height - 4);
  await page.screenshot({ path: path.join(outDir, 'screenshot-1.png') });
  console.log('saved graph');

  // Settings (not used in the README, written for review)
  await page.goto(`${base}/settings`);
  await page.waitForTimeout(600);
  await page.screenshot({ path: path.join(outDir, 'screenshot-settings.png') });
  console.log('saved settings');

  await browser.close();
  await server.close();
}

main().catch(err => {
  console.error(err);
  process.exit(1);
});
