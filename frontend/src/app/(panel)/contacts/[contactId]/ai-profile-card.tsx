'use client';

import { useRouter } from 'next/navigation';
import { useTransition } from 'react';
import { BadgeCheck, Sparkles } from 'lucide-react';
import { notify } from '@/components/notify';
import { useFormatDateTime } from '@/components/display-preferences';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { useT } from '@/i18n/client';
import type { Contact } from '@/lib/types';
import { verifyContactFieldsAction } from '../actions';

/** The current value of a provenance key (name, email, var1…5, attr:<key>). */
function valueOf(contact: Contact, field: string): string {
    if (field.startsWith('attr:'))
        return contact.attributes[field.slice(5)] ?? '';
    const v = (contact as unknown as Record<string, unknown>)[field];
    return typeof v === 'string' ? v : '';
}

/**
 * What the AI collected about a contact: the profile it builds for a
 * number nobody knew (status and completeness) and every value it stored,
 * marked "collected by the AI, not verified" until a person verifies it.
 */
export function AIProfileCard({
    contact,
    canUpdate,
}: {
    contact: Contact;
    canUpdate: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const formatDateTime = useFormatDateTime();
    const [pending, startTransition] = useTransition();
    const ai = (contact.field_sources ?? []).filter((s) => s.source === 'ai');
    const status = contact.profile_status;
    const building = status === 'collecting' || status === 'complete';
    if (!building && ai.length === 0) return null;

    const label = (field: string) =>
        field === 'name'
            ? t('contacts.fields.name')
            : field === 'email'
              ? t('contacts.fields.email')
              : field.startsWith('attr:')
                ? field.slice(5)
                : field;
    const verify = (fields: string[]) =>
        startTransition(async () => {
            if (notify(await verifyContactFieldsAction(contact.id, fields)))
                router.refresh();
        });
    const unverified = ai.filter((s) => !s.verified).map((s) => s.field);
    const p = contact.profile_progress;
    const pct =
        p && p.required > 0 ? Math.round((p.filled / p.required) * 100) : 100;

    return (
        <Card data-testid="ai-profile-card">
            <CardHeader>
                <CardTitle className="flex items-center gap-2">
                    <Sparkles className="size-4 text-primary" />
                    {t('contacts.aiProfile.title')}
                </CardTitle>
                <CardDescription>
                    {building
                        ? t('contacts.aiProfile.description')
                        : t('contacts.aiProfile.descriptionKnown')}
                </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
                {building && status && (
                    <div
                        className="space-y-1.5"
                        data-testid="ai-profile-progress"
                    >
                        <div className="flex items-center justify-between gap-2 text-sm">
                            <Badge
                                variant={
                                    status === 'complete'
                                        ? 'default'
                                        : 'secondary'
                                }
                            >
                                {t(`contacts.aiProfile.status.${status}`)}
                            </Badge>
                            {p && (
                                <span className="text-xs text-muted-foreground">
                                    {t('contacts.aiProfile.progress', {
                                        filled: p.filled,
                                        required: p.required,
                                    })}
                                </span>
                            )}
                        </div>
                        <div
                            className="h-1.5 overflow-hidden rounded-full bg-muted"
                            role="progressbar"
                            aria-valuemin={0}
                            aria-valuemax={100}
                            aria-valuenow={pct}
                            aria-label={t('contacts.aiProfile.title')}
                        >
                            <div
                                className="h-full rounded-full bg-primary transition-all"
                                style={{ width: `${pct}%` }}
                            />
                        </div>
                        {p && p.missing.length > 0 && (
                            <p className="text-xs text-muted-foreground">
                                {t('contacts.aiProfile.missing', {
                                    fields: p.missing.map(label).join(', '),
                                })}
                            </p>
                        )}
                    </div>
                )}
                {ai.length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                        {t('contacts.aiProfile.none')}
                    </p>
                ) : (
                    <ul className="divide-y text-sm">
                        {ai.map((s) => (
                            <li
                                key={s.field}
                                className="flex items-start justify-between gap-2 py-2"
                                data-testid={`ai-field-${s.field}`}
                            >
                                <div className="min-w-0 space-y-1">
                                    <p className="text-xs text-muted-foreground">
                                        {label(s.field)}
                                    </p>
                                    <p className="truncate font-medium">
                                        {valueOf(contact, s.field) || '—'}
                                    </p>
                                    {s.verified ? (
                                        <Badge variant="outline">
                                            <BadgeCheck />
                                            {t(
                                                'contacts.aiProfile.verifiedBadge',
                                            )}
                                        </Badge>
                                    ) : (
                                        <Badge
                                            variant="secondary"
                                            title={t(
                                                'contacts.aiProfile.collectedAt',
                                                {
                                                    time: formatDateTime(
                                                        s.collected_at,
                                                    ),
                                                },
                                            )}
                                        >
                                            <Sparkles />
                                            {t('contacts.aiProfile.unverified')}
                                        </Badge>
                                    )}
                                </div>
                                {canUpdate && !s.verified && (
                                    <Button
                                        size="sm"
                                        variant="outline"
                                        disabled={pending}
                                        onClick={() => verify([s.field])}
                                    >
                                        {t('contacts.aiProfile.verify')}
                                    </Button>
                                )}
                            </li>
                        ))}
                    </ul>
                )}
                {canUpdate && unverified.length > 1 && (
                    <Button
                        size="sm"
                        variant="secondary"
                        className="w-full"
                        disabled={pending}
                        onClick={() => verify(unverified)}
                    >
                        <BadgeCheck /> {t('contacts.aiProfile.verifyAll')}
                    </Button>
                )}
            </CardContent>
        </Card>
    );
}
