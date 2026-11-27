import { AuthShell } from '@/app/(auth)/_components/shell';

export default function PlatformAuthLayout({
    children,
}: {
    children: React.ReactNode;
}) {
    return <AuthShell>{children}</AuthShell>;
}
