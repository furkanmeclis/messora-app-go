import type { Metadata } from 'next';
import Link from 'next/link';
import {
    Database,
    HardDrive,
    Radio,
    Server,
    ShieldAlert,
    Timer,
    Workflow,
} from 'lucide-react';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { api, ApiError } from '@/lib/server/api';
import { can, requireMe } from '@/lib/server/auth';
import type { SystemStatus } from '@/lib/types';
import { PageHeading } from './panel-header';
import { RefreshButton } from './refresh-button';

export const metadata: Metadata = { title: 'Overview' };

type Component = SystemStatus['components'][string];

const COMPONENTS: {
    key: string;
    label: string;
    description: string;
    icon: React.ComponentType<{ className?: string }>;
}[] = [
    {
        key: 'postgres',
        label: 'PostgreSQL',
        description: 'Primary data store',
        icon: Database,
    },
    {
        key: 'schema',
        label: 'Database schema',
        description: 'Migrations applied',
        icon: HardDrive,
    },
    {
        key: 'redis',
        label: 'Redis',
        description: 'Queue, cache and locks',
        icon: Server,
    },
    {
        key: 'worker',
        label: 'Worker',
        description: 'Background jobs',
        icon: Workflow,
    },
    {
        key: 'scheduler',
        label: 'Scheduler',
        description: 'Periodic tasks',
        icon: Timer,
    },
    {
        key: 'centrifugo',
        label: 'Centrifugo',
        description: 'Realtime delivery',
        icon: Radio,
    },
];

function describe(key: string, c: Component | undefined): string {
    if (!c) return 'No result';
    if (c.status === 'fail') return c.error ?? 'Unavailable';
    const d = c.detail as Record<string, unknown> | Record<string, unknown>[];
    switch (key) {
        case 'schema':
            return `Version ${(d as { current: number }).current}`;
        case 'worker': {
            const w = d as { servers: number; active_tasks: number };
            return `${w.servers} process${w.servers === 1 ? '' : 'es'} · ${w.active_tasks} running`;
        }
        case 'centrifugo': {
            const n = (d as { version: string }[])[0];
            return n ? `v${n.version}` : 'Connected';
        }
        case 'postgres': {
            const p = d as { total_conns: number; max_conns: number };
            return `${p.total_conns}/${p.max_conns} connections`;
        }
        default:
            return 'Connected';
    }
}

export default async function OverviewPage() {
    const me = await requireMe();
    let status: SystemStatus | null = null;
    let statusError: string | null = null;
    if (can(me, 'settings.read')) {
        try {
            status = (await api<SystemStatus>('/system/status')).data;
        } catch (err) {
            if (!(err instanceof ApiError)) throw err;
            statusError = err.message;
        }
    }
    const firstName = me.user.name.split(' ')[0];

    return (
        <>
            <PageHeading
                title={`Welcome, ${firstName}`}
                description="Messora is being set up phase by phase. Messaging features arrive with the next releases."
                actions={status && <RefreshButton />}
            />

            {!me.user.mfa_enabled && (
                <Alert className="mb-6">
                    <ShieldAlert />
                    <AlertTitle>Protect your account</AlertTitle>
                    <AlertDescription>
                        <p>
                            Turn on two-factor authentication so a stolen
                            password is not enough to sign in.
                        </p>
                        <Button asChild size="sm" className="mt-2">
                            <Link href="/account/security">Set up now</Link>
                        </Button>
                    </AlertDescription>
                </Alert>
            )}

            {statusError && (
                <Alert variant="destructive" className="mb-6">
                    <AlertTitle>System status unavailable</AlertTitle>
                    <AlertDescription>{statusError}</AlertDescription>
                </Alert>
            )}

            {status && (
                <section aria-labelledby="system-status">
                    <div className="mb-3 flex items-center gap-3">
                        <h2 id="system-status" className="font-medium">
                            System status
                        </h2>
                        <Badge
                            variant={
                                status.healthy ? 'secondary' : 'destructive'
                            }
                        >
                            {status.healthy
                                ? 'All systems healthy'
                                : 'Attention needed'}
                        </Badge>
                        <span className="text-xs text-muted-foreground">
                            v{status.version}
                        </span>
                    </div>
                    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                        {COMPONENTS.map(
                            ({ key, label, description, icon: Icon }) => {
                                const c = status.components[key];
                                const ok = c?.status === 'ok';
                                return (
                                    <Card key={key}>
                                        <CardHeader className="flex flex-row items-start justify-between gap-2">
                                            <div className="space-y-1">
                                                <CardTitle className="flex items-center gap-2 text-base">
                                                    <Icon className="size-4 text-muted-foreground" />
                                                    {label}
                                                </CardTitle>
                                                <CardDescription>
                                                    {description}
                                                </CardDescription>
                                            </div>
                                            <Badge
                                                variant={
                                                    ok
                                                        ? 'secondary'
                                                        : 'destructive'
                                                }
                                            >
                                                {ok ? 'Healthy' : 'Failing'}
                                            </Badge>
                                        </CardHeader>
                                        <CardContent>
                                            <p
                                                className={
                                                    ok
                                                        ? 'text-sm text-muted-foreground'
                                                        : 'text-sm text-destructive'
                                                }
                                            >
                                                {describe(key, c)}
                                            </p>
                                        </CardContent>
                                    </Card>
                                );
                            },
                        )}
                    </div>
                </section>
            )}
        </>
    );
}
