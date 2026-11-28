'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { toast } from 'sonner';
import { toastApiError } from '@/components/plan-hint';
import { Trash2 } from 'lucide-react';
import { BrandMark } from '@/components/brand';
import { Button } from '@/components/ui/button';
import { FileDrop } from '@/components/ui/file-drop';
import { notify } from '@/components/notify';
import { useT } from '@/i18n/client';
import type { Settings } from '@/lib/types';
import { removeLogoAction } from './actions';

/** Uploads the organisation logo through the streaming BFF route. */
export function LogoUploader({
    logo,
    readOnly,
    onChange,
}: {
    logo: Settings['logo'];
    readOnly: boolean;
    onChange?: (logo: Settings['logo']) => void;
}) {
    const t = useT();
    const router = useRouter();
    const [current, setCurrent] = useState(logo);
    const [busy, start] = useTransition();
    const [key, setKey] = useState(0);

    const upload = (file: File | null) => {
        if (!file) return;
        start(async () => {
            const res = await fetch('/bff/logo', {
                method: 'PUT',
                headers: {
                    'Content-Type': file.type || 'application/octet-stream',
                },
                body: file,
            });
            const body = (await res.json().catch(() => null)) as {
                data?: Settings;
                error?: Parameters<typeof toastApiError>[1];
            } | null;
            setKey((k) => k + 1);
            if (!res.ok || !body?.data) {
                toastApiError(t, body?.error, t('common.somethingWentWrong'));
                return;
            }
            setCurrent(body.data.logo);
            onChange?.(body.data.logo);
            toast.success(t('settings.logo.uploaded'));
            router.refresh();
        });
    };

    return (
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center">
            <div className="flex size-24 shrink-0 items-center justify-center rounded-2xl border bg-muted/40 p-3">
                {current ? (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img
                        src={`/bff/logo?v=${encodeURIComponent(current.version)}`}
                        alt=""
                        className="max-h-full max-w-full object-contain"
                    />
                ) : (
                    <BrandMark className="size-12" />
                )}
            </div>
            {!readOnly && (
                <div className="min-w-0 flex-1 space-y-2">
                    <FileDrop
                        key={key}
                        compact
                        accept="image/png,image/jpeg,image/webp,image/svg+xml"
                        onFile={upload}
                        disabled={busy}
                        hint={t('settings.logo.hint')}
                    />
                    {current && (
                        <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            disabled={busy}
                            onClick={() =>
                                start(async () => {
                                    const r = await removeLogoAction();
                                    if (notify(r)) {
                                        setCurrent(null);
                                        onChange?.(null);
                                        router.refresh();
                                    }
                                })
                            }
                        >
                            <Trash2 />
                            {t('settings.logo.remove')}
                        </Button>
                    )}
                </div>
            )}
        </div>
    );
}
