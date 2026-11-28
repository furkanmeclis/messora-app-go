import {
    BriefcaseBusiness,
    Check,
    Crown,
    Headset,
    ShieldCheck,
    type LucideIcon,
} from 'lucide-react';
import { getT } from '@/i18n/server';
import type { MessageKey } from '@/i18n/translate';
import type { RolesAndPermissions } from '@/lib/types';
import { cn } from '@/lib/utils';
import { roleLabel } from '@/i18n/roles';

const ICON: Record<string, LucideIcon> = {
    owner: Crown,
    admin: ShieldCheck,
    manager: BriefcaseBusiness,
    agent: Headset,
};

/** What each role may do, as cards with the areas it reaches. */
export async function RoleCards({ data }: { data: RolesAndPermissions }) {
    const t = await getT();
    const total = data.permissions.length;
    const allAreas = Array.from(
        new Set(data.permissions.map((p) => p.key.split('.')[0]!)),
    ).sort();
    return (
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            {data.roles.map((r) => {
                const Icon = ICON[r.key] ?? ShieldCheck;
                const areas = new Set(
                    r.permissions.map((p) => p.split('.')[0]!),
                );
                const pct = total
                    ? Math.round((r.permissions.length / total) * 100)
                    : 0;
                return (
                    <div
                        key={r.key}
                        className="flex flex-col gap-3 rounded-2xl border bg-card p-4 shadow-xs"
                    >
                        <div className="flex items-center gap-3">
                            <span className="flex size-10 items-center justify-center rounded-xl bg-primary/10 text-primary">
                                <Icon className="size-5" />
                            </span>
                            <div className="min-w-0">
                                <p className="font-semibold">
                                    {roleLabel(t, r)}
                                </p>
                                <p className="text-xs text-muted-foreground tabular-nums">
                                    {t('users.permissionCount', {
                                        count: r.permissions.length,
                                        total,
                                    })}
                                </p>
                            </div>
                        </div>
                        <div className="h-1.5 overflow-hidden rounded-full bg-muted">
                            <div
                                className="h-full rounded-full bg-primary"
                                style={{ width: `${pct}%` }}
                            />
                        </div>
                        <p className="text-sm text-muted-foreground">
                            {t(`users.roleSummary.${r.key}` as MessageKey)}
                        </p>
                        <ul className="mt-auto flex flex-wrap gap-1.5">
                            {allAreas.map((a) => {
                                const on = areas.has(a);
                                return (
                                    <li
                                        key={a}
                                        className={cn(
                                            'flex items-center gap-1 rounded-full border px-2 py-0.5 text-[11px]',
                                            on
                                                ? 'border-primary/20 bg-primary/5 text-foreground'
                                                : 'text-muted-foreground/60 line-through decoration-muted-foreground/30',
                                        )}
                                    >
                                        {on && (
                                            <Check className="size-3 text-primary" />
                                        )}
                                        {t(`users.areas.${a}` as MessageKey)}
                                    </li>
                                );
                            })}
                        </ul>
                    </div>
                );
            })}
        </div>
    );
}
