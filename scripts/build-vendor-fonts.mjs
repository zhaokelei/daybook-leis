import { cp, copyFile, mkdir, rm, stat } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const root = path.resolve(__dirname, "..");

async function mustExist(file) {
  try {
    await stat(file);
  } catch {
    console.error(`Missing required font asset: ${file}`);
    console.error("Run `npm install` first.");
    process.exit(1);
  }
}

async function copyDirClean(source, target) {
  await mustExist(source);
  await rm(target, { recursive: true, force: true });
  await mkdir(path.dirname(target), { recursive: true });
  await cp(source, target, { recursive: true });
  console.log(`Copied ${source} -> ${target}`);
}

async function copyFileClean(source, target) {
  await mustExist(source);
  await mkdir(path.dirname(target), { recursive: true });
  await copyFile(source, target);
  console.log(`Copied ${source} -> ${target}`);
}

// Copy LXGW WenKai Screen
await copyDirClean(
  path.join(root, "node_modules", "lxgw-wenkai-screen-web", "lxgwwenkaiscreen"),
  path.join(root, "internal", "embedded", "static", "vendor", "fonts", "lxgw-wenkai-screen", "regular"),
);

// Copy Maple Mono CN Regular
await copyDirClean(
  path.join(root, "node_modules", "@chinese-fonts", "maple-mono-cn", "dist", "MapleMono-CN-Regular"),
  path.join(root, "internal", "embedded", "static", "vendor", "fonts", "maple-mono-cn", "regular"),
);

// Copy Maple Mono CN Italic
await copyDirClean(
  path.join(root, "node_modules", "@chinese-fonts", "maple-mono-cn", "dist", "MapleMono-CN-Italic"),
  path.join(root, "internal", "embedded", "static", "vendor", "fonts", "maple-mono-cn", "italic"),
);

// Copy Material Symbols Rounded (subset)
// 完整字体约 5.3MB，会拖慢首屏图标字样（出现 "palettatranslate" / "MEMBER" 原始文字闪烁）。
// 这里只使用模板中实际用到的字形，通过 Google Fonts CSS2 的 icon_names 参数裁剪为约 29KB 的子集。
// 需要新增图标时，用下面的地址重新生成（把图标名追加到 icon_names，逗号分隔），
// 将返回 CSS 里 src 的 woff2 下载后覆盖 scripts/assets/material-symbols-rounded.min.woff2：
// https://fonts.googleapis.com/css2?family=Material+Symbols+Rounded:opsz,wght,FILL,GRAD@20..48,100..700,0..1,-50..200&icon_names=account_tree,archive,arrow_back,arrow_downward,arrow_forward,arrow_upward,article,attach_file,center_focus_strong,chrome_reader_mode,close,content_copy,exit_to_app,format_list_bulleted,home,info,menu,menu_open,music_note_2,palette,scatter_plot,search,sell,share,translate
await copyFileClean(
  path.join(root, "scripts", "assets", "material-symbols-rounded.min.woff2"),
  path.join(root, "internal", "embedded", "static", "vendor", "fonts", "material-symbols", "material-symbols-rounded.min.woff2"),
);

// Copy Fraunces (Italic)
await copyFileClean(
  path.join(root, "node_modules", "@fontsource", "fraunces", "files", "fraunces-latin-400-italic.woff2"),
  path.join(root, "internal", "embedded", "static", "vendor", "fonts", "fraunces", "fraunces-latin-400-italic.woff2"),
);

// Copy Allura (Normal)
await copyFileClean(
  path.join(root, "node_modules", "@fontsource", "allura", "files", "allura-latin-400-normal.woff2"),
  path.join(root, "internal", "embedded", "static", "vendor", "fonts", "allura", "allura-latin-400-normal.woff2"),
);

// Copy Cormorant Garamond Meta (Italic)
await copyFileClean(
  path.join(root, "node_modules", "@fontsource", "cormorant-garamond", "files", "cormorant-garamond-latin-400-italic.woff2"),
  path.join(root, "internal", "embedded", "static", "vendor", "fonts", "cormorant-garamond", "cormorant-garamond-latin-400-italic.woff2"),
);

// Copy Noto Serif SC Variable CSS
await copyFileClean(
  path.join(root, "node_modules", "@fontsource-variable", "noto-serif-sc", "wght.css"),
  path.join(root, "internal", "embedded", "static", "vendor", "fonts", "noto-serif-sc", "wght.css"),
);

// Copy Noto Serif SC Variable Files
await copyDirClean(
  path.join(root, "node_modules", "@fontsource-variable", "noto-serif-sc", "files"),
  path.join(root, "internal", "embedded", "static", "vendor", "fonts", "noto-serif-sc", "files"),
);

console.log("Vendor fonts copied successfully.");
