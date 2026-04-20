const originalDir = new URL("../reference/original/", import.meta.url);
const prettyDir = new URL("../reference/pretty/", import.meta.url);

const formats = [
  { name: "index.js", parser: "babel" },
  { name: "index.html", parser: "html" },
] as const;

await Bun.write(new URL(".keep", prettyDir), "");

for (const file of formats) {
  const sourcePath = new URL(file.name, originalDir);
  const output = await Bun.$`bunx prettier --parser ${file.parser} ${sourcePath.pathname}`.quiet().text();
  await Bun.write(new URL(file.name, prettyDir), output);
  console.log(`formatted ${file.name}`);
}

const favicon = Bun.file(new URL("favicon.svg", originalDir));
if (await favicon.exists()) {
  await Bun.write(new URL("favicon.svg", prettyDir), await favicon.arrayBuffer());
  console.log("copied favicon.svg");
}
