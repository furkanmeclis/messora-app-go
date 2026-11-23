'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useEffect, useId, useRef, useState, useTransition } from 'react';
import {
    AlertCircle,
    CheckCircle2,
    Download,
    FileSpreadsheet,
    FileUp,
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
import { Label } from '@/components/ui/label';
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
import { cn } from '@/lib/utils';
import type { ContactImport } from '@/lib/types';
import {
    cancelImportAction,
    getImportAction,
    startImportAction,
} from '../actions';
import { TagsInput } from '../tags-input';
import { NUMBER_LOCALE } from '@/lib/format';

const MAX_BYTES = 50 * 1024 * 1024;

const STATUS: Record<
    ContactImport['status'],
    { label: string; variant: 'secondary' | 'outline' | 'destructive' }
> = {
    uploaded: { label: 'Waiting for mapping', variant: 'outline' },
    queued: { label: 'Queued', variant: 'outline' },
    running: { label: 'Importing', variant: 'secondary' },
    completed: { label: 'Completed', variant: 'secondary' },
    failed: { label: 'Failed', variant: 'destructive' },
    cancelled: { label: 'Cancelled', variant: 'outline' },
};

export function ImportStatus({ status }: { status: ContactImport['status'] }) {
    const s = STATUS[status];
    return (
        <Badge variant={s.variant} data-testid="import-status">
            {s.label}
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
            reject({ message: 'The upload failed. Check your connection.' });
        xhr.onabort = () => reject({ message: 'Upload cancelled.' });
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
                    message: 'Your session has ended. Sign in again.',
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
                    `The upload failed (${xhr.status}).`,
            });
        };
        xhr.send(file);
    });
    return { done, abort: () => xhr.abort() };
}

export function UploadCard() {
    const router = useRouter();
    const inputId = useId();
    const input = useRef<HTMLInputElement>(null);
    const abort = useRef<(() => void) | null>(null);
    const [file, setFile] = useState<File | null>(null);
    const [loaded, setLoaded] = useState(0);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');
    const [drag, setDrag] = useState(false);

    useEffect(() => () => abort.current?.(), []);

    const choose = (f: File | undefined) => {
        setError('');
        setLoaded(0);
        if (!f) return setFile(null);
        if (!/\.(csv|txt|xlsx)$/i.test(f.name)) {
            setFile(null);
            setError('Choose a CSV (.csv) or Excel (.xlsx) file.');
            return;
        }
        if (f.size > MAX_BYTES) {
            setFile(null);
            setError(
                'The file is larger than 50 MB; split it into smaller files.',
            );
            return;
        }
        if (f.size === 0) {
            setFile(null);
            setError('The file is empty.');
            return;
        }
        setFile(f);
    };

    const start = async () => {
        if (!file) return;
        setBusy(true);
        setError('');
        const u = upload(file, setLoaded);
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
                <CardTitle>1. Upload a file</CardTitle>
                <CardDescription>
                    The first row must hold column names. Phone numbers need the
                    country code (+90…). Nothing is imported until you choose
                    the columns in the next step.
                </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
                <div
                    onDragOver={(e) => {
                        e.preventDefault();
                        setDrag(true);
                    }}
                    onDragLeave={() => setDrag(false)}
                    onDrop={(e) => {
                        e.preventDefault();
                        setDrag(false);
                        if (!busy) choose(e.dataTransfer.files[0]);
                    }}
                    className={cn(
                        'flex flex-col items-center gap-3 rounded-lg border-2 border-dashed p-8 text-center text-sm transition-colors has-focus-visible:ring-2 has-focus-visible:ring-ring',
                        drag && 'border-primary bg-muted/50',
                    )}
                >
                    <FileUp className="size-8 text-muted-foreground" />
                    <p>
                        Drop a file here or{' '}
                        <Label
                            htmlFor={inputId}
                            className="inline cursor-pointer text-primary underline-offset-4 hover:underline"
                        >
                            choose one
                        </Label>
                        .
                    </p>
                    <p className="text-xs text-muted-foreground">
                        CSV or Excel (.xlsx), up to 50 MB.
                    </p>
                    <input
                        ref={input}
                        id={inputId}
                        type="file"
                        accept=".csv,.txt,.xlsx,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
                        className="sr-only"
                        disabled={busy}
                        aria-describedby={
                            error ? `${inputId}-error` : undefined
                        }
                        onChange={(e) => choose(e.target.files?.[0])}
                    />
                </div>
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
                                    label="Upload progress"
                                />
                                <p
                                    className="text-xs text-muted-foreground"
                                    aria-live="polite"
                                >
                                    {loaded < file.size
                                        ? `Uploading… ${formatBytes(loaded)} of ${formatBytes(file.size)}`
                                        : 'Reading the columns…'}
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
                        Cancel upload
                    </Button>
                )}
                <Button disabled={!file || busy} onClick={start}>
                    {busy ? <Spinner /> : <Upload />} Upload and continue
                </Button>
            </CardFooter>
        </Card>
    );
}

// ── Recent imports ─────────────────────────────────────────────────────────

export function ImportList({ imports }: { imports: ContactImport[] }) {
    const formatDateTime = useFormatDateTime();
    if (imports.length === 0) {
        return (
            <Empty className="py-10">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <FileSpreadsheet />
                    </EmptyMedia>
                    <EmptyTitle>No imports yet</EmptyTitle>
                    <EmptyDescription>
                        Files you upload are listed here.
                    </EmptyDescription>
                </EmptyHeader>
            </Empty>
        );
    }
    return (
        <Table>
            <TableHeader>
                <TableRow>
                    <TableHead>File</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="text-right">Rows</TableHead>
                    <TableHead className="text-right">
                        Created / updated
                    </TableHead>
                    <TableHead className="text-right">Failed</TableHead>
                    <TableHead>Uploaded</TableHead>
                </TableRow>
            </TableHeader>
            <TableBody>
                {imports.map((i) => (
                    <TableRow key={i.id}>
                        <TableCell>
                            <Link
                                href={`/contacts/import/${i.id}`}
                                className="font-medium hover:underline"
                            >
                                {i.filename}
                            </Link>
                        </TableCell>
                        <TableCell>
                            <ImportStatus status={i.status} />
                        </TableCell>
                        <TableCell className="text-right tabular-nums">
                            {i.total_rows.toLocaleString(NUMBER_LOCALE)}
                        </TableCell>
                        <TableCell className="text-right tabular-nums">
                            {i.created_count.toLocaleString(NUMBER_LOCALE)} /{' '}
                            {i.updated_count.toLocaleString(NUMBER_LOCALE)}
                        </TableCell>
                        <TableCell className="text-right tabular-nums">
                            {i.failed_count.toLocaleString(NUMBER_LOCALE)}
                        </TableCell>
                        <TableCell className="text-sm whitespace-nowrap text-muted-foreground">
                            {formatDateTime(i.created_at)}
                        </TableCell>
                    </TableRow>
                ))}
            </TableBody>
        </Table>
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
    ['skip', 'Do not import'],
    ['phone', 'Phone number'],
    ['name', 'Name'],
    ['email', 'E-mail'],
    ['var1', 'Variable 1'],
    ['var2', 'Variable 2'],
    ['var3', 'Variable 3'],
    ['var4', 'Variable 4'],
    ['var5', 'Variable 5'],
    ['tags', 'Tags (separated by ; , or |)'],
    ['attr', 'Custom field'],
] as const;
type Target = (typeof TARGETS)[number][0];

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
    const [choices, setChoices] = useState<ColumnChoice[]>(() => {
        const used = new Set<Target>();
        return imp.headers.map((h) => {
            if (h.toLowerCase().startsWith('attr:')) {
                return { target: 'attr' as Target, attr: attrName(h) };
            }
            let t = guess(h);
            if (t !== 'skip' && used.has(t)) t = 'skip';
            used.add(t);
            return { target: t, attr: attrName(h) };
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
                if (!n) return `Name the custom field for “${header}”.`;
                key = `attr:${n}`;
            }
            if (mapping[key]) {
                return c.target === 'attr'
                    ? `Two columns are saved as the custom field “${c.attr}”.`
                    : `Two columns are imported as ${TARGETS.find((t) => t[0] === c.target)?.[1]}; choose one.`;
            }
            mapping[key] = header;
        }
        if (!mapping.phone)
            return 'Choose the column that holds phone numbers.';
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
                    <CardTitle>2. Choose what each column is</CardTitle>
                    <CardDescription>
                        {imp.total_rows.toLocaleString(NUMBER_LOCALE)} row
                        {imp.total_rows === 1 ? '' : 's'} found. A phone number
                        column is required; everything else is optional.
                    </CardDescription>
                </CardHeader>
                <CardContent className="p-0">
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>Column in the file</TableHead>
                                <TableHead>First values</TableHead>
                                <TableHead className="w-72">
                                    Import as
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
                                                    (no name)
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
                                                        aria-label={`Import column ${h || i + 1} as`}
                                                    >
                                                        <SelectValue />
                                                    </SelectTrigger>
                                                    <SelectContent>
                                                        {TARGETS.map(
                                                            ([v, label]) => (
                                                                <SelectItem
                                                                    key={v}
                                                                    value={v}
                                                                >
                                                                    {label}
                                                                </SelectItem>
                                                            ),
                                                        )}
                                                    </SelectContent>
                                                </Select>
                                                {c.target === 'attr' && (
                                                    <Input
                                                        aria-label={`Custom field name for column ${h || i + 1}`}
                                                        placeholder="Field name"
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
                    <CardTitle>3. Options</CardTitle>
                </CardHeader>
                <CardContent className="grid gap-6 md:grid-cols-2">
                    <FieldSet>
                        <FieldLegend variant="label">
                            When a phone number already exists
                        </FieldLegend>
                        <div className="space-y-2 text-sm">
                            {(
                                [
                                    [
                                        'skip',
                                        'Skip the row',
                                        'Existing contacts stay as they are.',
                                    ],
                                    [
                                        'update',
                                        'Update the contact',
                                        'Filled cells replace the saved values; tags are added.',
                                    ],
                                ] as const
                            ).map(([v, label, hint]) => (
                                <label
                                    key={v}
                                    className="flex cursor-pointer items-start gap-2"
                                >
                                    <input
                                        type="radio"
                                        name="duplicate_policy"
                                        value={v}
                                        checked={policy === v}
                                        onChange={() => setPolicy(v)}
                                        className="mt-1 accent-primary"
                                    />
                                    <span>
                                        <span className="font-medium">
                                            {label}
                                        </span>
                                        <span className="block text-muted-foreground">
                                            {hint}
                                        </span>
                                    </span>
                                </label>
                            ))}
                        </div>
                    </FieldSet>
                    <div className="space-y-6">
                        <Field>
                            <FieldLabel htmlFor="import-tags">
                                Tag every imported contact
                            </FieldLabel>
                            <TagsInput
                                id="import-tags"
                                value={extraTags}
                                onChange={setExtraTags}
                                suggestions={tags}
                                max={10}
                            />
                            <FieldDescription>Up to 10 tags.</FieldDescription>
                        </Field>
                        <Field>
                            <FieldLabel htmlFor="import-group">
                                Add every imported contact to a group
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
                                        No group
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
                            <Link href="/contacts/import">Back</Link>
                        </Button>
                        <Button onClick={start} disabled={pending}>
                            {pending && <Spinner />} Start import
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
        ['Created', imp.created_count],
        ['Updated', imp.updated_count],
        ['Skipped', imp.skipped_count],
        ['Failed', imp.failed_count],
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
                    {active ? 'Importing contacts' : 'Import finished'}
                    <ImportStatus status={imp.status} />
                </CardTitle>
                <CardDescription>
                    {active
                        ? 'You can leave this page; the import keeps running.'
                        : imp.finished_at
                          ? `Finished ${formatDateTime(imp.finished_at)}.`
                          : null}
                </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
                <div className="space-y-2">
                    <ProgressBar
                        value={imp.processed_rows}
                        max={imp.total_rows}
                        label="Import progress"
                    />
                    <p
                        className="text-sm text-muted-foreground"
                        aria-live="polite"
                        data-testid="import-processed"
                    >
                        {imp.processed_rows.toLocaleString(NUMBER_LOCALE)} of{' '}
                        {imp.total_rows.toLocaleString(NUMBER_LOCALE)} rows
                        processed
                    </p>
                </div>
                <dl className="grid grid-cols-2 gap-4 sm:grid-cols-4">
                    {counters.map(([label, n]) => (
                        <div key={label} className="rounded-lg border p-3">
                            <dt className="text-xs text-muted-foreground">
                                {label}
                            </dt>
                            <dd
                                className="text-2xl font-semibold tabular-nums"
                                data-testid={`import-${label.toLowerCase()}`}
                            >
                                {n.toLocaleString(NUMBER_LOCALE)}
                            </dd>
                        </div>
                    ))}
                </dl>
                {imp.last_error && (
                    <Alert variant="destructive">
                        <AlertCircle />
                        <AlertTitle>The import stopped</AlertTitle>
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
                            <Download /> Download failed rows
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
                        {pending && <Spinner />} Cancel import
                    </Button>
                ) : (
                    <>
                        <Button variant="outline" asChild>
                            <Link href="/contacts/import">
                                Import another file
                            </Link>
                        </Button>
                        <Button asChild>
                            <Link href="/contacts">View contacts</Link>
                        </Button>
                    </>
                )}
            </CardFooter>
        </Card>
    );
}
