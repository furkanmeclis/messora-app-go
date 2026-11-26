import type { Metadata } from 'next';
import Link from 'next/link';
import { ArrowLeft } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { Schemas } from '@/lib/types';
import { PageHeading } from '../../panel-header';
import { PresetGallery } from './presets-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('automations.presets.title') };
}

export default async function PresetsPage() {
    const me = await requirePermission('automations.read');
    const t = await getT();
    const [presets, settings] = await Promise.all([
        api<Schemas['AutomationPreset'][]>('/automations/presets'),
        can(me, 'ai.read')
            ? api<Schemas['AISettings']>('/ai/settings')
                  .then((r) => r.data.enabled)
                  .catch(() => null)
            : Promise.resolve(null),
    ]);
    return (
        <>
            <PageHeading
                title={t('automations.presets.title')}
                description={t('automations.presets.description')}
                actions={
                    <Button asChild variant="outline">
                        <Link href="/automations">
                            <ArrowLeft /> {t('automations.list.title')}
                        </Link>
                    </Button>
                }
            />
            <PresetGallery
                presets={presets.data}
                canManage={can(me, 'automations.manage')}
                aiEnabled={settings}
            />
        </>
    );
}
