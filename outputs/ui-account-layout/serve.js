// 静态服务 upstream/frontend，并把 outputs 里的验证页挂到 /__h.html。
const http = require("http");
const fs = require("fs");
const path = require("path");

const ROOT = "C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/upstream/frontend";
const OUT = __dirname;
const PORT = Number(process.argv[2] || 8791);

const MIME = { ".html": "text/html; charset=utf-8", ".css": "text/css; charset=utf-8", ".js": "text/javascript; charset=utf-8", ".svg": "image/svg+xml", ".png": "image/png", ".woff2": "font/woff2" };

http.createServer((req, res) => {
  let p = decodeURIComponent(req.url.split("?")[0]);
  if (p === "/__h.html") {
    res.writeHead(200, { "Content-Type": MIME[".html"] });
    return res.end(fs.readFileSync(path.join(OUT, "harness.html")));
  }
  if (p === "/__p.html") {
    res.writeHead(200, { "Content-Type": MIME[".html"] });
    return res.end(fs.readFileSync(path.join(OUT, "harness-pool.html")));
  }
  const file = path.join(ROOT, p === "/" ? "index.html" : p);
  if (!file.startsWith(path.resolve(ROOT))) { res.writeHead(403); return res.end("no"); }
  fs.readFile(file, (err, buf) => {
    if (err) { res.writeHead(404); return res.end("404"); }
    res.writeHead(200, { "Content-Type": MIME[path.extname(file).toLowerCase()] || "application/octet-stream" });
    res.end(buf);
  });
}).listen(PORT, "127.0.0.1", () => console.log("serving http://127.0.0.1:" + PORT));
