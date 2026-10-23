import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

// Execute the actual standalone checker sent to the platform container, with a
// local response fixture instead of network access. No duplicate checker logic.
const smoke = readFileSync(new URL("../deploy/smoke-dev.sh", import.meta.url), "utf8");
const block = smoke.match(/node --input-type=module - <<'NODE'\r?\n([\s\S]*?)\r?\nNODE/);
assert.ok(block, "DEV smoke must contain its standalone login-page checker");
const AsyncFunction = Object.getPrototypeOf(async function () {}).constructor;
const check = new AsyncFunction("fetch", "process", "console", block[1]);

const fixtures = [
  ["SVG and localized VK label", '<button><svg aria-hidden="true"><path d="M0 0" /></svg>Войти с VK ID</button>', true, 0],
  ["nested English VK label", '<button><span>Sign in with <strong>VK</strong> ID</span></button>', true, 0],
  ["plain VK label", "<button>VK</button>", true, 0],
  ["missing VK control", "<button>Войти с Яндекс ID</button>", true, 1],
  ["VK text outside a control", "<p>VK</p><button>Вход через почту</button>", true, 1],
  ["disabled VK control", '<button type="button" disabled="">VK</button>', true, 1],
  ["VK only inside an SVG", "<button><svg><title>VK</title></svg>Другой способ входа</button>", true, 1],
  ["unhealthy login response", "<button>VK</button>", false, 1],
];

for (const [name, html, ok, expected] of fixtures) {
  test(name, async () => {
    const stopped = Symbol("checker exit");
    let exitCode = 0;
    const fetchFixture = async (url) => {
      assert.equal(url, "http://127.0.0.1:3000/ru/login");
      return { ok, text: async () => html };
    };
    try {
      await check(fetchFixture, { exit(code) { exitCode = code; throw stopped; } }, { log() {}, error() {} });
    } catch (error) {
      if (error !== stopped) throw error;
    }
    assert.equal(exitCode, expected);
  });
}
