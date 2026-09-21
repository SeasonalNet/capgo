# Node.js

Use `child_process.spawnSync` to pipe XML to `capgo` and parse the JSON document from stdout.

```js
const { spawnSync } = require("node:child_process");
const fs = require("node:fs");

const capgo = process.env.CAPGO_BIN || "capgo";
const xml = fs.readFileSync(0);
const result = spawnSync(capgo, ["-profile", "nws", "-compact"], {
  input: xml,
  encoding: "utf8",
});

if (result.error) throw result.error;
if (result.status !== 0 && result.status !== 1) {
  throw new Error(result.stderr || `capgo exited ${result.status}`);
}

const document = JSON.parse(result.stdout);
for (const action of document.actions) {
  console.log(`${action.event} (${action.language})`);
}
if (!document.valid) {
  for (const issue of document.issues) {
    console.error(`${issue.rule} ${issue.path}: ${issue.message}`);
  }
  process.exitCode = 1;
}
```
