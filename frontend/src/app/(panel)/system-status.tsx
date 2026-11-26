import Link from 'next/link';
import {
    Database,
    HardDrive,
    Radio,
    Server,
    ServerCrash,
    Timer,
    Workflow,
} from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import type { MessageKey, Translate } from '@/i18n/translate';
import type { SystemStatus } from '@/lib/types';

type Component = SystemStatus['components'][string];

const COMPONENTS: {
    key: string;
    /** A product name, or a key for a translated name. */
    label: string | MessageKey;
    description: MessageKey;
    icon: React.ComponentType<{ className?: string }>;
}[] = [
    {
        key: 'postgres',
        label: 'PostgreSQL',
        description: 'overview.components.postgres',
        icon: Database,
    },
    {
        key: 'schema',
        label: 'overview.components.schemaLabel',
        description: 'overview.components.schema',
        icon: HardDrive,
    },
    {
        key: 'redis',
        label: 'Redis',
        description: 'overview.components.redis',
        icon: Server,
    },
    {
        key: 'worker',
        label: 'overview.components.workerLabel',
        description: 'overview.components.worker',
        icon: Workflow,
    },
    {
        key: 'scheduler',
        label: 'overview.components.schedulerLabel',
        description: 'overview.components.scheduler',
        icon: Timer,
    },
    {
        key: 'centrifugo',
        label: 'Centrifugo',
        description: 'overview.components.centrifugo',
        icon: Radio,
    },
];

function describe(t: Translate, key: string, c: Component | undefined): string {
    if (!c) return t('overview.status.noResult');
    if (c.status === 'fail') return c.error ?? t('overview.status.failed');
    const d = c.detail as Record<string, unknown> | Record<string, unknown>[];
    switch (key) {
        case 'schema':
            return t('overview.status.version', {
                version: (d as { current: number }).current,
            });
        case 'worker': {
            const w = d as { servers: number; active_tasks: number };
            return `${t('overview.status.worker', { count: w.servers })} · ${t('overview.status.running', { count: w.active_tasks })}`;
        }
        case 'centrifugo': {
            const n = (d as { version: string }[])[0];
            return n ? `v${n.version}` : t('overview.status.connected');
        }
        case 'postgres': {
            const p = d as { total_conns: number; max_conns: number };
            return t('overview.status.connections', {
                used: p.total_conns,
                max: p.max_conns,
            });
        }
        default:
            return t('overview.status.connected');
    }
}

/** Health of every component (the overview's System tab). */
export function SystemStatusSection({
    status,
    t,
}: {
    status: SystemStatus;
    t: Translate;
}) {
    return (
        <section aria-labelledby="system-status">
            <div className="mb-3 flex items-center gap-3">
                <h2 id="system-status" className="font-medium">
                    {t('overview.status.title')}
                </h2>
                <Badge variant={status.healthy ? 'secondary' : 'destructive'}>
                    {status.healthy
                        ? t('overview.status.allHealthy')
                        : t('overview.status.attention')}
                </Badge>
                {status.failed_jobs > 0 && (
                    <Badge variant="destructive" asChild>
                        <Link
                            href="/system/failed-jobs"
                            data-testid="failed-jobs-badge"
                        >
                            <ServerCrash />
                            {t('overview.status.failedJobs', {
                                count: status.failed_jobs,
                            })}
                        </Link>
                    </Badge>
                )}
                <span className="text-xs text-muted-foreground">
                    v{status.version}
                </span>
            </div>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {COMPONENTS.map(({ key, label, description, icon: Icon }) => {
                    const c = status.components[key];
                    const ok = c?.status === 'ok';
                    return (
                        <Card key={key}>
                            <CardHeader className="flex flex-row items-start justify-between gap-2">
                                <div className="space-y-1">
                                    <CardTitle className="flex items-center gap-2 text-base">
                                        <Icon className="size-4 text-muted-foreground" />
                                        {label.startsWith('overview.')
                                            ? t(label as MessageKey)
                                            : label}
                                    </CardTitle>
                                    <CardDescription>
                                        {t(description)}
                                    </CardDescription>
                                </div>
                                <Badge
                                    variant={ok ? 'secondary' : 'destructive'}
                                >
                                    {ok
                                        ? t('overview.status.healthy')
                                        : t('overview.status.failing')}
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
                                    {describe(t, key, c)}
                                </p>
                            </CardContent>
                        </Card>
                    );
                })}
            </div>
        </section>
    );
}
