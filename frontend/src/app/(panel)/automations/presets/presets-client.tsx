'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { AlertTriangle, Bot, Download, Wrench, Zap } from 'lucide-react';
import { notify } from '@/components/notify';
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
import { Spinner } from '@/components/ui/spinner';
import { useLocale, useT } from '@/i18n/client';
import type { Schemas } from '@/lib/types';
import { installPresetAction } from '../actions';
import { describeTrigger, mk, stepLabel } from '../model';
import { StepIcon } from '../nodes';

type Preset = Schemas['AutomationPreset'];
type Localized = Schemas['LocalizedText'];

/** Step types of a preset in program order (conditions' branches too). */
function stepTypes(steps: Preset['steps']): string[] {
    const out: string[] = [];
    const walk = (list: Preset['steps'] | undefined) => {
        for (const s of list ?? []) {
            const r = s as Record<string, unknown>;
            out.push(String(r.type ?? ''));
            walk(r.then as Preset['steps']);
            walk(r.else as Preset['steps']);
        }
    };
    walk(steps);
    return out;
}

/**
 * The preset library: one card per ready-made automation. Installing one
 * creates a normal, disabled automation and opens it on the canvas.
 */
export function PresetGallery({
    presets,
    canManage,
    aiEnabled,
}: {
    presets: Preset[];
    canManage: boolean;
    aiEnabled: boolean | null;
}) {
    const t = useT();
    const locale = useLocale();
    const router = useRouter();
    const [pending, startTransition] = useTransition();
    const [busy, setBusy] = useState<string | null>(null);
    const text = (l: Localized | null | undefined) =>
        l ? (locale === 'tr' ? l.tr : l.en) || l.en : '';

    const install = (key: string) => {
        setBusy(key);
        startTransition(async () => {
            const r = await installPresetAction(key, locale);
            setBusy(null);
            if (notify(r) && r.ok) router.push(`/automations/${r.data.id}`);
        });
    };

    return (
        <div
            className="grid gap-4 md:grid-cols-2 xl:grid-cols-3"
            data-testid="preset-gallery"
        >
            {presets.map((p) => {
                const types = stepTypes(p.steps);
                return (
                    <Card
                        key={p.key}
                        data-testid="preset-card"
                        data-preset-key={p.key}
                        className="flex flex-col"
                    >
                        <CardHeader>
                            <div className="flex flex-wrap items-center gap-2">
                                <Badge variant="outline">
                                    {t(
                                        mk(
                                            `automations.presets.categories.${p.category}`,
                                        ),
                                    )}
                                </Badge>
                                {p.requires_ai && (
                                    <Badge
                                        variant="outline"
                                        className="border-fuchsia-500/40 text-fuchsia-700 dark:text-fuchsia-300"
                                        data-testid="preset-requires-ai"
                                    >
                                        <Bot className="size-3" />
                                        {t('automations.presets.requiresAI')}
                                    </Badge>
                                )}
                            </div>
                            <CardTitle className="mt-2">
                                {text(p.title)}
                            </CardTitle>
                            <CardDescription>
                                {text(p.description)}
                            </CardDescription>
                        </CardHeader>
                        <CardContent className="flex-1 space-y-3 text-sm">
                            <p className="flex items-start gap-2">
                                <Zap className="mt-0.5 size-4 shrink-0 text-primary" />
                                <span>
                                    {describeTrigger(
                                        p.trigger_type,
                                        p.trigger,
                                        t,
                                    )}
                                </span>
                            </p>
                            <ul className="flex flex-wrap gap-1.5">
                                {types.map((type, i) => (
                                    <li
                                        key={`${type}-${i}`}
                                        className="flex items-center gap-1 rounded-md bg-muted px-1.5 py-0.5 text-xs"
                                    >
                                        <StepIcon
                                            type={type}
                                            className="size-3"
                                        />
                                        {stepLabel(type, t)}
                                    </li>
                                ))}
                            </ul>
                            {p.setup.length > 0 && (
                                <div className="rounded-lg border border-sky-500/40 bg-sky-500/10 p-2.5 text-xs text-sky-800 dark:text-sky-300">
                                    <p className="mb-1 flex items-center gap-1.5 font-medium">
                                        <Wrench className="size-3.5" />
                                        {t('automations.presets.setup')}
                                    </p>
                                    <ul className="list-disc space-y-0.5 pl-5">
                                        {p.setup.map((s) => (
                                            <li key={`${s.step_id}-${s.field}`}>
                                                {text(s.label)}
                                            </li>
                                        ))}
                                    </ul>
                                </div>
                            )}
                            {p.guidance && (
                                <p className="rounded-lg border border-amber-500/40 bg-amber-500/10 p-2.5 text-xs text-amber-800 dark:text-amber-300">
                                    {text(p.guidance)}
                                </p>
                            )}
                            {p.requires_ai && aiEnabled === false && (
                                <p className="flex items-start gap-1.5 text-xs text-amber-700 dark:text-amber-400">
                                    <AlertTriangle className="mt-px size-3.5 shrink-0" />
                                    {t('automations.presets.aiOff')}
                                </p>
                            )}
                        </CardContent>
                        {canManage && (
                            <CardFooter>
                                <Button
                                    type="button"
                                    className="w-full"
                                    disabled={pending}
                                    data-testid={`install-preset-${p.key}`}
                                    onClick={() => install(p.key)}
                                >
                                    {busy === p.key ? (
                                        <Spinner />
                                    ) : (
                                        <Download />
                                    )}
                                    {t('automations.presets.install')}
                                </Button>
                            </CardFooter>
                        )}
                    </Card>
                );
            })}
        </div>
    );
}
