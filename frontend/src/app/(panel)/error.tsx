'use client';

import { AlertTriangle } from 'lucide-react';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { useT } from '@/i18n/client';

/** Shown instead of a blank page when a panel page fails to render. */
export default function PanelError({
    error,
    reset,
}: {
    error: Error & { digest?: string };
    reset: () => void;
}) {
    const t = useT();
    return (
        <Alert variant="destructive" className="max-w-xl">
            <AlertTriangle />
            <AlertTitle>{t('overview.error.title')}</AlertTitle>
            <AlertDescription>
                <p>
                    {t('overview.error.body')}{' '}
                    <code>docker compose logs messora-web messora-api</code>
                    {error.digest
                        ? ` ${t('overview.error.reference', { digest: error.digest })}`
                        : ''}
                    .
                </p>
                <Button
                    size="sm"
                    variant="outline"
                    className="mt-3"
                    onClick={reset}
                >
                    {t('common.retry')}
                </Button>
            </AlertDescription>
        </Alert>
    );
}
