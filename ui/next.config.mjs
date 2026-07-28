/** @type {import('next').NextConfig} */
const nextConfig = {
  // The UI is served from inside the go-via binary via go:embed, so the build
  // must be a self-contained static export rather than a Node server.
  output: "export",
  // go-via mounts the UI under /web/.
  basePath: "/web",
  images: { unoptimized: true },
};

export default nextConfig;
