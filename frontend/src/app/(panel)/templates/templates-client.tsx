'use client';

import { useState, useTransition } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import {
    AlertCircle,
    Braces,
    FileText,
    Image as ImageIcon,
    Info,
    Layers,
    Megaphone,
    Plus,
    RefreshCw,
    Search,
    Trash2,
    X,
} from 'lucide-react';
import Link from 'next/link';
import { toast } from 'sonner';
import { notify } from '@/components/notify';
import { useReauth } from '@/components/reauth';
import { Alert, AlertDescription } from '@/components/ui/alert';
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
    AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import {
    Empty,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from '@/components/ui/empty';
import { Input } from '@/components/ui/input';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Spinner } from '@/components/ui/spinner';
import { useFormatDateTime } from '@/components/display-preferences';
import { useLocale, useT } from '@/i18n/client';
import type { Translate } from '@/i18n/translate';
import {
    TEMPLATE_STATUSES,
    isMediaHeader,
    parseComponents,
    shapeOf,
    templateKey,
} from '@/lib/templates';
import {
    Sheet,
    SheetContent,
    SheetDescription,
    SheetHeader,
    SheetTitle,
} from '@/components/ui/sheet';
import { syncAccountAction } from '../channels/actions';
import { STATUS_TONE, TEMPLATE_STATUS_ICON } from '@/lib/status-icons';
import type { Template } from '@/lib/types';
import { deleteTemplateAction } from './actions';
import { WhatsAppPreview } from '@/components/whatsapp-preview';

type Option = { id: string; name: string };

const ALL = 'all';

/** A Meta status in the user's language; unknown values as Meta sends them. */
function statusLabel(t: Translate, s: string): string {
    return (TEMPLATE_STATUSES as readonly string[]).includes(s)
        ? t(`templates.statuses.${s as (typeof TEMPLATE_STATUSES)[number]}`)
        : s.charAt(0) + s.slice(1).toLowerCase();
}

export function TemplateStatusBadge({ status }: { status: string }) {
    const t = useT();
    const s = status.toUpperCase();
    const variant =
        s === 'APPROVED'
            ? 'secondary'
            : s === 'REJECTED' || s === 'DISABLED'
              ? 'destructive'
              : 'outline';
    return (
        <Badge variant={variant} data-status={s}>
            {statusLabel(t, s)}
        </Badge>
    );
}

// ── Filters ───────────────────────────────────────────────────────────────

export function TemplateFilters({
    accounts,
    account,
    status,
}: {
    accounts: Option[];
    account: string;
    status: string;
}) {
    const t = useT();
    const router = useRouter();
    const pathname = usePathname();
    const go = (next: { account?: string; status?: string }) => {
        const q = new URLSearchParams();
        const a = next.account ?? account;
        const s = next.status ?? status;
        if (a && a !== ALL) q.set('account', a);
        if (s && s !== ALL) q.set('status', s);
        const qs = q.toString();
        router.push(qs ? `${pathname}?${qs}` : pathname);
    };
    return (
        <div className="flex flex-wrap items-center gap-2">
            {accounts.length > 1 && (
                <Select
                    value={account || ALL}
                    onValueChange={(v) => go({ account: v })}
                >
                    <SelectTrigger
                        className="w-56"
                        aria-label={t('templates.filters.account')}
                    >
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value={ALL}>
                            {t('templates.filters.allAccounts')}
                        </SelectItem>
                        {accounts.map((a) => (
                            <SelectItem key={a.id} value={a.id}>
                                {a.name}
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>
            )}
            <Select
                value={status || ALL}
                onValueChange={(v) => go({ status: v })}
            >
                <SelectTrigger
                    className="w-44"
                    aria-label={t('templates.filters.status')}
                >
                    <SelectValue />
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value={ALL}>
                        <Layers className="text-muted-foreground" />
                        {t('templates.filters.allStatuses')}
                    </SelectItem>
                    {TEMPLATE_STATUSES.map((s) => {
                        const Icon = TEMPLATE_STATUS_ICON[s] ?? Layers;
                        return (
                            <SelectItem key={s} value={s}>
                                <Icon className={STATUS_TONE[s]} />
                                {statusLabel(t, s)}
                            </SelectItem>
                        );
                    })}
                </SelectContent>
            </Select>
            {(account || status) && (
                <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => router.push(pathname)}
                >
                    <X /> {t('templates.filters.clear')}
                </Button>
            )}
        </div>
    );
}

// ── List ──────────────────────────────────────────────────────────────────

export function TemplateList({
    templates,
    manage,
    canCampaign,
    filtered,
    hasAccounts,
}: {
    templates: Template[];
    manage: boolean;
    canCampaign: boolean;
    filtered: boolean;
    hasAccounts: boolean;
}) {
    const t = useT();
    const locale = useLocale();
    const [q, setQ] = useState('');
    const [cat, setCat] = useState('all');
    const [lang, setLang] = useState('all');
    const [head, setHead] = useState('all');
    const [open, setOpen] = useState<Template | null>(null);
    const category = (c: string) =>
        c in CATEGORIES
            ? t(`templates.categories.${c as Category}`).toLocaleLowerCase(
                  locale,
              )
            : c.toLowerCase();
    const languages = [...new Set(templates.map((x) => x.language))].sort();
    const shown = templates.filter((x) => {
        const sh = shapeOf(x);
        if (
            q &&
            !`${x.name} ${x.language}`
                .toLowerCase()
                .includes(q.trim().toLowerCase())
        )
            return false;
        if (cat !== 'all' && x.category.toUpperCase() !== cat) return false;
        if (lang !== 'all' && x.language !== lang) return false;
        if (
            head !== 'all' &&
            (head === 'media' ? !isMediaHeader(sh.header) : sh.header !== head)
        )
            return false;
        return true;
    });

    if (templates.length === 0) {
        return (
            <Card>
                <CardContent>
                    <Empty className="py-10">
                        <EmptyHeader>
                            <EmptyMedia variant="icon">
                                <FileText />
                            </EmptyMedia>
                            <EmptyTitle>
                                {filtered
                                    ? t('templates.list.emptyFiltered')
                                    : t('templates.list.empty')}
                            </EmptyTitle>
                            <EmptyDescription>
                                {filtered
                                    ? t('templates.list.emptyFilteredHint')
                                    : hasAccounts
                                      ? t('templates.list.emptyHint')
                                      : t('templates.list.emptyNoAccounts')}
                            </EmptyDescription>
                        </EmptyHeader>
                    </Empty>
                </CardContent>
            </Card>
        );
    }
    const select = (
        label: string,
        value: string,
        onChange: (v: string) => void,
        options: [string, string][],
    ) => (
        <Select value={value} onValueChange={onChange}>
            <SelectTrigger className="w-40" aria-label={label}>
                <SelectValue />
            </SelectTrigger>
            <SelectContent>
                {options.map(([v, l]) => (
                    <SelectItem key={v} value={v}>
                        {l}
                    </SelectItem>
                ))}
            </SelectContent>
        </Select>
    );
    return (
        <div className="space-y-4">
            <div className="flex flex-wrap items-center gap-2">
                <div className="relative w-full sm:w-64">
                    <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        aria-label={t('templates.list.search')}
                        placeholder={t('templates.list.search')}
                        className="pl-8"
                        value={q}
                        onChange={(e) => setQ(e.target.value)}
                    />
                </div>
                {select(t('templates.list.category'), cat, setCat, [
                    ['all', t('templates.list.allCategories')],
                    ...(Object.keys(CATEGORIES) as Category[]).map(
                        (c) =>
                            [c, t(`templates.categories.${c}`)] as [
                                string,
                                string,
                            ],
                    ),
                ])}
                {languages.length > 1 &&
                    select(t('templates.list.language'), lang, setLang, [
                        ['all', t('templates.list.allLanguages')],
                        ...languages.map((l) => [l, l] as [string, string]),
                    ])}
                {select(t('templates.list.headerType'), head, setHead, [
                    ['all', t('templates.list.allHeaders')],
                    ['none', t('templates.create.headerKinds.NONE')],
                    ['text', t('templates.create.headerKinds.TEXT')],
                    ['media', t('templates.list.mediaHeader')],
                ])}
                <span
                    className="ml-auto text-sm text-muted-foreground"
                    role="status"
                >
                    {t('templates.list.count', { count: shown.length })}
                </span>
            </div>
            {shown.length === 0 ? (
                <p className="rounded-lg border p-6 text-center text-sm text-muted-foreground">
                    {t('templates.list.emptyFiltered')}
                </p>
            ) : (
                <ul
                    className="grid gap-4 md:grid-cols-2 2xl:grid-cols-3"
                    aria-label={t('templates.list.label')}
                >
                    {shown.map((tpl) => {
                        const sh = shapeOf(tpl);
                        const vars =
                            sh.headerVars.length +
                            sh.bodyVars.length +
                            sh.urlButtons.length;
                        return (
                            <li key={tpl.id}>
                                <Card
                                    className="flex h-full flex-col gap-3 py-4"
                                    data-testid="template"
                                >
                                    <CardContent className="flex flex-1 flex-col gap-3 px-4">
                                        <div className="flex items-start justify-between gap-2">
                                            <div className="min-w-0">
                                                <h2 className="truncate font-mono text-sm font-medium">
                                                    {tpl.name}
                                                </h2>
                                                <div className="mt-1 flex flex-wrap gap-1">
                                                    <Badge variant="outline">
                                                        {tpl.language}
                                                    </Badge>
                                                    <Badge variant="outline">
                                                        {category(tpl.category)}
                                                    </Badge>
                                                    {isMediaHeader(
                                                        sh.header,
                                                    ) && (
                                                        <Badge variant="secondary">
                                                            <ImageIcon />{' '}
                                                            {t(
                                                                `params.media.${sh.header}`,
                                                            )}
                                                        </Badge>
                                                    )}
                                                    {sh.lto && (
                                                        <Badge variant="secondary">
                                                            {t(
                                                                'templates.builder.kinds.lto',
                                                            )}
                                                        </Badge>
                                                    )}
                                                    {sh.cards.length > 0 && (
                                                        <Badge variant="secondary">
                                                            {t(
                                                                'templates.builder.kinds.carousel',
                                                            )}
                                                        </Badge>
                                                    )}
                                                    {tpl.quality_score &&
                                                        tpl.quality_score.toUpperCase() !==
                                                            'UNKNOWN' && (
                                                            <Badge
                                                                variant="outline"
                                                                data-quality={tpl.quality_score.toUpperCase()}
                                                            >
                                                                {t(
                                                                    'templates.list.qualityLabel',
                                                                )}
                                                                :{' '}
                                                                {tpl.quality_score.toUpperCase() in
                                                                QUALITIES
                                                                    ? t(
                                                                          `templates.list.qualities.${tpl.quality_score.toUpperCase() as keyof typeof QUALITIES}`,
                                                                      )
                                                                    : tpl.quality_score.toLowerCase()}
                                                            </Badge>
                                                        )}
                                                    {vars > 0 && (
                                                        <Badge variant="secondary">
                                                            <Braces />{' '}
                                                            {t(
                                                                'campaigns.wizard.variableCount',
                                                                { count: vars },
                                                            )}
                                                        </Badge>
                                                    )}
                                                </div>
                                            </div>
                                            <TemplateStatusBadge
                                                status={tpl.status}
                                            />
                                        </div>
                                        {tpl.status.toUpperCase() ===
                                            'REJECTED' &&
                                            tpl.rejected_reason && (
                                                <Alert variant="destructive">
                                                    <AlertCircle />
                                                    <AlertDescription>
                                                        {t(
                                                            'templates.list.rejected',
                                                            {
                                                                reason: tpl.rejected_reason,
                                                            },
                                                        )}
                                                    </AlertDescription>
                                                </Alert>
                                            )}
                                        <WhatsAppPreview
                                            components={tpl.components}
                                            category={tpl.category}
                                            mediaSrc={sh.exampleMedia}
                                            className="flex-1"
                                        />
                                        <div className="flex flex-wrap items-center gap-2">
                                            {canCampaign &&
                                                tpl.status.toUpperCase() ===
                                                    'APPROVED' && (
                                                    <Button asChild size="sm">
                                                        <Link
                                                            href={`/campaigns/new?template=${encodeURIComponent(templateKey(tpl))}`}
                                                        >
                                                            <Megaphone />{' '}
                                                            {t(
                                                                'templates.list.useInCampaign',
                                                            )}
                                                        </Link>
                                                    </Button>
                                                )}
                                            <Button
                                                size="sm"
                                                variant="outline"
                                                onClick={() => setOpen(tpl)}
                                            >
                                                <Info />{' '}
                                                {t('templates.list.details')}
                                            </Button>
                                            {manage && (
                                                <span className="ml-auto">
                                                    <DeleteTemplate
                                                        template={tpl}
                                                    />
                                                </span>
                                            )}
                                        </div>
                                    </CardContent>
                                </Card>
                            </li>
                        );
                    })}
                </ul>
            )}
            <TemplateDetails template={open} onClose={() => setOpen(null)} />
        </div>
    );
}

function TemplateDetails({
    template,
    onClose,
}: {
    template: Template | null;
    onClose: () => void;
}) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const tpl = template;
    const sh = tpl ? shapeOf(tpl) : null;
    const p = tpl ? parseComponents(tpl.components) : null;
    const quality = (q: string) =>
        q in QUALITIES
            ? t(`templates.list.qualities.${q as keyof typeof QUALITIES}`)
            : q.toLowerCase();
    return (
        <Sheet open={!!tpl} onOpenChange={(o) => !o && onClose()}>
            <SheetContent className="w-full overflow-y-auto sm:max-w-lg">
                {tpl && sh && p && (
                    <>
                        <SheetHeader>
                            <SheetTitle className="font-mono">
                                {tpl.name}
                            </SheetTitle>
                            <SheetDescription>
                                {tpl.language} · {tpl.category.toLowerCase()}
                                {tpl.account_name
                                    ? ` · ${tpl.account_name}`
                                    : ''}
                            </SheetDescription>
                        </SheetHeader>
                        <div className="space-y-5 px-4 pb-6">
                            <WhatsAppPreview
                                components={tpl.components}
                                category={tpl.category}
                                mediaSrc={sh.exampleMedia}
                            />
                            <dl className="grid grid-cols-[9rem_1fr] gap-x-3 gap-y-1.5 text-sm">
                                <dt className="text-muted-foreground">
                                    {t('templates.list.status')}
                                </dt>
                                <dd>
                                    <TemplateStatusBadge status={tpl.status} />
                                </dd>
                                <dt className="text-muted-foreground">
                                    {t('templates.list.headerType')}
                                </dt>
                                <dd>
                                    {t(
                                        `templates.builder.headerKinds.${sh.header.toUpperCase() as 'NONE'}`,
                                    )}
                                </dd>
                                <dt className="text-muted-foreground">
                                    {t('templates.list.parameterFormat')}
                                </dt>
                                <dd>
                                    {sh.named
                                        ? t('templates.list.named')
                                        : t('templates.list.positional')}
                                </dd>
                                {tpl.quality_score &&
                                    tpl.quality_score !== 'UNKNOWN' && (
                                        <>
                                            <dt className="text-muted-foreground">
                                                {t(
                                                    'templates.list.qualityLabel',
                                                )}
                                            </dt>
                                            <dd>
                                                {quality(
                                                    tpl.quality_score.toUpperCase(),
                                                )}
                                            </dd>
                                        </>
                                    )}
                                <dt className="text-muted-foreground">
                                    {t('templates.list.syncedLabel')}
                                </dt>
                                <dd>{formatDateTime(tpl.synced_at)}</dd>
                            </dl>
                            {(sh.headerVars.length > 0 ||
                                sh.bodyVars.length > 0 ||
                                sh.urlButtons.length > 0) && (
                                <section className="space-y-2">
                                    <h3 className="text-sm font-medium">
                                        {t('templates.list.variables')}
                                    </h3>
                                    <ul className="space-y-1 text-sm">
                                        {[
                                            ...sh.headerVars.map((v) => [
                                                `header:${v}`,
                                                t('params.header', { name: v }),
                                            ]),
                                            ...sh.bodyVars.map((v) => [
                                                `body:${v}`,
                                                t('params.body', { name: v }),
                                            ]),
                                            ...sh.urlButtons.map((i) => [
                                                `button:${i}`,
                                                t('params.button', {
                                                    n: i + 1,
                                                }),
                                            ]),
                                        ].map(([k, label]) => (
                                            <li
                                                key={k}
                                                className="flex justify-between gap-3 rounded-md bg-muted/40 px-2 py-1"
                                            >
                                                <span>{label}</span>
                                                <span className="truncate text-muted-foreground">
                                                    {sh.examples[k!] ?? '—'}
                                                </span>
                                            </li>
                                        ))}
                                    </ul>
                                </section>
                            )}
                            {tpl.rejected_reason && (
                                <Alert variant="destructive">
                                    <AlertCircle />
                                    <AlertDescription>
                                        {t('templates.list.rejected', {
                                            reason: tpl.rejected_reason,
                                        })}
                                    </AlertDescription>
                                </Alert>
                            )}
                        </div>
                    </>
                )}
            </SheetContent>
        </Sheet>
    );
}

/** Pulls templates (and numbers) from Meta for every account. */
export function SyncTemplatesButton({ accounts }: { accounts: Option[] }) {
    const t = useT();
    const router = useRouter();
    const { guard } = useReauth();
    const [pending, start] = useTransition();
    return (
        <Button
            variant="outline"
            shortcut="s"
            disabled={pending || accounts.length === 0}
            onClick={() =>
                start(async () => {
                    let templates = 0;
                    for (const a of accounts) {
                        const r = await guard(() => syncAccountAction(a.id));
                        if (!r.ok) {
                            toast.error(`${a.name}: ${r.message}`);
                            continue;
                        }
                        templates += r.data.templates;
                    }
                    toast.success(t('templates.synced', { count: templates }));
                    router.refresh();
                })
            }
        >
            {pending ? <Spinner /> : <RefreshCw />}{' '}
            {t('templates.syncFromMeta')}
        </Button>
    );
}

function DeleteTemplate({ template }: { template: Template }) {
    const t = useT();
    const names = { name: template.name, language: template.language };
    const { guard } = useReauth();
    const [open, setOpen] = useState(false);
    const [pending, startTransition] = useTransition();
    return (
        <AlertDialog open={open} onOpenChange={setOpen}>
            <AlertDialogTrigger asChild>
                <Button
                    size="icon-sm"
                    variant="ghost"
                    aria-label={t('templates.delete.label', names)}
                >
                    <Trash2 />
                </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
                <AlertDialogHeader>
                    <AlertDialogTitle>
                        {t('templates.delete.title', names)}
                    </AlertDialogTitle>
                    <AlertDialogDescription>
                        {t('templates.delete.description')}
                    </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                    <AlertDialogCancel disabled={pending}>
                        {t('common.cancel')}
                    </AlertDialogCancel>
                    <AlertDialogAction
                        variant="destructive"
                        disabled={pending}
                        onClick={(e) => {
                            e.preventDefault();
                            startTransition(async () => {
                                if (
                                    notify(
                                        await guard(() =>
                                            deleteTemplateAction(template.id),
                                        ),
                                    )
                                ) {
                                    setOpen(false);
                                }
                            });
                        }}
                    >
                        {pending && <Spinner />} {t('common.delete')}
                    </AlertDialogAction>
                </AlertDialogFooter>
            </AlertDialogContent>
        </AlertDialog>
    );
}

// ── Create ────────────────────────────────────────────────────────────────

const CATEGORIES = { MARKETING: 1, UTILITY: 1, AUTHENTICATION: 1 } as const;
type Category = keyof typeof CATEGORIES;

const QUALITIES = {
    HIGH: 1,
    MEDIUM: 1,
    LOW: 1,
    GREEN: 1,
    YELLOW: 1,
    RED: 1,
} as const;

/** Opens the template builder (/templates/new). */
export function NewTemplateButton() {
    const t = useT();
    const router = useRouter();
    return (
        <Button
            shortcut="n"
            onClick={() => router.push('/templates/new')}
            data-testid="new-template"
        >
            <Plus /> {t('templates.create.open')}
        </Button>
    );
}
