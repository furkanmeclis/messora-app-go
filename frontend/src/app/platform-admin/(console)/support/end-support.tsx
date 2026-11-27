'use client';

import { useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { notify } from '@/components/notify';
import { Button } from '@/components/ui/button';
import { useT } from '@/i18n/client';
import { endSupportAction } from '../../actions';

export function EndSupport({ id }: { id: string }) {
    const t = useT();
    const router = useRouter();
    const [pending, start] = useTransition();
    return (
        <Button
            size="sm"
            variant="outline"
            disabled={pending}
            onClick={() =>
                start(async () => {
                    if (notify(await endSupportAction(id))) router.refresh();
                })
            }
        >
            {t('platform.support.end')}
        </Button>
    );
}
