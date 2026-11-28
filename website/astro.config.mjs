import { defineConfig } from 'astro/config';

// The sitemap (with hreflang pairs), robots.txt, icons and precompression are
// produced by scripts/postbuild.mjs from the built HTML.
export default defineConfig({
  site: 'https://messora.tech',
  trailingSlash: 'never',
  build: { format: 'directory', inlineStylesheets: 'auto' },
  compressHTML: true,
  devToolbar: { enabled: false },
});
