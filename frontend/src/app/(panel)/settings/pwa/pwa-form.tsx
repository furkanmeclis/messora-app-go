'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { CheckCircle2, Download, Smartphone } from 'lucide-react';
import { usePwa } from '@/components/pwa';
import { notify } from '@/components/notify';
import { Button } from '@/components/ui/button';
import { ColorPicker } from '@/components/ui/color-picker';
import { Input } from '@/components/ui/input';
import { RadioCard, RadioCards } from '@/components/ui/radio-cards';
import { Segmented } from '@/components/ui/segmented';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { useT } from '@/i18n/client';
import { NAV } from '@/lib/navigation';
import type { Settings } from '@/lib/types';
import { saveSettingsAction } from '../actions';
import { Row, Section } from '../section';

type Pwa = Settings['pwa'];

export function PwaForm({
    settings,
    readOnly,
}: {
    settings: Settings;
    readOnly: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const app = usePwa();
    const [saved, setSaved] = useState<Pwa>(settings.pwa);
    const [v, setV] = useState<Pwa>(settings.pwa);
    const [saving, start] = useTransition();
    const set = <K extends keyof Pwa>(k: K, val: Pwa[K]) =>
        setV((p) => ({ ...p, [k]: val }));
    const dirty = JSON.stringify(v) !== JSON.stringify(saved);
    const name = v.name || settings.organization_name;
    const logo = settings.logo;

    const save = () =>
        start(async () => {
            const r = await saveSettingsAction({ pwa: v }, 'pwa.saved');
            if (notify(r)) {
                setSaved(r.data.pwa);
                setV(r.data.pwa);
                router.refresh();
            }
        });

    return (
        <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_22rem]">
            <Section
                title={t('pwa.title')}
                description={t('pwa.description')}
                readOnly={readOnly}
                dirty={dirty}
                saving={saving}
                onSave={save}
                onReset={() => setV(saved)}
            >
                <fieldset disabled={readOnly} className="space-y-6">
                    <div className="flex items-start justify-between gap-4 rounded-xl border bg-muted/20 p-4">
                        <div>
                            <p className="text-sm font-medium">
                                {t('pwa.enabled')}
                            </p>
                            <p className="text-xs text-muted-foreground">
                                {t('pwa.enabledHint')}
                            </p>
                        </div>
                        <Switch
                            checked={v.enabled}
                            onCheckedChange={(on) => set('enabled', on)}
                            aria-label={t('pwa.enabled')}
                        />
                    </div>
                    <div className="grid gap-5 sm:grid-cols-2">
                        <Row label={t('pwa.name')} htmlFor="pwa-name">
                            <Input
                                id="pwa-name"
                                value={v.name}
                                maxLength={60}
                                placeholder={t('pwa.namePlaceholder')}
                                onChange={(e) => set('name', e.target.value)}
                            />
                        </Row>
                        <Row
                            label={t('pwa.shortName')}
                            hint={t('pwa.shortNameHint')}
                            htmlFor="pwa-short"
                        >
                            <Input
                                id="pwa-short"
                                value={v.short_name}
                                maxLength={20}
                                onChange={(e) =>
                                    set('short_name', e.target.value)
                                }
                            />
                        </Row>
                    </div>
                    <Row label={t('pwa.appDescription')} htmlFor="pwa-desc">
                        <Input
                            id="pwa-desc"
                            value={v.description}
                            maxLength={200}
                            onChange={(e) => set('description', e.target.value)}
                        />
                    </Row>
                    <div className="grid gap-5 sm:grid-cols-2">
                        <Row label={t('pwa.themeColor')}>
                            <div className="flex items-center gap-2">
                                <ColorPicker
                                    value={v.theme_color}
                                    onChange={(c) => set('theme_color', c)}
                                    aria-label={t('pwa.themeColor')}
                                />
                                <span className="font-mono text-xs uppercase">
                                    {v.theme_color}
                                </span>
                            </div>
                        </Row>
                        <Row label={t('pwa.backgroundColor')}>
                            <div className="flex items-center gap-2">
                                <ColorPicker
                                    value={v.background_color}
                                    onChange={(c) => set('background_color', c)}
                                    aria-label={t('pwa.backgroundColor')}
                                />
                                <span className="font-mono text-xs uppercase">
                                    {v.background_color}
                                </span>
                            </div>
                        </Row>
                    </div>
                    <Row label={t('pwa.display')}>
                        <Segmented
                            value={v.display}
                            onValueChange={(d) => set('display', d)}
                            options={(
                                [
                                    'standalone',
                                    'fullscreen',
                                    'minimal-ui',
                                ] as const
                            ).map((d) => ({
                                value: d,
                                label: t(`pwa.displays.${d}`),
                            }))}
                        />
                    </Row>
                    <Row label={t('pwa.startUrl')}>
                        <Select
                            value={v.start_url}
                            onValueChange={(u) => set('start_url', u)}
                        >
                            <SelectTrigger className="w-full sm:w-72">
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                {NAV.filter((n) => n.group !== 'account').map(
                                    (n) => (
                                        <SelectItem key={n.id} value={n.href}>
                                            <n.icon />
                                            {t(n.label)}
                                        </SelectItem>
                                    ),
                                )}
                            </SelectContent>
                        </Select>
                    </Row>
                    <Row
                        label={t('pwa.icon')}
                        hint={!logo ? t('pwa.noLogo') : undefined}
                    >
                        <RadioCards
                            value={v.icon}
                            onValueChange={(i) => set('icon', i as Pwa['icon'])}
                            className="sm:grid-cols-2"
                        >
                            <RadioCard value="brand">
                                {t('pwa.icons.brand')}
                            </RadioCard>
                            <RadioCard value="logo" disabled={!logo}>
                                {t('pwa.icons.logo')}
                            </RadioCard>
                        </RadioCards>
                    </Row>
                </fieldset>
            </Section>

            <aside className="space-y-3 lg:sticky lg:top-20">
                <p className="text-sm font-medium">{t('pwa.preview')}</p>
                <div className="overflow-hidden rounded-2xl border shadow-lg">
                    <div
                        className="flex h-9 items-center gap-2 px-3 text-xs font-medium"
                        style={{ background: v.theme_color, color: '#fff' }}
                    >
                        <span className="size-2.5 rounded-full bg-white/70" />
                        {name}
                    </div>
                    <div
                        className="flex h-40 flex-col items-center justify-center gap-2"
                        style={{ background: v.background_color }}
                    >
                        {/* eslint-disable-next-line @next/next/no-img-element */}
                        <img
                            src={
                                v.icon === 'logo' && logo
                                    ? `/bff/logo?v=${logo.version}`
                                    : '/bff/favicon?brand=1'
                            }
                            alt=""
                            className="size-14 rounded-2xl object-contain shadow"
                        />
                        <span className="text-xs font-medium text-neutral-600">
                            {v.short_name || name.slice(0, 12)}
                        </span>
                    </div>
                </div>
                <div className="rounded-2xl border bg-card p-4 text-sm">
                    {app.installed ? (
                        <p className="flex items-center gap-2 text-success">
                            <CheckCircle2 className="size-4" />{' '}
                            {t('pwa.installed')}
                        </p>
                    ) : (
                        <>
                            <Button
                                className="w-full"
                                disabled={!app.canInstall}
                                onClick={() => void app.install()}
                            >
                                <Download /> {t('pwa.install')}
                            </Button>
                            <p className="mt-2 flex items-start gap-2 text-xs text-muted-foreground">
                                <Smartphone className="mt-0.5 size-3.5 shrink-0" />
                                {t('pwa.installHint')}
                            </p>
                        </>
                    )}
                </div>
            </aside>
        </div>
    );
}
