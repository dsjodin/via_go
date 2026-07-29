/** @type {import('next').NextConfig} */
const nextConfig = {
  // The UI is served from inside the go-via binary via go:embed, so the build
  // must be a self-contained static export rather than a Node server.
  output: "export",
  // go-via mounts the UI under /web/.
  basePath: "/web",
  // Export each route as <route>/index.html rather than <route>.html. Go's
  // http.FileServer has no .html fallback, so without this a reload or a
  // typed URL on any page but the root is a 404 — client-side navigation
  // hides it, which is why it survives a click-through.
  trailingSlash: true,
  images: { unoptimized: true },
};

export default nextConfig;
