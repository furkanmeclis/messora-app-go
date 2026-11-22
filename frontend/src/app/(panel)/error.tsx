'use client';

import { AlertTriangle } from 'lucide-react';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';

/** Shown instead of a blank page when a panel page fails to render. */
export default function PanelError({
    error,
    reset,
}: {
    error: Error & { digest?: string };
    reset: () => void;
}) {
    return (
        <Alert variant="destructive" className="max-w-xl">
            <AlertTriangle />
            <AlertTitle>This page could not be loaded</AlertTitle>
            <AlertDescription>
                <p>
                    Something went wrong while talking to the Messora API. Try
                    again; if it keeps happening, check{' '}
                    <code>docker compose logs messora-web messora-api</code>
                    {error.digest ? ` (reference ${error.digest})` : ''}.
                </p>
                <Button
                    size="sm"
                    variant="outline"
                    className="mt-3"
                    onClick={reset}
                >
                    Try again
                </Button>
            </AlertDescription>
        </Alert>
    );
}
