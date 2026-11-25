'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useEffect, useId, useRef, useState, useTransition } from 'react';
import {
    AlertCircle,
    CheckCircle2,
    Download,
    FileSpreadsheet,
    Upload,
} from 'lucide-react';
import { notify } from '@/components/notify';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    Card,
    CardContent,
    CardDescription,
    CardFooter,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import {
    Empty,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from '@/components/ui/empty';
import {
    Field,
    FieldDescription,
    FieldLabel,
    FieldLegend,
    FieldSet,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Spinner } from '@/components/ui/spinner';
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table';
import { useFormatDateTime } from '@/components/display-preferences';
import { FileDrop } from '@/components/ui/file-drop';
import { RadioCard, RadioCards } from '@/components/ui/radio-cards';
import { useT } from '@/i18n/client';
import type { MessageKey, Translate } from '@/i18n/translate';
import type { ContactImport } from '@/lib/types';
import {
    cancelImportAction,
    getImportAction,
    startImportAction,
} from '../actions';
import { TagsInput } from '../tags-input';
import { NUMBER_LOCALE } from '@/lib/format';
import { ClientDataTable } from '@/components/client-data-table';

const MAX_BYTES = 50 * 1024 * 1024;

const STATUS: Record<
    ContactImport['status'],
    'secondary' | 'outline' | 'destructive'
> = {
    uploaded: 'outline',
    queued: 'outline',
    running: 'secondary',
    completed: 'secondary',
    failed: 'destructive',
    cancelled: 'outline',
};

export function ImportStatus({ status }: { status: ContactImport['status'] }) {
    const t = useT();
    return (
        <Badge variant={STATUS[status]} data-testid="import-status">
            {t(`contacts.imports.status.${status}`)}
        </Badge>
    );
}

function ProgressBar({
    value,
    max,
    label,
}: {
    value: number;
    max: number;
    label: string;
}) {
    const pct = max > 0 ? Math.min(100, Math.round((value / max) * 100)) : 0;
    return (
        <div
            role="progressbar"
            aria-label={label}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={pct}
            className="h-2 w-full overflow-hidden rounded-full bg-muted"
        >
            <div
                className="h-full rounded-full bg-primary transition-[width] duration-300"
                style={{ width: `${pct}%` }}
            />
        </div>
    );
}

function formatBytes(n: number): string {
    if (n < 1024) return `${n} B`;
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(0)} KB`;
    return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

// ── Upload ─────────────────────────────────────────────────────────────────

type UploadError = { message: string; signedOut?: boolean };

/** Sends the file with XMLHttpRequest, which reports upload progress. */
function upload(
    t: Translate,
    file: File,
    onProgress: (loaded: number) => void,
): { done: Promise<ContactImport>; abort: () => void } {
    const xhr = new XMLHttpRequest();
    const done = new Promise<ContactImport>((resolve, reject) => {
        xhr.open(
            'POST',
            `/bff/contact-imports?filename=${encodeURIComponent(file.name)}`,
        );
        const lower = file.name.toLowerCase();
        xhr.setRequestHeader(
            'Content-Type',
            lower.endsWith('.xlsx')
                ? 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
                : 'text/csv',
        );
        xhr.upload.onprogress = (e) => onProgress(e.loaded);
        xhr.onerror = () =>
            reject({ message: t('contacts.imports.uploadNetwork') });
        xhr.onabort = () =>
            reject({ message: t('contacts.imports.uploadCancelled') });
        xhr.onload = () => {
            let body: {
                data?: ContactImport;
                error?: {
                    message?: string;
                    details?: { message?: string }[];
                };
            } | null = null;
            try {
                body = JSON.parse(xhr.responseText);
            } catch {
                body = null;
            }
            if (xhr.status === 401) {
                reject({
                    message: t('errors.SESSION_INVALID'),
                    signedOut: true,
                });
                return;
            }
            if (xhr.status === 201 && body?.data) {
                resolve(body.data);
                return;
            }
            reject({
                message:
                    body?.error?.details?.[0]?.message ??
                    body?.error?.message ??
                    t('contacts.imports.uploadFailed', { status: xhr.status }),
            });
        };
        xhr.send(file);
    });
    return { done, abort: () => xhr.abort() };
}

export function UploadCard() {
    const t = useT();
    const router = useRouter();
    const inputId = useId();
    const abort = useRef<(() => void) | null>(null);
    const [file, setFile] = useState<File | null>(null);
    const [loaded, setLoaded] = useState(0);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');

    useEffect(() => () => abort.current?.(), []);

    const choose = (f: File | undefined) => {
        setError('');
        setLoaded(0);
        if (!f) return setFile(null);
        if (!/\.(csv|txt|xlsx)$/i.test(f.name)) {
            setFile(null);
            setError(t('contacts.imports.wrongType'));
            return;
        }
        if (f.size > MAX_BYTES) {
            setFile(null);
            setError(t('contacts.imports.tooLarge'));
            return;
        }
        if (f.size === 0) {
            setFile(null);
            setError(t('contacts.imports.empty'));
            return;
        }
        setFile(f);
    };

    const start = async () => {
        if (!file) return;
        setBusy(true);
        setError('');
        const u = upload(t, file, setLoaded);
        abort.current = u.abort;
        try {
            const imp = await u.done;
            router.push(`/contacts/import/${imp.id}`);
        } catch (e) {
            const err = e as UploadError;
            if (err.signedOut) {
                router.push('/login?next=/contacts/import');
                return;
            }
            setError(err.message);
            setBusy(false);
        } finally {
            abort.current = null;
        }
    };

    return (
        <Card>
            <CardHeader>
                <CardTitle>{t('contacts.imports.step1')}</CardTitle>
                <CardDescription>
                    {t('contacts.imports.step1Hint')}
                </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
                <FileDrop
                    accept=".csv,.txt,.xlsx,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
                    file={file}
                    disabled={busy}
                    onFile={(f) => choose(f ?? undefined)}
                    hint={t('contacts.imports.formats')}
                />
                {file && (
                    <div className="space-y-2 text-sm">
                        <div className="flex items-center gap-2">
                            <FileSpreadsheet className="size-4 text-muted-foreground" />
                            <span className="font-medium">{file.name}</span>
                            <span className="text-muted-foreground">
                                {formatBytes(file.size)}
                            </span>
                        </div>
                        {busy && (
                            <>
                                <ProgressBar
                                    value={loaded}
                                    max={file.size}
                                    label={t('contacts.imports.uploadProgress')}
                                />
                                <p
                                    className="text-xs text-muted-foreground"
                                    aria-live="polite"
                                >
                                    {loaded < file.size
                                        ? t('contacts.imports.uploading', {
                                              loaded: formatBytes(loaded),
                                              total: formatBytes(file.size),
                                          })
                                        : t('contacts.imports.reading')}
                                </p>
                            </>
                        )}
                    </div>
                )}
                {error && (
                    <Alert variant="destructive" id={`${inputId}-error`}>
                        <AlertCircle />
                        <AlertDescription>{error}</AlertDescription>
                    </Alert>
                )}
            </CardContent>
            <CardFooter className="justify-end gap-2">
                {busy && (
                    <Button variant="outline" onClick={() => abort.current?.()}>
                        {t('contacts.imports.cancelUpload')}
                    </Button>
                )}
                <Button disabled={!file || busy} onClick={start}>
                    {busy ? <Spinner /> : <Upload />}{' '}
                    {t('contacts.imports.upload')}
                </Button>
            </CardFooter>
        </Card>
    );
}

// ── Recent imports ─────────────────────────────────────────────────────────

export function ImportList({ imports }: { imports: ContactImport[] }) {
    const t = useT();
    const router = useRouter();
    const formatDateTime = useFormatDateTime();
    if (imports.length === 0) {
        return (
            <Empty className="py-10">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <FileSpreadsheet />
                    </EmptyMedia>
                    <EmptyTitle>{t('contacts.imports.none')}</EmptyTitle>
                    <EmptyDescription>
                        {t('contacts.imports.noneHint')}
                    </EmptyDescription>
                </EmptyHeader>
            </Empty>
        );
    }
    return (
        <ClientDataTable
            rows={imports}
            rowId={(i) => i.id}
            storageKey="contact-imports"
            initialSort="-uploaded"
            pageSize={10}
            searchText={(i) => i.filename}
            searchPlaceholder={t('table.search')}
            noMatch={
                <p className="p-8 text-center text-sm text-muted-foreground">
                    {t('table.noMatch')}
                </p>
            }
            columns={[
                {
                    id: 'file',
                    header: t('contacts.imports.file'),
                    hideable: false,
                    sortValue: (i) => i.filename,
                    cell: (i) => (
                        <Link
                            href={`/contacts/import/${i.id}`}
                            className="font-medium hover:underline"
                        >
                            {i.filename}
                        </Link>
                    ),
                },
                {
                    id: 'status',
                    header: t('contacts.imports.statusLabel'),
                    sortValue: (i) => t(`contacts.imports.status.${i.status}`),
                    cell: (i) => <ImportStatus status={i.status} />,
                },
                {
                    id: 'rows',
                    header: t('contacts.imports.rows'),
                    className: 'text-right',
                    sortValue: (i) => i.total_rows,
                    cell: (i) => (
                        <span className="tabular-nums">
                            {i.total_rows.toLocaleString(NUMBER_LOCALE)}
                        </span>
                    ),
                },
                {
                    id: 'created',
                    header: t('contacts.imports.createdUpdated'),
                    className: 'text-right',
                    sortValue: (i) => i.created_count + i.updated_count,
                    cell: (i) => (
                        <span className="tabular-nums">
                            {i.created_count.toLocaleString(NUMBER_LOCALE)} /{' '}
                            {i.updated_count.toLocaleString(NUMBER_LOCALE)}
                        </span>
                    ),
                },
                {
                    id: 'failed',
                    header: t('contacts.imports.counters.failed'),
                    className: 'text-right',
                    sortValue: (i) => i.failed_count,
                    cell: (i) => (
                        <span className="tabular-nums">
                            {i.failed_count.toLocaleString(NUMBER_LOCALE)}
                        </span>
                    ),
                },
                {
                    id: 'uploaded',
                    header: t('contacts.imports.uploaded'),
                    sortValue: (i) => i.created_at,
                    cell: (i) => (
                        <span className="text-sm whitespace-nowrap text-muted-foreground">
                            {formatDateTime(i.created_at)}
                        </span>
                    ),
                },
            ]}
            rowMenu={(i) => [
                [
                    {
                        id: 'open',
                        label: i.filename,
                        icon: FileSpreadsheet,
                        run: () => router.push(`/contacts/import/${i.id}`),
                    },
                ],
            ]}
            onRowClick={(i) => router.push(`/contacts/import/${i.id}`)}
        />
    );
}

// ── Wizard ─────────────────────────────────────────────────────────────────

type Option = { id: string; name: string };
type TagOption = { name: string; color: string };

export function ImportWizard({
    initial,
    groups,
    tags,
}: {
    initial: ContactImport;
    groups: Option[];
    tags: TagOption[];
}) {
    const [imp, setImp] = useState(initial);
    if (imp.status === 'uploaded') {
        return (
            <MappingStep
                imp={imp}
                groups={groups}
                tags={tags}
                onStarted={setImp}
            />
        );
    }
    return <ProgressView imp={imp} onChange={setImp} />;
}

// ── Step 2: mapping ────────────────────────────────────────────────────────

const TARGETS = [
    'skip',
    'phone',
    'name',
    'email',
    'var1',
    'var2',
    'var3',
    'var4',
    'var5',
    'tags',
    'attr',
] as const;
type Target = (typeof TARGETS)[number];

const targetLabel = (t: Translate, target: Target) =>
    t(`contacts.imports.targets.${target}` as MessageKey);

function guess(header: string): Target {
    const h = header
        .trim()
        .toLowerCase()
        .replace(/[\s_-]+/g, '');
    if (
        /^(phone|phonenumber|mobile|msisdn|gsm|telefon|tel|cep|number|whatsapp)$/.test(
            h,
        )
    )
        return 'phone';
    if (/^(name|fullname|ad|adsoyad|isim|contact|contactname)$/.test(h))
        return 'name';
    if (/^(email|e-?mail|mail|eposta)$/.test(h)) return 'email';
    const v = /^var([1-5])$/.exec(h);
    if (v) return `var${v[1]}` as Target;
    if (/^(tags?|etiket(ler)?|labels?)$/.test(h)) return 'tags';
    return 'skip';
}

function attrName(header: string): string {
    return header
        .trim()
        .toLowerCase()
        .replace(/^attr:/, '')
        .replace(/[^a-z0-9]+/g, '_')
        .replace(/^_+|_+$/g, '')
        .slice(0, 60);
}

type ColumnChoice = { target: Target; attr: string };

function MappingStep({
    imp,
    groups,
    tags,
    onStarted,
}: {
    imp: ContactImport;
    groups: Option[];
    tags: TagOption[];
    onStarted: (i: ContactImport) => void;
}) {
    const t = useT();
    const [choices, setChoices] = useState<ColumnChoice[]>(() => {
        const used = new Set<Target>();
        return imp.headers.map((h) => {
            if (h.toLowerCase().startsWith('attr:')) {
                return { target: 'attr' as Target, attr: attrName(h) };
            }
            let g = guess(h);
            if (g !== 'skip' && used.has(g)) g = 'skip';
            used.add(g);
            return { target: g, attr: attrName(h) };
        });
    });
    const [policy, setPolicy] = useState<'skip' | 'update'>('skip');
    const [extraTags, setExtraTags] = useState<string[]>([]);
    const [group, setGroup] = useState('');
    const [error, setError] = useState('');
    const [pending, startTransition] = useTransition();

    const sample = (col: number) =>
        imp.preview
            .map((r) => r[col] ?? '')
            .filter((v) => v !== '')
            .slice(0, 3);

    const set = (i: number, patch: Partial<ColumnChoice>) =>
        setChoices((c) => c.map((x, j) => (j === i ? { ...x, ...patch } : x)));

    const build = (): Record<string, string> | string => {
        const mapping: Record<string, string> = {};
        for (const [i, c] of choices.entries()) {
            if (c.target === 'skip') continue;
            const header = imp.headers[i]!;
            let key: string = c.target;
            if (c.target === 'attr') {
                const n = c.attr.trim();
                if (!n) return t('contacts.imports.errAttrName', { header });
                key = `attr:${n}`;
            }
            if (mapping[key]) {
                return c.target === 'attr'
                    ? t('contacts.imports.errAttrTwice', { name: c.attr })
                    : t('contacts.imports.errTwice', {
                          target: targetLabel(t, c.target),
                      });
            }
            mapping[key] = header;
        }
        if (!mapping.phone) return t('contacts.imports.errPhone');
        return mapping;
    };

    const start = () => {
        const mapping = build();
        if (typeof mapping === 'string') {
            setError(mapping);
            return;
        }
        setError('');
        startTransition(async () => {
            const r = await startImportAction(imp.id, {
                mapping,
                duplicate_policy: policy,
                add_tags: extraTags,
                group_id: group || undefined,
            });
            if (!r.ok) {
                setError(Object.values(r.fieldErrors ?? {})[0] ?? r.message);
                return;
            }
            notify(r);
            const fresh = await getImportAction(imp.id);
            onStarted(
                fresh.ok ? fresh.data : { ...imp, status: 'queued' as const },
            );
        });
    };

    return (
        <div className="space-y-6">
            <Card>
                <CardHeader>
                    <CardTitle>{t('contacts.imports.step2')}</CardTitle>
                    <CardDescription>
                        {t('contacts.imports.rowsFound', {
                            count: imp.total_rows,
                            formatted:
                                imp.total_rows.toLocaleString(NUMBER_LOCALE),
                        })}{' '}
                        {t('contacts.imports.step2Hint')}
                    </CardDescription>
                </CardHeader>
                <CardContent className="p-0">
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>
                                    {t('contacts.imports.column')}
                                </TableHead>
                                <TableHead>
                                    {t('contacts.imports.firstValues')}
                                </TableHead>
                                <TableHead className="w-72">
                                    {t('contacts.imports.importAs')}
                                </TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {imp.headers.map((h, i) => {
                                const c = choices[i]!;
                                return (
                                    <TableRow key={`${i}-${h}`}>
                                        <TableCell className="font-medium">
                                            {h || (
                                                <span className="text-muted-foreground">
                                                    {t(
                                                        'contacts.imports.noName',
                                                    )}
                                                </span>
                                            )}
                                        </TableCell>
                                        <TableCell className="max-w-xs text-sm text-muted-foreground">
                                            <span className="line-clamp-2 break-all">
                                                {sample(i).join(' · ') || '—'}
                                            </span>
                                        </TableCell>
                                        <TableCell>
                                            <div className="flex flex-col gap-2">
                                                <Select
                                                    value={c.target}
                                                    onValueChange={(v) =>
                                                        set(i, {
                                                            target: v as Target,
                                                        })
                                                    }
                                                >
                                                    <SelectTrigger
                                                        className="w-full"
                                                        aria-label={t(
                                                            'contacts.imports.importColumnAs',
                                                            {
                                                                column:
                                                                    h || i + 1,
                                                            },
                                                        )}
                                                    >
                                                        <SelectValue />
                                                    </SelectTrigger>
                                                    <SelectContent>
                                                        {TARGETS.map((v) => (
                                                            <SelectItem
                                                                key={v}
                                                                value={v}
                                                            >
                                                                {targetLabel(
                                                                    t,
                                                                    v,
                                                                )}
                                                            </SelectItem>
                                                        ))}
                                                    </SelectContent>
                                                </Select>
                                                {c.target === 'attr' && (
                                                    <Input
                                                        aria-label={t(
                                                            'contacts.imports.attrNameFor',
                                                            {
                                                                column:
                                                                    h || i + 1,
                                                            },
                                                        )}
                                                        placeholder={t(
                                                            'contacts.imports.fieldName',
                                                        )}
                                                        maxLength={60}
                                                        value={c.attr}
                                                        onChange={(e) =>
                                                            set(i, {
                                                                attr: e.target
                                                                    .value,
                                                            })
                                                        }
                                                    />
                                                )}
                                            </div>
                                        </TableCell>
                                    </TableRow>
                                );
                            })}
                        </TableBody>
                    </Table>
                </CardContent>
            </Card>

            <Card>
                <CardHeader>
                    <CardTitle>{t('contacts.imports.step3')}</CardTitle>
                </CardHeader>
                <CardContent className="grid gap-6 md:grid-cols-2">
                    <FieldSet>
                        <FieldLegend variant="label">
                            {t('contacts.imports.duplicates')}
                        </FieldLegend>
                        <RadioCards
                            name="duplicate_policy"
                            value={policy}
                            onValueChange={(v) =>
                                setPolicy(v as 'skip' | 'update')
                            }
                        >
                            {(['skip', 'update'] as const).map((v) => (
                                <RadioCard key={v} value={v}>
                                    <span className="font-medium">
                                        {t(`contacts.imports.policy.${v}`)}
                                    </span>
                                    <span className="block text-muted-foreground">
                                        {t(`contacts.imports.policy.${v}Hint`)}
                                    </span>
                                </RadioCard>
                            ))}
                        </RadioCards>
                    </FieldSet>
                    <div className="space-y-6">
                        <Field>
                            <FieldLabel htmlFor="import-tags">
                                {t('contacts.imports.tagAll')}
                            </FieldLabel>
                            <TagsInput
                                id="import-tags"
                                value={extraTags}
                                onChange={setExtraTags}
                                suggestions={tags}
                                max={10}
                            />
                            <FieldDescription>
                                {t('contacts.imports.tagAllHint')}
                            </FieldDescription>
                        </Field>
                        <Field>
                            <FieldLabel htmlFor="import-group">
                                {t('contacts.imports.groupAll')}
                            </FieldLabel>
                            <Select
                                value={group || 'none'}
                                onValueChange={(v) =>
                                    setGroup(v === 'none' ? '' : v)
                                }
                            >
                                <SelectTrigger
                                    id="import-group"
                                    className="w-full"
                                >
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="none">
                                        {t('contacts.imports.noGroup')}
                                    </SelectItem>
                                    {groups.map((g) => (
                                        <SelectItem key={g.id} value={g.id}>
                                            {g.name}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </Field>
                    </div>
                </CardContent>
                <CardFooter className="flex-col items-stretch gap-3">
                    {error && (
                        <Alert variant="destructive">
                            <AlertCircle />
                            <AlertDescription role="alert">
                                {error}
                            </AlertDescription>
                        </Alert>
                    )}
                    <div className="flex justify-end gap-2">
                        <Button variant="outline" asChild>
                            <Link href="/contacts/import">
                                {t('common.back')}
                            </Link>
                        </Button>
                        <Button onClick={start} disabled={pending}>
                            {pending && <Spinner />}{' '}
                            {t('contacts.imports.start')}
                        </Button>
                    </div>
                </CardFooter>
            </Card>
        </div>
    );
}

// ── Step 4: progress ───────────────────────────────────────────────────────

const ACTIVE: ContactImport['status'][] = ['queued', 'running'];

function ProgressView({
    imp,
    onChange,
}: {
    imp: ContactImport;
    onChange: (i: ContactImport) => void;
}) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const [pending, startTransition] = useTransition();
    const active = ACTIVE.includes(imp.status);

    // Poll while the worker runs; stop on any finished state.
    useEffect(() => {
        if (!active) return;
        let stopped = false;
        const timer = setInterval(async () => {
            const r = await getImportAction(imp.id);
            if (!stopped && r.ok) onChange(r.data);
        }, 2000);
        return () => {
            stopped = true;
            clearInterval(timer);
        };
    }, [active, imp.id, onChange]);

    const counters = [
        ['created', imp.created_count],
        ['updated', imp.updated_count],
        ['skipped', imp.skipped_count],
        ['failed', imp.failed_count],
    ] as const;

    return (
        <Card>
            <CardHeader>
                <CardTitle className="flex items-center gap-2">
                    {imp.status === 'completed' ? (
                        <CheckCircle2 className="size-5 text-primary" />
                    ) : active ? (
                        <Spinner />
                    ) : null}
                    {active
                        ? t('contacts.imports.importing')
                        : t('contacts.imports.finished')}
                    <ImportStatus status={imp.status} />
                </CardTitle>
                <CardDescription>
                    {active
                        ? t('contacts.imports.leaveHint')
                        : imp.finished_at
                          ? t('contacts.imports.finishedAt', {
                                time: formatDateTime(imp.finished_at),
                            })
                          : null}
                </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
                <div className="space-y-2">
                    <ProgressBar
                        value={imp.processed_rows}
                        max={imp.total_rows}
                        label={t('contacts.imports.progress')}
                    />
                    <p
                        className="text-sm text-muted-foreground"
                        aria-live="polite"
                        data-testid="import-processed"
                    >
                        {t('contacts.imports.processed', {
                            done: imp.processed_rows.toLocaleString(
                                NUMBER_LOCALE,
                            ),
                            total: imp.total_rows.toLocaleString(NUMBER_LOCALE),
                        })}
                    </p>
                </div>
                <dl className="grid grid-cols-2 gap-4 sm:grid-cols-4">
                    {counters.map(([key, n]) => (
                        <div key={key} className="rounded-lg border p-3">
                            <dt className="text-xs text-muted-foreground">
                                {t(`contacts.imports.counters.${key}`)}
                            </dt>
                            <dd
                                className="text-2xl font-semibold tabular-nums"
                                data-testid={`import-${key}`}
                            >
                                {n.toLocaleString(NUMBER_LOCALE)}
                            </dd>
                        </div>
                    ))}
                </dl>
                {imp.last_error && (
                    <Alert variant="destructive">
                        <AlertCircle />
                        <AlertTitle>{t('contacts.imports.stopped')}</AlertTitle>
                        <AlertDescription>{imp.last_error}</AlertDescription>
                    </Alert>
                )}
            </CardContent>
            <CardFooter className="flex-wrap justify-end gap-2">
                {imp.has_error_report && (
                    <Button variant="outline" asChild>
                        <a
                            href={`/bff/contact-imports/${imp.id}/errors`}
                            download
                        >
                            <Download /> {t('contacts.imports.downloadFailed')}
                        </a>
                    </Button>
                )}
                {active ? (
                    <Button
                        variant="destructive"
                        disabled={pending}
                        onClick={() =>
                            startTransition(async () => {
                                const r = await cancelImportAction(imp.id);
                                if (notify(r)) onChange(r.data);
                            })
                        }
                    >
                        {pending && <Spinner />} {t('contacts.imports.cancel')}
                    </Button>
                ) : (
                    <>
                        <Button variant="outline" asChild>
                            <Link href="/contacts/import">
                                {t('contacts.imports.another')}
                            </Link>
                        </Button>
                        <Button asChild>
                            <Link href="/contacts">
                                {t('contacts.imports.viewContacts')}
                            </Link>
                        </Button>
                    </>
                )}
            </CardFooter>
        </Card>
    );
}
