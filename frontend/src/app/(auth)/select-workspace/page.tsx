import type { Metadata } from 'next';
import Link from 'next/link';
import { ArrowRight, Building2, LayoutGrid } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { requireMe } from '@/lib/server/auth';
import type { Membership } from '@/lib/auth/contracts';
import { cn } from '@/lib/utils';
import { FormAlert } from '../_components/fields';
import { AuthCard } from '../_components/shell';
import { switchWorkspaceAction } from '../actions';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('auth.workspace.metaTitle') };
}

/** The workspaces this person belongs to. */
async function memberships(): Promise<Membership[]> {
    return (await api<Membership[]>('/me/memberships')).data;
}

function initials(name: string) {
    return (
        name
            .split(/\s+/)
            .filter(Boolean)
            .slice(0, 2)
            .map((w) => w[0]!.toLocaleUpperCase('tr'))
            .join('') || '?'
    );
}

export default async function SelectWorkspacePage() {
    await requireMe();
    const [t, list] = await Promise.all([getT(), memberships()]);

    if (list.length <= 1) {
        return (
            <AuthCard
                icon={LayoutGrid}
                title={t('auth.workspace.unavailableTitle')}
            >
                <div className="flex flex-col gap-5">
                    <FormAlert tone="info">
                        <span>{t('auth.workspace.unavailableBody')}</span>
                    </FormAlert>
                    <Button
                        asChild
                        className="h-11 w-full rounded-xl text-[15px] font-semibold"
                    >
                        <Link href="/">{t('auth.workspace.continue')}</Link>
                    </Button>
                </div>
            </AuthCard>
        );
    }

    return (
        <AuthCard
            icon={LayoutGrid}
            title={t('auth.workspace.title')}
            description={t('auth.workspace.description')}
            footer={
                <a
                    href="/logout"
                    className="inline-flex min-h-11 items-center rounded-md font-medium text-primary underline-offset-4 hover:underline"
                >
                    {t('auth.workspace.signOut')}
                </a>
            }
        >
            <ul className="flex flex-col gap-3">
                {list.map((m) => (
                    <li key={m.tenant.id}>
                        <form action={switchWorkspaceAction}>
                            <input
                                type="hidden"
                                name="tenant_id"
                                value={m.tenant.id}
                            />
                            <input type="hidden" name="url" value={m.url} />
                            <button
                                type="submit"
                                className={cn(
                                    'group flex min-h-16 w-full items-center gap-4 rounded-xl border bg-background p-3 pr-4 text-left transition-colors hover:border-primary/40 hover:bg-primary/5 focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none dark:bg-input/20',
                                    m.current &&
                                        'border-primary/40 bg-primary/5',
                                )}
                            >
                                <span className="flex size-11 shrink-0 items-center justify-center overflow-hidden rounded-xl bg-indigo-brand font-display text-sm font-extrabold text-white">
                                    {m.tenant.logo_url ? (
                                        // eslint-disable-next-line @next/next/no-img-element -- workspace logo of any size
                                        <img
                                            src={m.tenant.logo_url}
                                            alt=""
                                            className="size-full bg-white object-contain p-1"
                                        />
                                    ) : m.tenant.name ? (
                                        initials(m.tenant.name)
                                    ) : (
                                        <Building2
                                            className="size-5"
                                            aria-hidden
                                        />
                                    )}
                                </span>
                                <span className="flex min-w-0 flex-1 flex-col">
                                    <span className="flex items-center gap-2">
                                        <span className="truncate font-semibold">
                                            {m.tenant.name}
                                        </span>
                                        {m.current && (
                                            <span className="rounded-full bg-mint px-2 py-0.5 text-[11px] font-semibold text-night">
                                                {t('auth.workspace.current')}
                                            </span>
                                        )}
                                    </span>
                                    <span className="truncate text-sm text-muted-foreground">
                                        {m.tenant.slug} ·{' '}
                                        {t('auth.workspace.role', {
                                            role: m.role.name,
                                        })}
                                    </span>
                                </span>
                                <ArrowRight
                                    className="size-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5 group-hover:text-primary"
                                    aria-hidden
                                />
                            </button>
                        </form>
                    </li>
                ))}
            </ul>
        </AuthCard>
    );
}
