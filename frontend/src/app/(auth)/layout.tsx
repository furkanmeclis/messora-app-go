// The brand typefaces, bundled (no third-party request; the CSP allows
// fonts from this origin only).
import '@fontsource-variable/inter/wght.css';
import '@fontsource-variable/plus-jakarta-sans/wght.css';
import { AuthShell } from './_components/shell';

export default function AuthLayout({
    children,
}: {
    children: React.ReactNode;
}) {
    return <AuthShell>{children}</AuthShell>;
}
