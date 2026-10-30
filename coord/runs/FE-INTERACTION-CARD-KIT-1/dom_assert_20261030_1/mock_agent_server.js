// DOM 断言腿的 mock agent：只服务 /agent/* 端点，回 canned fixture。
// 请求体落 requests_log.jsonl 供断言回读（interaction/respond 的载荷核对）。
const http = require("http");
const fs = require("fs");
const path = require("path");

const dir = __dirname;
const logPath = path.join(dir, "requests_log.jsonl");
const fixtures = {
  "/agent/chat": "chat_choreo_confirm.json",
  "/agent/interaction/respond": "respond_settle.json"
};

const server = http.createServer((req, res) => {
  const chunks = [];
  req.on("data", (chunk) => chunks.push(chunk));
  req.on("end", () => {
    const body = Buffer.concat(chunks).toString("utf8");
    const urlPath = (req.url || "").split("?")[0];
    fs.appendFileSync(logPath, JSON.stringify({ t: new Date().toISOString(), method: req.method, path: urlPath, body }) + "\n");
    const fixture = fixtures[urlPath];
    if (fixture) {
      const payload = fs.readFileSync(path.join(dir, fixture));
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(payload);
      return;
    }
    if (urlPath === "/agent/events") {
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ events: [], next_seq: 0 }));
      return;
    }
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end("{}");
  });
});

server.listen(5799, "127.0.0.1", () => {
  console.log("mock agent listening on 127.0.0.1:5799");
});
