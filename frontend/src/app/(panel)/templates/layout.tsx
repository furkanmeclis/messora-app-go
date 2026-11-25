import { TemplatesNav } from './templates-nav';

export default function TemplatesLayout({
    children,
}: {
    children: React.ReactNode;
}) {
    return (
        <>
            <TemplatesNav />
            {children}
        </>
    );
}
