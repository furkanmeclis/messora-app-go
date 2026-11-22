import type { Metadata } from 'next';
import { redirect } from 'next/navigation';
import { Terminal } from 'lucide-react';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { api } from '@/lib/server/api';
import type { Schemas } from '@/lib/types';
import { SetupForm } from '../forms';

export const metadata: Metadata = { title: 'Set up Messora' };

export default async function SetupPage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    const status = (
        await api<Schemas['SetupStatus']>('/setup/status', { auth: 'none' })
    ).data;
    if (!status.setup_required) redirect('/login');
    const token = (await searchParams).token ?? '';

    return (
        <Card>
            <CardHeader>
                <CardTitle>Set up Messora</CardTitle>
                <CardDescription>
                    Create the owner account for this installation.
                </CardDescription>
            </CardHeader>
            <CardContent>
                {token ? (
                    <SetupForm token={token} />
                ) : (
                    <Alert>
                        <Terminal />
                        <AlertTitle>Open the setup link</AlertTitle>
                        <AlertDescription>
                            <p>
                                The one-time link is printed in the API logs on
                                first start:
                            </p>
                            <code className="mt-2 block rounded bg-muted p-2 text-xs">
                                docker compose logs messora-api | grep setup_url
                            </code>
                            <p className="mt-2">For a new link run:</p>
                            <code className="mt-2 block rounded bg-muted p-2 text-xs">
                                docker compose exec messora-api messora setup
                                token
                            </code>
                        </AlertDescription>
                    </Alert>
                )}
            </CardContent>
        </Card>
    );
}
