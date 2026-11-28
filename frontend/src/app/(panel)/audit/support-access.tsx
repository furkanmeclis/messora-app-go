import { LifeBuoy } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table';
import { getLocale, getT } from '@/i18n/server';
import { intlLocale } from '@/i18n/config';
import type { Schemas } from '@/lib/types';

/**
 * When Messora support opened this workspace (support access from the
 * platform admin panel): who, why, how long and whether it could change
 * anything. Hidden while there has been none.
 */
export async function SupportAccess({
    grants,
}: {
    grants: Schemas['SupportGrant'][];
}) {
    if (grants.length === 0) return null;
    const [t, locale] = await Promise.all([getT(), getLocale()]);
    const fmt = new Intl.DateTimeFormat(intlLocale(locale), {
        dateStyle: 'medium',
        timeStyle: 'short',
    });
    return (
        <section className="mb-8 space-y-3" data-testid="support-access">
            <div>
                <h2 className="flex items-center gap-2 font-semibold tracking-tight">
                    <LifeBuoy className="size-4 text-primary" aria-hidden />
                    {t('audit.support.title')}
                </h2>
                <p className="text-sm text-muted-foreground">
                    {t('audit.support.description')}
                </p>
            </div>
            <div className="overflow-x-auto rounded-xl border">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>{t('audit.support.when')}</TableHead>
                            <TableHead>{t('audit.support.who')}</TableHead>
                            <TableHead>{t('audit.support.reason')}</TableHead>
                            <TableHead>{t('audit.support.access')}</TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {grants.map((g) => (
                            <TableRow key={g.id}>
                                <TableCell className="whitespace-nowrap tabular-nums">
                                    {fmt.format(new Date(g.created_at))} –{' '}
                                    {fmt.format(
                                        new Date(g.ended_at ?? g.expires_at),
                                    )}
                                </TableCell>
                                <TableCell>{g.admin.name}</TableCell>
                                <TableCell className="max-w-80 truncate">
                                    {g.reason}
                                </TableCell>
                                <TableCell className="space-x-1">
                                    <Badge variant="outline">
                                        {g.read_only
                                            ? t('topbar.support.readOnly')
                                            : t('topbar.support.write')}
                                    </Badge>
                                    {g.active && (
                                        <Badge>
                                            {t('audit.support.active')}
                                        </Badge>
                                    )}
                                </TableCell>
                            </TableRow>
                        ))}
                    </TableBody>
                </Table>
            </div>
        </section>
    );
}
