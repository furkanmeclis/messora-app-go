'use client';

import { RefreshCw } from 'lucide-react';
import { useRouter } from 'next/navigation';
import { useTransition } from 'react';
import { Button } from '@/components/ui/button';
import { useT } from '@/i18n/client';

export function RefreshButton() {
    const t = useT();
    const router = useRouter();
    const [pending, startTransition] = useTransition();
    return (
        <Button
            variant="outline"
            size="sm"
            disabled={pending}
            onClick={() => startTransition(() => router.refresh())}
        >
            <RefreshCw className={pending ? 'size-4 animate-spin' : 'size-4'} />
            {t('overview.refresh')}
        </Button>
    );
}
