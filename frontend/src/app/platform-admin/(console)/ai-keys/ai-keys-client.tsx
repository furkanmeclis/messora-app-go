'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { KeyRound, Pencil, Plus, ShieldCheck, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { useConfirm } from '@/components/confirm';
import { useFormatDateTime } from '@/components/display-preferences';
import { notify } from '@/components/notify';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import {
    Dialog,
    DialogContent,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { useLocale, useT } from '@/i18n/client';
import { formatCredits } from '@/lib/plan';
import type { Schemas } from '@/lib/types';
import {
    createPlatformAIProviderAction,
    deletePlatformAIProviderAction,
    testPlatformAIProviderAction,
    updatePlatformAIProviderAction,
    updatePlatformAISettingsAction,
} from '../../plan-actions';

type Provider = Schemas['PlatformAIProvider'];
type Kind = Schemas['AIProviderKind'];
const KINDS: Kind[] = ['openai', 'anthropic', 'gemini', 'deepseek'];

const TONE: Record<Provider['status'], string> = {
    ok: 'border-success/40 bg-success/10 text-success',
    invalid: 'border-destructive/40 bg-destructive/10 text-destructive',
    no_balance: 'border-warning/40 bg-warning/10 text-warning',
    error: 'border-destructive/40 bg-destructive/10 text-destructive',
    unknown: '',
};

export function AIKeysClient({ providers }: { providers: Provider[] }) {
    const t = useT();
    const router = useRouter();
    const ask = useConfirm();
    const fmt = useFormatDateTime();
    const [pending, start] = useTransition();
    const [editing, setEditing] = useState<Provider | 'new' | null>(null);
    const used = providers.map((p) => p.kind);
    const test = (p: Provider) =>
        start(async () => {
            const r = await testPlatformAIProviderAction(p.id);
            if (!r.ok) {
                notify(r);
                return;
            }
            const status = t(`ai.providers.status.${r.data.status}`);
            const msg = t('plans.admin.aiKeys.tested', { status });
            if (r.data.status === 'ok') toast.success(msg);
            else toast.error(msg);
            router.refresh();
        });
    const toggle = (p: Provider, enabled: boolean) =>
        start(async () => {
            if (notify(await updatePlatformAIProviderAction(p.id, { enabled })))
                router.refresh();
        });
    const remove = async (p: Provider) => {
        const ok = await ask({
            title: t('plans.admin.aiKeys.remove'),
            description: t('plans.admin.aiKeys.removeConfirm', {
                name: p.name,
            }),
            confirmLabel: t('plans.admin.aiKeys.remove'),
            destructive: true,
        });
        if (!ok) return;
        start(async () => {
            if (notify(await deletePlatformAIProviderAction(p.id)))
                router.refresh();
        });
    };
    return (
        <Card>
            <CardHeader className="flex flex-row items-start justify-between gap-4">
                <div className="grid gap-1.5">
                    <CardTitle>{t('plans.admin.aiKeys.title')}</CardTitle>
                    <CardDescription>
                        {t('plans.admin.aiKeys.description')}
                    </CardDescription>
                </div>
                <Button
                    onClick={() => setEditing('new')}
                    disabled={used.length >= KINDS.length}
                    data-testid="ai-key-add"
                >
                    <Plus /> {t('plans.admin.aiKeys.add')}
                </Button>
            </CardHeader>
            <CardContent>
                {providers.length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                        {t('plans.admin.aiKeys.empty')}
                    </p>
                ) : (
                    <ul className="divide-y" data-testid="ai-keys">
                        {providers.map((p) => (
                            <li
                                key={p.id}
                                className="flex flex-wrap items-center gap-4 py-3"
                                data-kind={p.kind}
                            >
                                <div className="min-w-0 flex-1">
                                    <div className="flex flex-wrap items-center gap-2 font-medium">
                                        {t(`ai.kinds.${p.kind}`)} · {p.name}
                                        <Badge
                                            variant="outline"
                                            className={TONE[p.status]}
                                        >
                                            {t(
                                                `ai.providers.status.${p.status}`,
                                            )}
                                        </Badge>
                                    </div>
                                    <div className="text-xs text-muted-foreground">
                                        <span className="font-mono">
                                            •••• {p.key_last4}
                                        </span>{' '}
                                        ·{' '}
                                        {t('plans.admin.aiKeys.models', {
                                            count: p.model_count,
                                        })}
                                        {p.checked_at && (
                                            <>
                                                {' '}
                                                ·{' '}
                                                {t(
                                                    'plans.admin.aiKeys.lastCheck',
                                                    {
                                                        date: fmt(p.checked_at),
                                                    },
                                                )}
                                            </>
                                        )}
                                    </div>
                                    {p.status_detail && (
                                        <p className="mt-1 line-clamp-2 text-xs text-muted-foreground">
                                            {p.status_detail}
                                        </p>
                                    )}
                                </div>
                                <Switch
                                    checked={p.enabled}
                                    disabled={pending}
                                    aria-label={t('plans.admin.aiKeys.enabled')}
                                    onCheckedChange={(v) => toggle(p, v)}
                                />
                                <div className="flex gap-1">
                                    <Button
                                        size="sm"
                                        variant="outline"
                                        disabled={pending}
                                        onClick={() => test(p)}
                                    >
                                        <ShieldCheck />
                                        {t('plans.admin.aiKeys.test')}
                                    </Button>
                                    <Button
                                        size="sm"
                                        variant="outline"
                                        onClick={() => setEditing(p)}
                                        aria-label={t('plans.admin.plans.edit')}
                                    >
                                        <Pencil />
                                    </Button>
                                    <Button
                                        size="sm"
                                        variant="outline"
                                        onClick={() => void remove(p)}
                                        aria-label={t(
                                            'plans.admin.aiKeys.remove',
                                        )}
                                    >
                                        <Trash2 />
                                    </Button>
                                </div>
                            </li>
                        ))}
                    </ul>
                )}
            </CardContent>
            {editing && (
                <KeyDialog
                    provider={editing === 'new' ? null : editing}
                    kinds={KINDS.filter(
                        (k) =>
                            !used.includes(k) ||
                            (editing !== 'new' && editing.kind === k),
                    )}
                    onClose={() => setEditing(null)}
                />
            )}
        </Card>
    );
}

function KeyDialog({
    provider,
    kinds,
    onClose,
}: {
    provider: Provider | null;
    kinds: Kind[];
    onClose: () => void;
}) {
    const t = useT();
    const router = useRouter();
    const [kind, setKind] = useState<Kind>(provider?.kind ?? kinds[0]!);
    const [name, setName] = useState(provider?.name ?? '');
    const [key, setKey] = useState('');
    const [baseUrl, setBaseUrl] = useState(provider?.base_url ?? '');
    const [pending, start] = useTransition();
    const submit = () =>
        start(async () => {
            const r = provider
                ? await updatePlatformAIProviderAction(provider.id, {
                      name: name.trim(),
                      ...(key ? { api_key: key } : {}),
                      ...(kind === 'deepseek' || kind === 'openai'
                          ? { base_url: baseUrl.trim() }
                          : {}),
                  })
                : await createPlatformAIProviderAction({
                      kind,
                      name: name.trim() || `Messora ${t(`ai.kinds.${kind}`)}`,
                      api_key: key,
                      ...(baseUrl.trim() ? { base_url: baseUrl.trim() } : {}),
                  });
            if (notify(r)) {
                onClose();
                router.refresh();
            }
        });
    return (
        <Dialog open onOpenChange={(o) => !o && onClose()}>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle className="flex items-center gap-2">
                        <KeyRound className="size-4" />
                        {provider ? provider.name : t('plans.admin.aiKeys.add')}
                    </DialogTitle>
                </DialogHeader>
                <form
                    id="ai-key-form"
                    className="grid gap-4"
                    onSubmit={(e) => {
                        e.preventDefault();
                        submit();
                    }}
                >
                    {!provider && (
                        <div className="grid gap-2">
                            <Label htmlFor="ai-key-kind">
                                {t('plans.admin.aiKeys.kind')}
                            </Label>
                            <Select
                                value={kind}
                                onValueChange={(v) => setKind(v as Kind)}
                            >
                                <SelectTrigger id="ai-key-kind">
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
                        </div>
                    )}
                    <div className="grid gap-2">
                        <Label htmlFor="ai-key-name">
                            {t('plans.admin.aiKeys.name')}
                        </Label>
                        <Input
                            id="ai-key-name"
                            value={name}
                            maxLength={120}
                            onChange={(e) => setName(e.target.value)}
                        />
                    </div>
                    <div className="grid gap-2">
                        <Label htmlFor="ai-key-secret">
                            {t('plans.admin.aiKeys.apiKey')}
                        </Label>
                        <Input
                            id="ai-key-secret"
                            type="password"
                            autoComplete="off"
                            value={key}
                            required={!provider}
                            placeholder={
                                provider
                                    ? t('plans.admin.aiKeys.apiKeyKeep')
                                    : ''
                            }
                            onChange={(e) => setKey(e.target.value)}
                        />
                    </div>
                    {(kind === 'deepseek' || kind === 'openai') && (
                        <div className="grid gap-2">
                            <Label htmlFor="ai-key-base">
                                {t('plans.admin.aiKeys.baseUrl')}
                            </Label>
                            <Input
                                id="ai-key-base"
                                value={baseUrl}
                                placeholder="https://"
                                onChange={(e) => setBaseUrl(e.target.value)}
                            />
                        </div>
                    )}
                </form>
                <DialogFooter>
                    <Button type="button" variant="outline" onClick={onClose}>
                        {t('common.cancel')}
                    </Button>
                    <Button
                        type="submit"
                        form="ai-key-form"
                        disabled={pending || (!provider && !key)}
                    >
                        {t('common.save')}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}

const DECIMAL = /^[0-9]{1,9}(\.[0-9]{1,6})?$/;
const NONE = '__none__';

/** Credits per call and the default model of Messora's keys. */
export function AISettingsCard({
    settings: s,
    providers,
}: {
    settings: Schemas['PlatformAISettings'];
    providers: Provider[];
}) {
    const t = useT();
    const locale = useLocale();
    const router = useRouter();
    const [usd, setUsd] = useState(s.usd_per_credit);
    const [markup, setMarkup] = useState(s.markup);
    const [unpriced, setUnpriced] = useState(s.unpriced_call_credits);
    const [provider, setProvider] = useState(s.default_provider_id ?? NONE);
    const [model, setModel] = useState(s.default_model);
    const [pending, start] = useTransition();
    const valid = [usd, markup, unpriced].every((v) => DECIMAL.test(v));
    const models = s.models.filter((m) => m.provider_id === provider);
    const example =
        valid && Number(usd) > 0
            ? Math.ceil(((0.002 * Number(markup)) / Number(usd)) * 1000) / 1000
            : null;
    const save = () =>
        start(async () => {
            const r = await updatePlatformAISettingsAction({
                usd_per_credit: usd,
                markup,
                unpriced_call_credits: unpriced,
                default_provider_id: provider === NONE ? null : provider,
                default_model: provider === NONE ? '' : model.trim(),
            });
            if (notify(r)) router.refresh();
        });
    return (
        <Card>
            <CardHeader>
                <CardTitle>{t('plans.admin.aiKeys.creditsTitle')}</CardTitle>
                <CardDescription>
                    {t('plans.admin.aiKeys.creditsHint')}
                </CardDescription>
            </CardHeader>
            <CardContent>
                <form
                    className="grid gap-5"
                    onSubmit={(e) => {
                        e.preventDefault();
                        save();
                    }}
                >
                    <div className="grid gap-4 sm:grid-cols-3">
                        <div className="grid gap-2">
                            <Label htmlFor="ai-usd">
                                {t('plans.admin.aiKeys.usdPerCredit')}
                            </Label>
                            <Input
                                id="ai-usd"
                                inputMode="decimal"
                                value={usd}
                                aria-invalid={!DECIMAL.test(usd)}
                                onChange={(e) => setUsd(e.target.value)}
                            />
                        </div>
                        <div className="grid gap-2">
                            <Label htmlFor="ai-markup">
                                {t('plans.admin.aiKeys.markup')}
                            </Label>
                            <Input
                                id="ai-markup"
                                inputMode="decimal"
                                value={markup}
                                aria-invalid={!DECIMAL.test(markup)}
                                onChange={(e) => setMarkup(e.target.value)}
                            />
                        </div>
                        <div className="grid gap-2">
                            <Label htmlFor="ai-unpriced">
                                {t('plans.admin.aiKeys.unpriced')}
                            </Label>
                            <Input
                                id="ai-unpriced"
                                inputMode="decimal"
                                value={unpriced}
                                aria-invalid={!DECIMAL.test(unpriced)}
                                onChange={(e) => setUnpriced(e.target.value)}
                            />
                        </div>
                    </div>
                    {example !== null && (
                        <p className="text-xs text-muted-foreground">
                            {t('plans.admin.aiKeys.example', {
                                credits: formatCredits(example, locale),
                            })}
                        </p>
                    )}
                    <div className="grid gap-4 sm:grid-cols-2">
                        <div className="grid gap-2">
                            <Label htmlFor="ai-default-provider">
                                {t('plans.admin.aiKeys.defaultProvider')}
                            </Label>
                            <Select
                                value={provider}
                                onValueChange={(v) => {
                                    setProvider(v);
                                    setModel('');
                                }}
                            >
                                <SelectTrigger id="ai-default-provider">
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value={NONE}>
                                        {t('plans.admin.aiKeys.none')}
                                    </SelectItem>
                                    {providers.map((p) => (
                                        <SelectItem key={p.id} value={p.id}>
                                            {t(`ai.kinds.${p.kind}`)} · {p.name}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </div>
                        <div className="grid gap-2">
                            <Label htmlFor="ai-default-model">
                                {t('plans.admin.aiKeys.defaultModel')}
                            </Label>
                            {models.length > 0 ? (
                                <Select
                                    value={model || undefined}
                                    onValueChange={setModel}
                                >
                                    <SelectTrigger id="ai-default-model">
                                        <SelectValue
                                            placeholder={t(
                                                'ai.settings.chooseModel',
                                            )}
                                        />
                                    </SelectTrigger>
                                    <SelectContent>
                                        {models.map((m) => (
                                            <SelectItem
                                                key={m.model_id}
                                                value={m.model_id}
                                            >
                                                {m.model_id}
                                            </SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                            ) : (
                                <Input
                                    id="ai-default-model"
                                    value={model}
                                    disabled={provider === NONE}
                                    className="font-mono text-sm"
                                    onChange={(e) => setModel(e.target.value)}
                                />
                            )}
                        </div>
                    </div>
                    <p className="text-xs text-muted-foreground">
                        {t('plans.admin.aiKeys.defaultHint')}
                    </p>
                    <div className="flex justify-end">
                        <Button type="submit" disabled={pending || !valid}>
                            {t('common.save')}
                        </Button>
                    </div>
                </form>
            </CardContent>
        </Card>
    );
}
