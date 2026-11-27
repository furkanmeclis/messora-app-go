// The brand typefaces, bundled (no third-party request; the CSP allows
// fonts from this origin only).
import '@fontsource-variable/inter/wght.css';
import '@fontsource-variable/plus-jakarta-sans/wght.css';

/**
 * The platform admin panel, served on admin.<base> only: the edge proxy
 * rewrites every path of that host into this segment and answers 404 for
 * it on every other host.
 */
export default function PlatformLayout({
    children,
}: {
    children: React.ReactNode;
}) {
    return children;
}
