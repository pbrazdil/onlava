const baseURL = "https://local.drizzle.studio";
const outDir = new URL("../reference/original/", import.meta.url);

const files = [
  ["index.html", "/"],
  ["index.js", "/index.js"],
  ["favicon.svg", "/favicon.svg"],
] as const;

await Bun.write(
  new URL(".keep", outDir),
  "",
);

for (const [name, path] of files) {
  const response = await fetch(new URL(path, baseURL));
  if (!response.ok) {
    throw new Error(`failed fetching ${path}: ${response.status} ${response.statusText}`);
  }
  await Bun.write(new URL(name, outDir), await response.arrayBuffer());
  console.log(`synced ${name}`);
}

const sourceMapPath = "/index.js.map";
const sourceMapResponse = await fetch(new URL(sourceMapPath, baseURL));
if (sourceMapResponse.ok) {
  const contentType = (sourceMapResponse.headers.get("content-type") || "").toLowerCase();
  const text = await sourceMapResponse.text();
  let isValidSourceMap = contentType.includes("json");
  if (!isValidSourceMap) {
    try {
      const parsed = JSON.parse(text) as { version?: unknown; sources?: unknown };
      isValidSourceMap = parsed.version === 3 && Array.isArray(parsed.sources);
    } catch {
      isValidSourceMap = false;
    }
  }
  const mapURL = new URL("index.js.map", outDir);
  if (isValidSourceMap) {
    await Bun.write(mapURL, text);
    console.log("synced index.js.map");
  } else {
    await Bun.file(mapURL).delete().catch(() => {});
    console.log(`index.js.map unavailable (received ${contentType || "unknown content-type"})`);
  }
} else {
  console.log(`index.js.map unavailable (${sourceMapResponse.status} ${sourceMapResponse.statusText})`);
}
