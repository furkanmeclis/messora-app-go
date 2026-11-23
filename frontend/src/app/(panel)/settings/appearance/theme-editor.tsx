'use client';

import { useMemo, useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { AlertTriangle, Moon, RotateCcw, Sun } from 'lucide-react';
import { Monitor } from 'lucide-react';
import { ColorPicker } from '@/components/ui/color-picker';
import { RadioCard, RadioCards } from '@/components/ui/radio-cards';
import { Segmented } from '@/components/ui/segmented';
import { Slider } from '@/components/ui/slider';
import { notify } from '@/components/notify';
import { useT } from '@/i18n/client';
import type { Theme } from '@/lib/types';
import { cn } from '@/lib/utils';
import { parseColour, toHex, contrast } from '@/theme/colour';
import { FONTS, resolvePalette } from '@/theme/css';
import { PALETTES, type Preset } from '@/theme/presets';
import { CONTRAST_PAIRS, TOKEN_GROUPS, type Token } from '@/theme/tokens';
import { WALLPAPERS, WALLPAPER_SIZE } from '@/theme/wallpapers';
import { saveSettingsAction } from '../actions';
import { Row, Section } from '../section';
import { PresetPicker } from './preset-picker';
import { ThemePreview } from './theme-preview';

const same = (a: Theme, b: Theme) => JSON.stringify(a) === JSON.stringify(b);

export function ThemeEditor({
    theme,
    readOnly,
}: {
    theme: Theme;
    readOnly: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const [saved, setSaved] = useState(theme);
    const [draft, setDraft] = useState(theme);
    const [scheme, setScheme] = useState<'light' | 'dark'>('light');
    const [saving, start] = useTransition();
    const set = (patch: Partial<Theme>) =>
        setDraft((d) => ({ ...d, ...patch }));
    const palette = useMemo(
        () => resolvePalette(draft, scheme),
        [draft, scheme],
    );
    const base = PALETTES[draft.preset as Preset]?.[scheme];
    const overrides = draft[scheme];
    const overridden = Object.keys(overrides).length;

    const setToken = (token: Token, hex: string | null) =>
        setDraft((d) => {
            const next = { ...d[scheme] };
            if (hex === null) delete next[token];
            else next[token] = hex;
            return { ...d, [scheme]: next };
        });

    const lowContrast = CONTRAST_PAIRS.map(([fg, bg]) => ({
        fg,
        bg,
        ratio: contrast(palette[fg], palette[bg]),
    })).filter((p) => p.ratio !== null && p.ratio < 4.5);

    const save = () =>
        start(async () => {
            const r = await saveSettingsAction(
                { theme: draft },
                'appearance.saved',
            );
            if (notify(r)) {
                setSaved(r.data.theme);
                setDraft(r.data.theme);
                router.refresh();
            }
        });

    return (
        <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_22rem]">
            <Section
                title={t('appearance.title')}
                description={
                    readOnly
                        ? t('appearance.ownerOnly')
                        : t('appearance.description')
                }
                readOnly={readOnly}
                dirty={!same(draft, saved)}
                saving={saving}
                onSave={save}
                onReset={() => setDraft(saved)}
            >
                <fieldset disabled={readOnly} className="space-y-7">
                    <Row label={t('appearance.presets')}>
                        <PresetPicker
                            value={draft.preset}
                            scheme={scheme}
                            onChange={(preset) => set({ preset })}
                            disabled={readOnly}
                        />
                    </Row>
                    <Row
                        label={t('appearance.mode')}
                        hint={t('appearance.modeHint')}
                    >
                        <Segmented
                            value={draft.mode}
                            onValueChange={(mode) => {
                                set({ mode });
                                if (mode !== 'system') setScheme(mode);
                            }}
                            disabled={readOnly}
                            options={[
                                {
                                    value: 'light',
                                    label: t('common.light'),
                                    icon: <Sun />,
                                },
                                {
                                    value: 'dark',
                                    label: t('common.dark'),
                                    icon: <Moon />,
                                },
                                {
                                    value: 'system',
                                    label: t('common.system'),
                                    icon: <Monitor />,
                                },
                            ]}
                        />
                    </Row>
                    <Row label={t('appearance.font')}>
                        <RadioCards
                            value={draft.font}
                            onValueChange={(f) =>
                                set({ font: f as Theme['font'] })
                            }
                            disabled={readOnly}
                            className="grid-cols-2 sm:grid-cols-3"
                        >
                            {(Object.keys(FONTS) as Theme['font'][]).map(
                                (f) => (
                                    <RadioCard key={f} value={f}>
                                        <span
                                            className="block text-lg leading-tight font-semibold"
                                            style={{
                                                fontFamily: FONTS[f].family,
                                            }}
                                        >
                                            Ağ Şğ 123
                                        </span>
                                        <span className="text-xs text-muted-foreground">
                                            {FONTS[f].label}
                                        </span>
                                    </RadioCard>
                                ),
                            )}
                        </RadioCards>
                    </Row>
                    <div className="grid gap-7 sm:grid-cols-2">
                        <Row label={t('appearance.radius')}>
                            <div className="flex items-center gap-3">
                                <span className="text-xs text-muted-foreground">
                                    {t('appearance.radiusSquare')}
                                </span>
                                <Slider
                                    value={[draft.radius]}
                                    min={0}
                                    max={1.5}
                                    step={0.125}
                                    onValueChange={([r]) =>
                                        set({ radius: r ?? 0 })
                                    }
                                    disabled={readOnly}
                                    aria-label={t('appearance.radius')}
                                />
                                <span className="text-xs text-muted-foreground">
                                    {t('appearance.radiusRound')}
                                </span>
                            </div>
                        </Row>
                        <Row label={t('appearance.density')}>
                            <Segmented
                                value={draft.density}
                                onValueChange={(density) => set({ density })}
                                disabled={readOnly}
                                options={(
                                    [
                                        'compact',
                                        'comfortable',
                                        'spacious',
                                    ] as const
                                ).map((d) => ({
                                    value: d,
                                    label: t(`appearance.densities.${d}`),
                                }))}
                            />
                        </Row>
                    </div>
                    <Row label={t('appearance.wallpaper')}>
                        <div
                            role="radiogroup"
                            className="grid grid-cols-3 gap-2 sm:grid-cols-6"
                        >
                            {(
                                Object.keys(WALLPAPERS) as Theme['wallpaper'][]
                            ).map((w) => (
                                <button
                                    key={w}
                                    type="button"
                                    role="radio"
                                    aria-checked={draft.wallpaper === w}
                                    disabled={readOnly}
                                    onClick={() => set({ wallpaper: w })}
                                    className={cn(
                                        'group space-y-1 rounded-xl text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50',
                                    )}
                                >
                                    <span
                                        className={cn(
                                            'block aspect-video rounded-lg border transition-all group-hover:-translate-y-0.5',
                                            draft.wallpaper === w &&
                                                'border-primary shadow-[0_0_0_2px_var(--primary)]',
                                        )}
                                        style={{
                                            background: WALLPAPERS[w],
                                            backgroundSize: WALLPAPER_SIZE[w],
                                        }}
                                    />
                                    <span className="block text-center text-xs">
                                        {t(`appearance.wallpapers.${w}`)}
                                    </span>
                                </button>
                            ))}
                        </div>
                    </Row>

                    <div className="space-y-4 rounded-xl border bg-muted/20 p-4">
                        <div className="flex flex-wrap items-center gap-3">
                            <div className="min-w-0 flex-1">
                                <h3 className="text-sm font-semibold">
                                    {t('appearance.colours')}
                                </h3>
                                <p className="text-xs text-muted-foreground">
                                    {t('appearance.coloursHint')}
                                </p>
                            </div>
                            <Segmented
                                aria-label={t('appearance.editingScheme')}
                                value={scheme}
                                onValueChange={setScheme}
                                options={[
                                    {
                                        value: 'light',
                                        label: t('common.light'),
                                        icon: <Sun />,
                                    },
                                    {
                                        value: 'dark',
                                        label: t('common.dark'),
                                        icon: <Moon />,
                                    },
                                ]}
                            />
                        </div>
                        {overridden > 0 && (
                            <div className="flex items-center justify-between rounded-lg bg-accent/50 px-3 py-2 text-xs">
                                {t('appearance.customised', {
                                    count: overridden,
                                })}
                                {!readOnly && (
                                    <button
                                        type="button"
                                        className="flex items-center gap-1 font-medium text-primary"
                                        onClick={() =>
                                            set({
                                                [scheme]: {},
                                            } as Partial<Theme>)
                                        }
                                    >
                                        <RotateCcw className="size-3" />
                                        {t('appearance.resetColours')}
                                    </button>
                                )}
                            </div>
                        )}
                        {lowContrast.length > 0 && (
                            <ul className="space-y-1">
                                {lowContrast.map((p) => (
                                    <li
                                        key={p.fg}
                                        className="flex items-start gap-2 rounded-lg border border-warning/40 bg-warning/10 px-3 py-2 text-xs"
                                    >
                                        <AlertTriangle className="mt-0.5 size-3.5 shrink-0 text-warning" />
                                        <span>
                                            <strong>
                                                {t(`appearance.tokens.${p.fg}`)}{' '}
                                                /{' '}
                                                {t(`appearance.tokens.${p.bg}`)}
                                            </strong>
                                            {' — '}
                                            {t('appearance.contrastLow', {
                                                ratio: (p.ratio ?? 0).toFixed(
                                                    1,
                                                ),
                                            })}
                                        </span>
                                    </li>
                                ))}
                            </ul>
                        )}
                        {(
                            Object.keys(
                                TOKEN_GROUPS,
                            ) as (keyof typeof TOKEN_GROUPS)[]
                        ).map((g) => (
                            <details
                                key={g}
                                className="group rounded-lg border bg-card"
                                open={g === 'brand'}
                            >
                                <summary className="flex cursor-pointer list-none items-center justify-between px-3 py-2 text-sm font-medium">
                                    {t(`appearance.tokenGroups.${g}`)}
                                    <span className="flex -space-x-1">
                                        {TOKEN_GROUPS[g]
                                            .slice(0, 5)
                                            .map((tok) => (
                                                <span
                                                    key={tok}
                                                    className="size-4 rounded-full border-2 border-card"
                                                    style={{
                                                        background:
                                                            palette[tok],
                                                    }}
                                                />
                                            ))}
                                    </span>
                                </summary>
                                <div className="grid gap-1 border-t p-2 sm:grid-cols-2">
                                    {TOKEN_GROUPS[g].map((tok) => {
                                        const custom = tok in overrides;
                                        const rgb = parseColour(palette[tok]);
                                        return (
                                            <div
                                                key={tok}
                                                className="flex items-center gap-2 rounded-md px-1.5 py-1 hover:bg-muted/50"
                                            >
                                                {readOnly ? (
                                                    <span
                                                        className="size-8 rounded-lg border"
                                                        style={{
                                                            background:
                                                                palette[tok],
                                                        }}
                                                    />
                                                ) : (
                                                    <ColorPicker
                                                        value={palette[tok]}
                                                        onChange={(hex) =>
                                                            setToken(tok, hex)
                                                        }
                                                        aria-label={t(
                                                            `appearance.tokens.${tok}`,
                                                        )}
                                                        swatches={
                                                            base
                                                                ? [
                                                                      base[tok],
                                                                      base.primary,
                                                                      base[
                                                                          'chart-2'
                                                                      ],
                                                                      base.foreground,
                                                                      base.background,
                                                                  ]
                                                                : []
                                                        }
                                                    />
                                                )}
                                                <span className="min-w-0 flex-1">
                                                    <span className="block truncate text-xs font-medium">
                                                        {t(
                                                            `appearance.tokens.${tok}`,
                                                        )}
                                                    </span>
                                                    <span className="block font-mono text-[10px] text-muted-foreground uppercase">
                                                        {rgb
                                                            ? toHex(rgb)
                                                            : palette[tok]}
                                                    </span>
                                                </span>
                                                {custom && !readOnly && (
                                                    <button
                                                        type="button"
                                                        aria-label={t(
                                                            'common.resetToDefault',
                                                        )}
                                                        onClick={() =>
                                                            setToken(tok, null)
                                                        }
                                                        className="rounded p-1 text-muted-foreground hover:bg-muted hover:text-foreground"
                                                    >
                                                        <RotateCcw className="size-3" />
                                                    </button>
                                                )}
                                            </div>
                                        );
                                    })}
                                </div>
                            </details>
                        ))}
                    </div>
                </fieldset>
            </Section>
            <aside className="space-y-2 lg:sticky lg:top-4">
                <div className="flex items-center justify-between">
                    <span className="text-sm font-medium">
                        {t('appearance.previewTitle')}
                    </span>
                    <Segmented
                        aria-label={t('appearance.previewTitle')}
                        value={scheme}
                        onValueChange={setScheme}
                        options={[
                            {
                                value: 'light',
                                label: <Sun className="size-4" />,
                            },
                            {
                                value: 'dark',
                                label: <Moon className="size-4" />,
                            },
                        ]}
                    />
                </div>
                <ThemePreview theme={draft} scheme={scheme} />
            </aside>
        </div>
    );
}
