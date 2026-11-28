'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import {
    KeyRound,
    Plus,
    PlugZap,
    RefreshCw,
    ShieldCheck,
    Trash2,
} from 'lucide-react';
import { toast } from 'sonner';
import { ClientDataTable } from '@/components/client-data-table';
import { useConfirm } from '@/components/confirm';
import { useFormatDateTime } from '@/components/display-preferences';
import { notify } from '@/components/notify';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog';
import {
    Empty,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from '@/components/ui/empty';
import { Field, FieldDescription, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Spinner } from '@/components/ui/spinner';
import { Switch } from '@/components/ui/switch';
import { UpgradeHint } from '@/components/plan-hint';
import { useT } from '@/i18n/client';
import type { Schemas } from '@/lib/types';
import { PlatformProviders } from './platform-providers';
import {
    checkProviderAction,
    createProviderAction,
    deleteProviderAction,
    syncModelsAction,
    updateProviderAction,
} from '../actions';

type Provider = Schemas['AIProvider'];
type Kind = Schemas['AIProviderKind'];

const STATUS_TONE: Record<Provider['status'], string> = {
    ok: 'border-success/40 bg-success/10 text-success',
    invalid: 'border-destructive/40 bg-destructive/10 text-destructive',
    no_balance: 'border-warning/40 bg-warning/10 text-warning',
    error: 'border-destructive/40 bg-destructive/10 text-destructive',
    unknown: '',
};

/** Connected AI providers: add, check, refresh models, rotate, remove. */
export function ProvidersClient({
    providers,
    fakeAvailable,
    canManage: mayManage,
    byo = true,
    platform = [],
}: {
    providers: Provider[];
    fakeAvailable: boolean;
    canManage: boolean;
    /** The plan includes the company's own AI keys (Enterprise). */
    byo?: boolean;
    /** Messora's keys the workspace can use (paid with AI credits). */
    platform?: Schemas['AIPlatformProvider'][];
}) {
    // Own keys are managed only with byo_ai_keys; stored keys of a plan
    // without it stay listed so they can be deleted.
    const canManage = mayManage && byo;
    const t = useT();
    const router = useRouter();
    const ask = useConfirm();
    const formatDateTime = useFormatDateTime();
    const [pending, start] = useTransition();
    const [adding, setAdding] = useState(false);
    const [rotating, setRotating] = useState<Provider | null>(null);

    const run = (fn: () => Promise<void>) =>
        start(async () => {
            await fn();
            router.refresh();
        });

    const check = (p: Provider) =>
        run(async () => {
            const r = await checkProviderAction(p.id);
            if (r.ok) {
                const status = t(`ai.providers.status.${r.data.status}`);
                if (r.data.status === 'ok')
                    toast.success(t('ai.providers.checked', { status }));
                else toast.error(t('ai.providers.checked', { status }));
            } else notify(r);
        });
    const sync = (p: Provider) =>
        run(async () => {
            const r = await syncModelsAction(p.id);
            if (r.ok)
                toast.success(
                    t('ai.providers.synced', { count: r.data.models }),
                );
            else notify(r);
        });
    const toggle = (p: Provider, enabled: boolean) =>
        run(async () => {
            notify(
                await updateProviderAction(
                    p.id,
                    { enabled },
                    'ai.providers.updated',
                ),
            );
        });
    const remove = async (p: Provider) => {
        const ok = await ask({
            title: t('ai.providers.deleteTitle', { name: p.name }),
            description: t('ai.providers.deleteBody'),
            confirmLabel: t('ai.providers.delete'),
            destructive: true,
        });
        if (ok) run(async () => void notify(await deleteProviderAction(p.id)));
    };

    const ownSection = byo || providers.length > 0;
    return (
        <div className="space-y-6">
            <PlatformProviders providers={platform} />
            {byo ? (
                <p className="text-sm text-muted-foreground">
                    {t('plans.ai.enterpriseFallback')}
                </p>
            ) : (
                <UpgradeHint testId="byo-locked">
                    {t('plans.ai.ownLocked')}
                </UpgradeHint>
            )}
            {ownSection && (
                <div className="space-y-4" data-testid="own-ai-keys">
                    {!byo && (
                        <p
                            className="text-sm text-muted-foreground"
                            data-testid="byo-stored-unused"
                        >
                            {t('plans.ai.storedUnused')}
                        </p>
                    )}
                    <div className="flex flex-wrap items-start justify-between gap-4">
                        <div>
                            <h2 className="font-semibold tracking-tight">
                                {t('ai.providers.title')}
                            </h2>
                            <p className="text-sm text-muted-foreground">
                                {t('ai.providers.description')}
                            </p>
                        </div>
                        {canManage && (
                            <Button
                                onClick={() => setAdding(true)}
                                shortcut="n"
                            >
                                <Plus /> {t('ai.providers.add')}
                            </Button>
                        )}
                    </div>
                    <ClientDataTable
                        rows={providers}
                        rowId={(p) => p.id}
                        storageKey="ai-providers"
                        initialSort="name"
                        rowTestId="ai-provider-row"
                        empty={
                            <Empty className="py-12">
                                <EmptyHeader>
                                    <EmptyMedia variant="icon">
                                        <PlugZap />
                                    </EmptyMedia>
                                    <EmptyTitle>
                                        {t('ai.providers.empty')}
                                    </EmptyTitle>
                                    <EmptyDescription>
                                        {t('ai.providers.emptyHint')}
                                    </EmptyDescription>
                                </EmptyHeader>
                            </Empty>
                        }
                        columns={[
                            {
                                id: 'name',
                                header: t('ai.providers.columns.name'),
                                hideable: false,
                                sortValue: (p) => p.name,
                                cell: (p) => (
                                    <div className="min-w-0">
                                        <div className="font-medium">
                                            {p.name}
                                        </div>
                                        <div className="text-xs text-muted-foreground">
                                            {t(`ai.kinds.${p.kind}`)}
                                            {p.base_url && (
                                                <span className="ml-1 font-mono">
                                                    · {p.base_url}
                                                </span>
                                            )}
                                        </div>
                                    </div>
                                ),
                            },
                            {
                                id: 'status',
                                header: t('ai.providers.columns.status'),
                                sortValue: (p) => p.status,
                                cell: (p) => (
                                    <div className="max-w-sm space-y-1">
                                        <Badge
                                            variant="outline"
                                            className={STATUS_TONE[p.status]}
                                            data-testid="ai-provider-status"
                                        >
                                            {t(
                                                `ai.providers.status.${p.status}`,
                                            )}
                                        </Badge>
                                        {p.status_detail && (
                                            <p className="line-clamp-2 text-xs text-muted-foreground">
                                                {p.status_detail}
                                            </p>
                                        )}
                                        <p className="text-xs text-muted-foreground">
                                            {t('ai.providers.checkedAt', {
                                                time: p.checked_at
                                                    ? formatDateTime(
                                                          p.checked_at,
                                                      )
                                                    : t('ai.providers.never'),
                                            })}
                                        </p>
                                    </div>
                                ),
                            },
                            {
                                id: 'key',
                                header: t('ai.providers.columns.key'),
                                cell: (p) => (
                                    <span className="font-mono text-xs whitespace-nowrap text-muted-foreground">
                                        •••• {p.key_last4}
                                    </span>
                                ),
                            },
                            {
                                id: 'models',
                                header: t('ai.providers.columns.models'),
                                sortValue: (p) => p.models_synced_at,
                                cell: (p) => (
                                    <span className="text-sm whitespace-nowrap text-muted-foreground">
                                        {t('ai.providers.syncedAt', {
                                            time: p.models_synced_at
                                                ? formatDateTime(
                                                      p.models_synced_at,
                                                  )
                                                : t('ai.providers.never'),
                                        })}
                                    </span>
                                ),
                            },
                            {
                                id: 'enabled',
                                header: t('ai.providers.columns.enabled'),
                                sortValue: (p) => (p.enabled ? 1 : 0),
                                cell: (p) => (
                                    <Switch
                                        checked={p.enabled}
                                        disabled={!canManage || pending}
                                        aria-label={t(
                                            'ai.providers.columns.enabled',
                                        )}
                                        onCheckedChange={(c) => toggle(p, c)}
                                    />
                                ),
                            },
                        ]}
                        rowActions={
                            canManage
                                ? (p) => (
                                      <div className="flex gap-1">
                                          <Button
                                              size="sm"
                                              variant="outline"
                                              disabled={pending}
                                              onClick={() => check(p)}
                                          >
                                              {pending ? (
                                                  <Spinner />
                                              ) : (
                                                  <ShieldCheck />
                                              )}
                                              {t('ai.providers.check')}
                                          </Button>
                                          <Button
                                              size="sm"
                                              variant="outline"
                                              disabled={pending}
                                              onClick={() => sync(p)}
                                          >
                                              <RefreshCw />
                                              {t('ai.providers.sync')}
                                          </Button>
                                      </div>
                                  )
                                : undefined
                        }
                        rowMenu={
                            mayManage && !byo
                                ? (p) => [
                                      [
                                          {
                                              id: 'delete',
                                              label: t('ai.providers.delete'),
                                              icon: Trash2,
                                              danger: true,
                                              run: () => void remove(p),
                                          },
                                      ],
                                  ]
                                : canManage
                                  ? (p) => [
                                        [
                                            {
                                                id: 'check',
                                                label: t('ai.providers.check'),
                                                icon: ShieldCheck,
                                                run: () => check(p),
                                            },
                                            {
                                                id: 'sync',
                                                label: t('ai.providers.sync'),
                                                icon: RefreshCw,
                                                run: () => sync(p),
                                            },
                                            {
                                                id: 'rotate',
                                                label: t('ai.providers.rotate'),
                                                icon: KeyRound,
                                                run: () => setRotating(p),
                                            },
                                        ],
                                        [
                                            {
                                                id: 'delete',
                                                label: t('ai.providers.delete'),
                                                icon: Trash2,
                                                danger: true,
                                                run: () => void remove(p),
                                            },
                                        ],
                                    ]
                                  : undefined
                        }
                    />
                    <AddProviderDialog
                        open={adding}
                        onOpenChange={setAdding}
                        fakeAvailable={fakeAvailable}
                        onDone={() => router.refresh()}
                    />
                    <RotateKeyDialog
                        provider={rotating}
                        onClose={() => setRotating(null)}
                        onDone={() => router.refresh()}
                    />
                </div>
            )}
        </div>
    );
}

function AddProviderDialog({
    open,
    onOpenChange,
    fakeAvailable,
    onDone,
}: {
    open: boolean;
    onOpenChange: (o: boolean) => void;
    fakeAvailable: boolean;
    onDone: () => void;
}) {
    const t = useT();
    const [kind, setKind] = useState<Kind>('openai');
    const [name, setName] = useState('');
    const [key, setKey] = useState('');
    const [baseURL, setBaseURL] = useState('');
    const [errors, setErrors] = useState<Record<string, string>>({});
    const [pending, start] = useTransition();
    const kinds: Kind[] = ['openai', 'anthropic', 'gemini', 'deepseek'];
    if (fakeAvailable) kinds.push('fake');
    const allowsBase = kind === 'deepseek' || kind === 'openai';

    const reset = () => {
        setName('');
        setKey('');
        setBaseURL('');
        setErrors({});
    };
    const submit = (e: React.FormEvent) => {
        e.preventDefault();
        start(async () => {
            const r = await createProviderAction({
                kind,
                name: name.trim() || t(`ai.kinds.${kind}`),
                api_key: key.trim(),
                ...(allowsBase && baseURL.trim()
                    ? { base_url: baseURL.trim() }
                    : {}),
            });
            if (notify(r)) {
                reset();
                onOpenChange(false);
                onDone();
            } else if (!r.ok) setErrors(r.fieldErrors ?? {});
        });
    };

    return (
        <Dialog
            open={open}
            onOpenChange={(o) => {
                onOpenChange(o);
                if (!o) reset();
            }}
        >
            <DialogContent className="max-h-[90svh] overflow-y-auto">
                <DialogHeader>
                    <DialogTitle>{t('ai.providers.addTitle')}</DialogTitle>
                    <DialogDescription>
                        {t('ai.providers.addDescription')}
                    </DialogDescription>
                </DialogHeader>
                <form onSubmit={submit} className="space-y-4">
                    <Field>
                        <FieldLabel htmlFor="ai-provider-kind">
                            {t('ai.providers.kind')}
                        </FieldLabel>
                        <Select
                            value={kind}
                            onValueChange={(v) => setKind(v as Kind)}
                        >
                            <SelectTrigger
                                id="ai-provider-kind"
                                className="w-full"
                            >
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                {kinds.map((k) => (
                                    <SelectItem key={k} value={k}>
                                        {t(`ai.kinds.${k}`)}
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </Field>
                    <Field data-invalid={!!errors.name}>
                        <FieldLabel htmlFor="ai-provider-name">
                            {t('ai.providers.name')}
                        </FieldLabel>
                        <Input
                            id="ai-provider-name"
                            value={name}
                            maxLength={120}
                            placeholder={t('ai.providers.namePlaceholder')}
                            onChange={(e) => setName(e.target.value)}
                        />
                        {errors.name && (
                            <FieldDescription className="text-destructive">
                                {errors.name}
                            </FieldDescription>
                        )}
                    </Field>
                    <Field data-invalid={!!errors.api_key}>
                        <FieldLabel htmlFor="ai-provider-key">
                            {t('ai.providers.apiKey')}
                        </FieldLabel>
                        <Input
                            id="ai-provider-key"
                            type="password"
                            autoComplete="off"
                            required
                            minLength={8}
                            maxLength={500}
                            value={key}
                            onChange={(e) => setKey(e.target.value)}
                            className="font-mono"
                        />
                        <FieldDescription
                            className={errors.api_key ? 'text-destructive' : ''}
                        >
                            {errors.api_key ?? t('ai.providers.apiKeyHint')}
                        </FieldDescription>
                    </Field>
                    {allowsBase && (
                        <Field data-invalid={!!errors.base_url}>
                            <FieldLabel htmlFor="ai-provider-base">
                                {t('ai.providers.baseUrl')}
                            </FieldLabel>
                            <Input
                                id="ai-provider-base"
                                type="url"
                                placeholder="https://"
                                value={baseURL}
                                onChange={(e) => setBaseURL(e.target.value)}
                                className="font-mono"
                            />
                            <FieldDescription
                                className={
                                    errors.base_url ? 'text-destructive' : ''
                                }
                            >
                                {errors.base_url ??
                                    t('ai.providers.baseUrlHint')}
                            </FieldDescription>
                        </Field>
                    )}
                    <DialogFooter>
                        <Button type="submit" disabled={pending || !key}>
                            {pending && <Spinner />}
                            {t('ai.providers.create')}
                        </Button>
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    );
}

function RotateKeyDialog({
    provider,
    onClose,
    onDone,
}: {
    provider: Provider | null;
    onClose: () => void;
    onDone: () => void;
}) {
    const t = useT();
    const [key, setKey] = useState('');
    const [pending, start] = useTransition();
    const submit = (e: React.FormEvent) => {
        e.preventDefault();
        if (!provider) return;
        start(async () => {
            const r = await updateProviderAction(
                provider.id,
                { api_key: key.trim() },
                'ai.providers.rotated',
            );
            if (notify(r)) {
                setKey('');
                onClose();
                onDone();
            }
        });
    };
    return (
        <Dialog
            open={!!provider}
            onOpenChange={(o) => {
                if (!o) {
                    setKey('');
                    onClose();
                }
            }}
        >
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>{t('ai.providers.rotateTitle')}</DialogTitle>
                    <DialogDescription>
                        {t('ai.providers.rotateDescription')}
                    </DialogDescription>
                </DialogHeader>
                <form onSubmit={submit} className="space-y-4">
                    <Field>
                        <FieldLabel htmlFor="ai-provider-new-key">
                            {t('ai.providers.newKey')}
                        </FieldLabel>
                        <Input
                            id="ai-provider-new-key"
                            type="password"
                            autoComplete="off"
                            required
                            minLength={8}
                            value={key}
                            onChange={(e) => setKey(e.target.value)}
                            className="font-mono"
                        />
                    </Field>
                    <DialogFooter>
                        <Button type="submit" disabled={pending || !key}>
                            {pending && <Spinner />}
                            {t('ai.providers.rotate')}
                        </Button>
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    );
}
