import { copyFile, mkdir, rm } from "node:fs/promises";
import { join } from "node:path";

const projectDir = join(import.meta.dir, "..");
const outputDir = join(projectDir, "dist");

await rm(outputDir, { recursive: true, force: true });
await mkdir(outputDir, { recursive: true });

const result = await Bun.build({
  entrypoints: [
    join(projectDir, "src", "index.ts"),
    join(projectDir, "src", "docs_api.ts"),
  ],
  outdir: outputDir,
  target: "browser",
  minify: true,
  naming: "[name].[ext]",
});

if (!result.success) {
  for (const log of result.logs) {
    console.error(log);
  }
  process.exit(1);
}

await copyFile(
  join(projectDir, "src", "index.html"),
  join(outputDir, "index.html"),
);
await copyFile(
  join(projectDir, "src", "docs_api.html"),
  join(outputDir, "docs_api.html"),
);
