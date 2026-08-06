import path from "node:path";
import { fileURLToPath } from "node:url";
import CopyPlugin from "copy-webpack-plugin";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// Bundles src/client.ts (Engine.launch(...) + your scenes) into a single
// out/client.js, alongside out/index.html (written once by `jc init`) and
// out/assets/* (copied from public/assets/ below). `jc dev`/`jc deploy`
// both read this directory straight off disk -- see jc-cli's own
// cmd/dev.go / cmd/deploy.go.
export default (_env, argv) => ({
  mode: argv.mode === "development" ? "development" : "production",
  target: "web",
  devtool: argv.mode === "development" ? "eval-source-map" : false,
  entry: "./src/client.ts",
  output: {
    path: path.resolve(__dirname, "out"),
    filename: "client.js",
    clean: false,
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
    // A title's whole client is one script tag (out/index.html) -- no
    // route-based code splitting to gain from, and it keeps the bundle
    // predictable to zip up in jc deploy's createBundleArchive.
    splitChunks: false,
  },
  plugins: [
    new CopyPlugin({
      patterns: [{ from: "public/assets", to: "assets", noErrorOnMissing: true }],
    }),
  ],
  devServer: {
    static: path.resolve(__dirname, "out"),
    port: 8080,
    open: false,
  },
});
