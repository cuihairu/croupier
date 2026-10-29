/* Requires Playwright/Chromium. Optional CROUPIER_PLAYWRIGHT_MODULE points at an existing install.
   Build docs first. Screenshots default to /tmp/croupier-architecture-review. */
const { chromium } = require(
  process.env.CROUPIER_PLAYWRIGHT_MODULE || "playwright",
);
const fs = require("node:fs/promises"),
  path = require("node:path");
const root = path.resolve(__dirname, "../.."),
  out =
    process.env.CROUPIER_ARCH_SCREENSHOTS ||
    "/tmp/croupier-architecture-review";
(async () => {
  await fs.mkdir(out, { recursive: true });
  const browser = await chromium.launch({
    headless: true,
    args: ["--no-sandbox"],
  });
  try {
    const page = await browser.newPage({
        viewport: { width: 1440, height: 1000 },
      }),
      fail = [];
    for (const name of (
      await fs.readdir(root + "/docs/public/architecture")
    ).filter((x) => x.endsWith(".svg"))) {
      const svg = await fs.readFile(
        root + "/docs/public/architecture/" + name,
        "utf8",
      );
      await page.setContent(
        '<!doctype html><html><meta charset="utf-8"><body style="margin:0">' +
          svg +
          "</body></html>",
      );
      await page.evaluate(() => document.fonts.ready);
      const errors = await page.evaluate(() => {
        const svg = document.querySelector("svg"),
          v = svg.viewBox.baseVal,
          ts = [...svg.querySelectorAll("text")],
          errors = [];
        for (const n of ts) {
          const b = n.getBBox(),
            box = n.closest("[data-node]")?.querySelector("rect").getBBox();
          if (
            b.x < 0 ||
            b.x + b.width > v.width ||
            b.y < 0 ||
            b.y + b.height > v.height
          )
            errors.push("canvas overflow: " + n.textContent);
          if (
            box &&
            (b.x < box.x + 5 ||
              b.x + b.width > box.x + box.width - 5 ||
              b.y < box.y ||
              b.y + b.height > box.y + box.height - 3)
          )
            errors.push("node overflow: " + n.textContent);
        }
        for (let i = 0; i < ts.length; i++)
          for (let j = i + 1; j < ts.length; j++) {
            const a = ts[i].getBBox(),
              b = ts[j].getBBox();
            if (
              Math.min(a.x + a.width, b.x + b.width) - Math.max(a.x, b.x) > 1 &&
              Math.min(a.y + a.height, b.y + b.height) - Math.max(a.y, b.y) > 1
            )
              errors.push(
                "overlap: " + ts[i].textContent + " / " + ts[j].textContent,
              );
          }
        if (svg.getAttribute("height") === "2310")
          for (const e of svg.querySelectorAll("path.edge")) {
            const hits = new Set();
            for (let d = 4; d < e.getTotalLength() - 4; d += 2) {
              const p = e.getPointAtLength(d);
              for (const n of ts) {
                const b = n.getBBox();
                if (
                  p.x > b.x - 1 &&
                  p.x < b.x + b.width + 1 &&
                  p.y > b.y - 1 &&
                  p.y < b.y + b.height + 1
                )
                  hits.add(n.textContent);
              }
            }
            for (const s of hits) errors.push("edge through text: " + s);
          }
        return errors;
      });
      if (errors.length) fail.push({ name, errors });
      if (!name.includes("mobile"))
        await page.screenshot({
          path: path.join(out, name + ".png"),
          fullPage: true,
          timeout: 10000,
        });
      console.log(name + ": " + (errors.length ? "FAIL" : "PASS"));
    }
    await page.route("http://architecture.local/**", async (route) => {
      let p = new URL(route.request().url()).pathname.replace(
        /^\/croupier\//,
        "/",
      );
      if (p.endsWith("/")) p += "index.html";
      else if (!p.split("/").pop().includes(".")) p += ".html";
      const type =
        {
          html: "text/html",
          js: "text/javascript",
          css: "text/css",
          svg: "image/svg+xml",
          png: "image/png",
          woff2: "font/woff2",
        }[p.split(".").pop()] || "application/octet-stream";
      try {
        await route.fulfill({
          body: await fs.readFile(path.join(root, "docs/.vitepress/dist", p)),
          contentType: type,
        });
      } catch {
        await route.fulfill({ status: 404, body: "not found" });
      }
    });
    for (const lang of ["en", "zh"])
      for (const width of [1440, 390])
        for (const suffix of [
          "",
          "architecture/",
          "architecture/runtime-topology",
        ]) {
          await page.setViewportSize({ width, height: 1000 });
          const url = `http://architecture.local/croupier/${lang === "zh" ? "zh/" : ""}${suffix}`;
          await page.goto(url);
          const fig = page.locator(".architecture-diagram");
          await fig.first().scrollIntoViewIfNeeded();
          await page.waitForFunction(() =>
            [...document.querySelectorAll(".architecture-diagram img")].every(
              (i) => i.complete && i.naturalWidth > 0,
            ),
          );
          for (const image of await page
            .locator('img[src*="/architecture/"]')
            .all()) {
            await image.scrollIntoViewIfNeeded();
            await image.evaluate((i) => i.decode());
          }
          const info = await page.evaluate(() => ({
            overflow: document.documentElement.scrollWidth > innerWidth,
            images: [
              ...document.querySelectorAll('img[src*="/architecture/"]'),
            ].map((i) => ({
              src: i.currentSrc,
              loaded: i.complete && i.naturalWidth > 0,
            })),
            link: document
              .querySelector(".architecture-diagram a")
              ?.getAttribute("href"),
          }));
          if (
            info.overflow ||
            !info.images.some((i) =>
              i.src.endsWith(
                `system-${lang}${width < 768 ? "-mobile" : ""}.svg`,
              ),
            ) ||
            info.images.some((i) => !i.loaded) ||
            !info.link.endsWith(`system-${lang}.svg`)
          )
            fail.push({ url, width, info });
          if (!suffix)
            await fig.screenshot({
              path: path.join(out, `home-${lang}-${width}.png`),
              timeout: 10000,
            });
          console.log(`${lang} ${width} ${suffix || "home"}: page/image check`);
        }
    for (const lang of ["en", "zh"])
      for (const width of [1440, 390]) {
        await page.setViewportSize({ width, height: 1000 });
        const url = `http://architecture.local/croupier/${lang === "zh" ? "zh/" : ""}analytics/core-and-external`;
        const response = await page.goto(url);
        await page.locator(".vp-doc h1").waitFor();
        const info = await page.evaluate(() => ({
          overflow: document.documentElement.scrollWidth > innerWidth,
          title: document.querySelector(".vp-doc h1")?.textContent,
        }));
        if (response.status() !== 200 || info.overflow || !info.title)
          fail.push({ url, width, info });
        await page.screenshot({
          path: path.join(out, `analytics-${lang}-${width}.png`),
          fullPage: true,
          timeout: 10000,
        });
        console.log(`${lang} ${width} analytics boundary: page check`);
      }
    if (fail.length) throw Error(JSON.stringify(fail, null, 2));
    console.log(
      "PASS: 16 SVGs, 12 architecture pages and 4 analytics pages; screenshots: " +
        out,
    );
  } finally {
    await browser.close();
  }
})().catch((e) => {
  console.error(e);
  process.exitCode = 1;
});
