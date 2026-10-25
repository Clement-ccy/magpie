// Run with Node's test runner and Playwright on the module path; see README.md.
// The request format a model is asked on, picked on the Agents page: the pill
// beside a model's name opens the API menu, and the pick is saved for that
// agent alone. The list stays open across a pick, so several models can be
// changed in a row (the list closed at the first pick);
// Escape over the open menu closes the menu, not the list.
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { test } = require("node:test");
const { chromium, webkit } = require("playwright");

const assets = process.env.MAGPIE_ASSETS ? path.resolve(process.env.MAGPIE_ASSETS) : path.resolve(__dirname, "../assets");
const models = () => [
  { id: "group:fast", name: "fast", group: "Routing groups", icons: ["openai", "anthropic"] },
  { id: "oa/gpt-6.1-sol", name: "GPT-6.1 Sol", group: "OpenAI", icon: "openai", context: 400e3, api: "", auto: "chat" },
  { id: "oa/gpt-6.1-luna", name: "GPT-6.1 Luna", group: "OpenAI", icon: "openai", context: 400e3, api: "", auto: "chat" },
];

function fixture(lang) {
  const picks = [];
  const state = () => ({
    agents: [{
      id: "dsh", name: "DeepSeek Harness", path: "/test/dsh.json", icon: "deepseek-color", wired: true,
      fields: [{ key: "model", label: "model", value: "magpie/oa/gpt-6.1-sol", options: [{ value: "magpie/oa/gpt-6.1-sol", label: "GPT-6.1 Sol", ref: "oa/gpt-6.1-sol" }] }],
      models: { shown: 3, listed: 3, by: [{ name: "OpenAI", icon: "openai", n: 2 }] },
    }],
    profiles: [], settings: { lang, theme: "light" },
  });
  async function serve(route) {
    const req = route.request();
    const url = new URL(req.url());
    const json = (data) => route.fulfill({ json: data });
    if (url.pathname === "/boot.js") return route.fulfill({ contentType: "text/javascript", body: `window.bootPrefs = {lang:"${lang}",theme:"light",web:true};` });
    if (url.pathname === "/wails/runtime.js") return route.fulfill({ contentType: "text/javascript", body: "export const Window = {};" });
    if (url.pathname === "/api/state") return json(state());
    if (url.pathname === "/api/agent-models/dsh") {
      return json({ models: models(), picksAPI: true, ordered: false, only: false, own: false, ownHidden: false });
    }
    if (url.pathname === "/api/agent-api" && req.method() === "POST") {
      const pick = req.postDataJSON();
      picks.push(pick);
      return json({ api: pick.api });
    }
    if (url.pathname === "/api/groups") return json({ groups: [] });
    if (url.pathname === "/api/providers") return json({ providers: [], gateway: { running: true, window: true } });
    if (url.pathname.startsWith("/api/")) return json({});
    const file = path.join(assets, url.pathname === "/" ? "index.html" : url.pathname);
    const contentType = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml", ".png": "image/png" }[path.extname(file)];
    await route.fulfill({ body: await fs.readFile(file), contentType });
  }
  return { serve, picks };
}

const W = {
  en: { auto: "Auto", menu: "API this model is asked on", note: "As its provider or group says: OpenAI", responses: "Responses", anthropic: "Anthropic" },
  zh: { auto: "自动", menu: "这个模型请求所用的协议", note: "按供应商或路由组指定：OpenAI", responses: "Responses", anthropic: "Anthropic" },
};

for (const engine of (process.env.BROWSER ? [process.env.BROWSER] : ["chromium", "webkit"])) {
  for (const lang of ["en", "zh"]) {
    test(`${engine} ${lang}: a model's request format`, async (t) => {
      const w = W[lang];
      const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
      const page = await browser.newPage({ viewport: { width: 1000, height: 700 } });
      page.setDefaultTimeout(5000);
      const errors = [];
      page.on("pageerror", (e) => errors.push(e.message));
      const fx = fixture(lang);
      await page.route("**/*", fx.serve);
      t.after(async () => {
        if (process.env.ARTIFACT_DIR) {
          await fs.mkdir(process.env.ARTIFACT_DIR, { recursive: true });
          await page.screenshot({ path: path.join(process.env.ARTIFACT_DIR, `${engine}-${lang}-agent-api.png`) });
        }
        await browser.close();
      });
      await page.goto("http://magpie.test/");
      const row = page.locator('.row.agent[data-id="dsh"]');
      await row.locator(".ag-link").click();
      await row.locator(".ag-exp .ag-chips .ag-quiet").click();
      const pop = page.locator(".am-pop:not(.leaving)");
      await pop.waitFor();
      const menu = page.locator(".proto-menu");
      const item = (name) => menu.locator(".pm-item").filter({ has: page.locator(".pm-name", { hasText: new RegExp("^" + name + "$") }) });
      const pill = (name) => pop.locator(".am-mr", { hasText: name }).first().locator(".am-api");
      const said = (name) => page.evaluate((n) => [...document.querySelectorAll(".am-pop .am-mr")].find((r) => r.innerText.includes(n))?.querySelector(".am-api")?.innerText, name);

      // a model left on Auto says so, and offers what it is asked on now
      assert.equal(await pill("GPT-6.1 Sol").innerText(), w.auto);
      await pill("GPT-6.1 Sol").click();
      await menu.waitFor();
      assert.equal(await menu.locator(".pm-head").innerText(), w.menu);
      assert.equal(await item(w.auto).locator(".pm-note").innerText(), w.note);

      // a pick is saved for this agent and this model, and the list stays up
      await item(w.responses).click();
      await page.waitForFunction(() => [...document.querySelectorAll(".am-pop .am-api")].some((b) => b.innerText === "Responses"));
      assert.deepEqual(fx.picks, [{ for: "dsh", ref: "oa/gpt-6.1-sol", api: "responses" }]);
      assert.equal(await pop.count(), 1, "the list stays open after a pick");
      assert.equal(await menu.count(), 0, "the menu closes on it");
      assert(await pop.locator(".am-api.on").count() === 1);

      // so the next model is one click away, the list never closing
      await pill("GPT-6.1 Luna").click();
      await menu.waitFor();
      await item(w.anthropic).click();
      await page.waitForFunction(() => document.querySelectorAll(".am-pop .am-api.on").length === 2);
      assert.deepEqual(fx.picks[1], { for: "dsh", ref: "oa/gpt-6.1-luna", api: "anthropic" });
      assert.equal(await pop.count(), 1, "still open");
      assert.equal(await said("GPT-6.1 Sol"), "Responses");
      assert.equal(await said("GPT-6.1 Luna"), "Anthropic");

      // the menu's own Escape closes the menu alone; the list takes the next
      await pill("GPT-6.1 Sol").click();
      await menu.waitFor();
      await page.keyboard.press("Escape");
      assert.equal(await menu.count(), 0);
      assert.equal(await pop.count(), 1, "Escape over the menu leaves the list");
      await page.keyboard.press("Escape");
      await page.waitForFunction(() => !document.querySelector(".am-pop:not(.leaving)"));

      assert.deepEqual(errors, []);
    });
  }
}
