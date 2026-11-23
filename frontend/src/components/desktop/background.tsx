'use client';

import { useState } from 'react';
import { Check } from 'lucide-react';
import { toast } from 'sonner';
import { ColorPicker } from '@/components/ui/color-picker';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog';
import { FileDrop } from '@/components/ui/file-drop';
import { Segmented } from '@/components/ui/segmented';
import { useT } from '@/i18n/client';
import type { Theme } from '@/lib/types';
import { cn } from '@/lib/utils';
import { WALLPAPERS, WALLPAPER_SIZE } from '@/theme/wallpapers';

type Fit = 'cover' | 'contain' | 'center';

/** The desktop background a person chose (stored in this browser). */
export type Wallpaper =
    | { kind: 'workspace' }
    | { kind: 'preset'; value: Theme['wallpaper'] }
    | { kind: 'color'; value: string }
    | { kind: 'image'; value: string; fit: Fit };

const KEY = 'messora:wallpaper';

export function loadWallpaper(): Wallpaper {
    try {
        const v = JSON.parse(
            localStorage.getItem(KEY) ?? 'null',
        ) as Wallpaper | null;
        if (!v) return { kind: 'workspace' };
        if (v.kind === 'preset' && v.value in WALLPAPERS) return v;
        if (v.kind === 'color' && /^#[0-9a-f]{6}$/i.test(v.value)) return v;
        if (v.kind === 'image' && v.value.startsWith('data:image/')) return v;
    } catch {
        // storage unavailable
    }
    return { kind: 'workspace' };
}

function save(v: Wallpaper) {
    try {
        localStorage.setItem(KEY, JSON.stringify(v));
        return true;
    } catch {
        return false; // e.g. the picture exceeds the storage quota
    }
}

/** CSS for the desktop root. */
export function wallpaperStyle(
    w: Wallpaper,
    workspace: Theme['wallpaper'],
): React.CSSProperties {
    switch (w.kind) {
        case 'preset':
            return {
                background: WALLPAPERS[w.value],
                backgroundSize: WALLPAPER_SIZE[w.value],
            };
        case 'color':
            return { background: w.value };
        case 'image':
            return {
                backgroundColor: 'var(--background)',
                backgroundImage: `url("${w.value}")`,
                backgroundSize: w.fit === 'center' ? 'auto' : w.fit,
                backgroundPosition: 'center',
                backgroundRepeat: 'no-repeat',
            };
        default:
            return {
                background: WALLPAPERS[workspace],
                backgroundSize: WALLPAPER_SIZE[workspace],
            };
    }
}

/** Scales a picture down (longest side 2560 px) to a JPEG data URL. */
async function shrink(file: File): Promise<string> {
    const url = URL.createObjectURL(file);
    try {
        const img = await new Promise<HTMLImageElement>((resolve, reject) => {
            const i = new Image();
            i.onload = () => resolve(i);
            i.onerror = reject;
            i.src = url;
        });
        const scale = Math.min(1, 2560 / Math.max(img.width, img.height));
        const canvas = document.createElement('canvas');
        canvas.width = Math.round(img.width * scale);
        canvas.height = Math.round(img.height * scale);
        canvas
            .getContext('2d')!
            .drawImage(img, 0, 0, canvas.width, canvas.height);
        return canvas.toDataURL('image/jpeg', 0.82);
    } finally {
        URL.revokeObjectURL(url);
    }
}

export function BackgroundDialog({
    open,
    onOpenChange,
    value,
    workspace,
    onChange,
}: {
    open: boolean;
    onOpenChange: (o: boolean) => void;
    value: Wallpaper;
    workspace: Theme['wallpaper'];
    onChange: (w: Wallpaper) => void;
}) {
    const t = useT();
    const [busy, setBusy] = useState(false);
    const set = (w: Wallpaper) => {
        if (!save(w)) {
            toast.error(t('common.somethingWentWrong'));
            return;
        }
        onChange(w);
    };
    const tile = (
        key: string,
        label: string,
        style: React.CSSProperties,
        on: boolean,
        pick: () => void,
    ) => (
        <button
            key={key}
            type="button"
            onClick={pick}
            className="group space-y-1 text-left outline-none"
        >
            <span
                className={cn(
                    'relative block aspect-video rounded-lg border transition-all group-hover:-translate-y-0.5 group-focus-visible:ring-3 group-focus-visible:ring-ring/50',
                    on && 'border-primary shadow-[0_0_0_2px_var(--primary)]',
                )}
                style={style}
            >
                {on && (
                    <span className="absolute top-1 right-1 flex size-5 items-center justify-center rounded-full bg-primary text-primary-foreground">
                        <Check className="size-3" />
                    </span>
                )}
            </span>
            <span className="block truncate text-xs">{label}</span>
        </button>
    );

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-2xl">
                <DialogHeader>
                    <DialogTitle>{t('desktop.background')}</DialogTitle>
                    <DialogDescription>
                        {t('desktop.backgroundHint')}
                    </DialogDescription>
                </DialogHeader>
                <div className="space-y-5">
                    <div className="grid grid-cols-3 gap-3 sm:grid-cols-4">
                        {tile(
                            'workspace',
                            t('desktop.workspaceDefault'),
                            wallpaperStyle({ kind: 'workspace' }, workspace),
                            value.kind === 'workspace',
                            () => set({ kind: 'workspace' }),
                        )}
                        {(Object.keys(WALLPAPERS) as Theme['wallpaper'][]).map(
                            (w) =>
                                tile(
                                    w,
                                    t(`appearance.wallpapers.${w}`),
                                    wallpaperStyle(
                                        { kind: 'preset', value: w },
                                        workspace,
                                    ),
                                    value.kind === 'preset' &&
                                        value.value === w,
                                    () => set({ kind: 'preset', value: w }),
                                ),
                        )}
                    </div>
                    <div className="flex items-center gap-3 rounded-xl border p-3">
                        <ColorPicker
                            aria-label={t('desktop.solidColour')}
                            value={
                                value.kind === 'color' ? value.value : '#334155'
                            }
                            onChange={(hex) =>
                                set({ kind: 'color', value: hex })
                            }
                        />
                        <span className="text-sm">
                            {t('desktop.solidColour')}
                        </span>
                        {value.kind === 'color' && (
                            <Check className="ml-auto size-4 text-primary" />
                        )}
                    </div>
                    <div className="space-y-3 rounded-xl border p-3">
                        <p className="text-sm font-medium">
                            {t('desktop.picture')}
                        </p>
                        <FileDrop
                            compact
                            accept="image/jpeg,image/png,image/webp"
                            disabled={busy}
                            hint={t('desktop.pictureHint')}
                            onFile={async (f) => {
                                if (!f) return;
                                setBusy(true);
                                try {
                                    set({
                                        kind: 'image',
                                        value: await shrink(f),
                                        fit:
                                            value.kind === 'image'
                                                ? value.fit
                                                : 'cover',
                                    });
                                } catch {
                                    toast.error(t('common.somethingWentWrong'));
                                } finally {
                                    setBusy(false);
                                }
                            }}
                        />
                        {value.kind === 'image' && (
                            <div className="flex items-center gap-3">
                                <span className="text-sm text-muted-foreground">
                                    {t('desktop.fit')}
                                </span>
                                <Segmented
                                    value={value.fit}
                                    onValueChange={(fit) =>
                                        set({ ...value, fit })
                                    }
                                    options={(
                                        ['cover', 'contain', 'center'] as const
                                    ).map((f) => ({
                                        value: f,
                                        label: t(`desktop.fits.${f}`),
                                    }))}
                                />
                            </div>
                        )}
                    </div>
                </div>
            </DialogContent>
        </Dialog>
    );
}
