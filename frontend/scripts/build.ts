import { copyFile, mkdir, rm } from "node:fs/promises";
import { join } from "node:path";

const frontendDir = join(import.meta.dir, "..");
const frontendDist = join(frontendDir, "dist");

await rm(frontendDist, { recursive: true, force: true });
await mkdir(frontendDist, { recursive: true });

const frontendBuild = await Bun.build({
  entrypoints: [join(frontendDir, "src", "index.ts")],
  outdir: frontendDist,
  target: "browser",
  minify: true,
  naming: "[name].[ext]",
});

if (!frontendBuild.success) {
  for (const frontendLog of frontendBuild.logs) {
    console.error(frontendLog);
  }
  process.exit(1);
}

await copyFile(join(frontendDir, "src", "index.html"), join(frontendDist, "index.html"));
