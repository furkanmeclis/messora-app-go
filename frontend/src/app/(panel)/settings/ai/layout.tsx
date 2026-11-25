import { requirePermission } from '@/lib/server/auth';
import { AITabs } from './ai-tabs';

export default async function AISettingsLayout({
    children,
}: {
    children: React.ReactNode;
}) {
    await requirePermission('ai.read');
    return (
        <>
            <AITabs />
            {children}
        </>
    );
}
