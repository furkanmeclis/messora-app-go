import {
    Activity,
    Database,
    HardDrive,
    Radio,
    Server,
    Timer,
    Workflow,
} from 'lucide-react';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { StatusBadge } from '@/components/ui/status-badge';
import { ApiError, apiFetch } from '@/lib/server/api';
import { RefreshButton } from './refresh-button';

export const dynamic = 'force-dynamic';
export const metadata = { title: 'System status' };

type ComponentResult = {
    status: 'ok' | 'fail';
    detail?: unknown;
    error?: string;
};
type SystemStatus = {
    version: string;
    healthy: boolean;
    components: Record<string, ComponentResult>;
    checked_at: string;
};

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
        description: 'Background job processor',
        icon: Workflow,
    },
    {
        key: 'scheduler',
        label: 'Scheduler',
        description: 'Periodic tasks (leader)',
        icon: Timer,
    },
    {
        key: 'centrifugo',
        label: 'Centrifugo',
        description: 'Realtime delivery',
        icon: Radio,
    },
];

function describe(key: string, r: ComponentResult | undefined): string {
    if (!r) return 'No result';
    if (r.status === 'fail') return r.error ?? 'Unavailable';
    const d = r.detail as Record<string, unknown> | unknown[] | undefined;
    switch (key) {
        case 'schema':
            return `Version ${(d as { current: number }).current}`;
        case 'worker': {
            const w = d as { servers: number; active_tasks: number };
            return `${w.servers} process${w.servers === 1 ? '' : 'es'} · ${w.active_tasks} active task${w.active_tasks === 1 ? '' : 's'}`;
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

async function loadStatus(): Promise<{
    status?: SystemStatus;
    error?: ApiError;
}> {
    try {
        const { data } = await apiFetch<SystemStatus>('/api/v1/system/status');
        return { status: data };
    } catch (err) {
        if (err instanceof ApiError) return { error: err };
        throw err;
    }
}

export default async function SystemStatusPage() {
    const { status, error } = await loadStatus();

    return (
        <main className="mx-auto flex min-h-svh w-full max-w-4xl flex-col gap-6 px-4 py-10">
            <header className="flex flex-wrap items-center justify-between gap-4">
                <div className="flex items-center gap-3">
                    <div className="flex size-10 items-center justify-center rounded-lg bg-primary text-primary-foreground">
                        <Activity className="size-5" />
                    </div>
                    <div>
                        <h1 className="text-xl font-semibold">Messora</h1>
                        <p className="text-sm text-muted-foreground">
                            Installation status
                            {status ? ` · v${status.version}` : ''}
                        </p>
                    </div>
                </div>
                <div className="flex items-center gap-3">
                    {status && (
                        <StatusBadge
                            status={status.healthy ? 'active' : 'failed'}
                        />
                    )}
                    <RefreshButton />
                </div>
            </header>

            {error ? (
                <Card className="border-destructive/40">
                    <CardHeader>
                        <CardTitle>API unavailable</CardTitle>
                        <CardDescription>
                            {error.message} (request {error.requestId}). Check{' '}
                            <code>docker compose logs messora-api</code>.
                        </CardDescription>
                    </CardHeader>
                </Card>
            ) : (
                <section
                    className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3"
                    aria-label="Components"
                >
                    {COMPONENTS.map(
                        ({ key, label, description, icon: Icon }) => {
                            const r = status?.components[key];
                            const ok = r?.status === 'ok';
                            return (
                                <Card key={key}>
                                    <CardHeader className="flex flex-row items-start justify-between gap-2 space-y-0">
                                        <div className="space-y-1">
                                            <CardTitle className="flex items-center gap-2 text-base">
                                                <Icon className="size-4 text-muted-foreground" />
                                                {label}
                                            </CardTitle>
                                            <CardDescription>
                                                {description}
                                            </CardDescription>
                                        </div>
                                        <StatusBadge
                                            status={ok ? 'active' : 'failed'}
                                        />
                                    </CardHeader>
                                    <CardContent>
                                        <p
                                            className={
                                                ok
                                                    ? 'text-sm text-muted-foreground'
                                                    : 'text-sm text-destructive-foreground'
                                            }
                                        >
                                            {describe(key, r)}
                                        </p>
                                    </CardContent>
                                </Card>
                            );
                        },
                    )}
                </section>
            )}

            {status && (
                <p className="text-xs text-muted-foreground">
                    Checked {new Date(status.checked_at).toUTCString()}. The
                    setup wizard and sign-in arrive in the next release (Phase
                    2).
                </p>
            )}
        </main>
    );
}
