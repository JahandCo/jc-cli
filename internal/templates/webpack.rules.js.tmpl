import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// Bundles src/rules.ts into dist/rules.js -- session-host's V8 isolates
// (isolated-vm) have zero module resolution at runtime, so this MUST stay
// a single, fully self-contained ESM file: no code splitting, no
// externals, no dynamic import(). `output.module`/`experiments.outputModule`
// produce plain `export`/`import` syntax (no CommonJS wrapper) so
// `handleIntent` is a real named export -- jc dev's local RulesRunner (and
// session-host itself in production) both load this with a static
// `import { handleIntent } from "..."`.
export default {
  mode: "production",
  target: "node",
  devtool: false,
  entry: "./src/rules.ts",
  output: {
    path: path.resolve(__dirname, "dist"),
    filename: "rules.js",
    module: true,
    clean: false,
    // Without this, webpack has no signal that handleIntent is a public
    // interface rather than dead code -- production mode's default
    // usedExports/tree-shaking silently stripped both the `reply` import
    // and handleIntent's own body down to a dangling `var reply;` the
    // first time this config shipped without it. `library: { type:
    // "module" }` tells webpack this entry's exports form the module's
    // real API and must survive.
    library: { type: "module" },
  },
  experiments: {
    outputModule: true,
  },
  resolve: {
    extensions: [".ts", ".js"],
  },
  module: {
    rules: [
      {
        test: /\.ts$/,
        exclude: /node_modules/,
        use: {
          loader: "ts-loader",
          // tsconfig.json sets noEmit: true for the standalone `tsc --noEmit`
          // typecheck script -- ts-loader needs real (in-memory) emission
          // regardless, so override it here rather than in tsconfig.json.
          options: { compilerOptions: { noEmit: false } },
        },
      },
    ],
  },
  optimization: {
    splitChunks: false,
    // Left unminified on purpose -- this only ever runs server-side inside
    // a trusted session isolate, never shipped to a browser, so there's no
    // payload-size reason to minify, and it keeps stack traces from a
    // thrown handleIntent readable in session-host's own logs.
    minimize: false,
  },
};
