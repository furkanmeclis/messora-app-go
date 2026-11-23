'use client';

import { GripVertical } from 'lucide-react';
import { useT } from '@/i18n/client';
import { cn } from '@/lib/utils';
import { CATEGORIES, mk, STEP_TYPES, type StepType, stepLabel } from './model';
import { CATEGORY_STYLE, StepIcon } from './nodes';

/** Drag-and-drop payload type for a palette item. */
export const DND_TYPE = 'application/x-messora-step';

/**
 * The node palette: every step type by category. Drag an item onto the
 * canvas, or click it to add it after the selected node.
 */
export function Palette({
    onAdd,
    disabled,
}: {
    onAdd: (type: StepType) => void;
    disabled?: boolean;
}) {
    const t = useT();
    return (
        <nav
            aria-label={t('automations.palette.title')}
            className="flex h-full flex-col"
        >
            <div className="border-b px-3 py-2.5">
                <h2 className="text-sm font-semibold">
                    {t('automations.palette.title')}
                </h2>
                <p className="text-xs text-muted-foreground">
                    {t('automations.palette.hint')}
                </p>
            </div>
            <div className="min-h-0 flex-1 space-y-4 overflow-y-auto p-2">
                {CATEGORIES.map((c) => (
                    <section key={c} aria-labelledby={`palette-${c}`}>
                        <h3
                            id={`palette-${c}`}
                            className="mb-1 flex items-center gap-1.5 px-1.5 text-[10px] font-semibold tracking-wider text-muted-foreground uppercase"
                        >
                            <span
                                className={cn(
                                    'size-1.5 rounded-full',
                                    CATEGORY_STYLE[c].dot,
                                )}
                            />
                            {t(mk(`automations.categories.${c}`))}
                        </h3>
                        <ul className="space-y-0.5">
                            {STEP_TYPES.filter((s) => s.category === c).map(
                                (s) => (
                                    <li key={s.type}>
                                        <button
                                            type="button"
                                            draggable={!disabled}
                                            disabled={disabled}
                                            data-testid="palette-item"
                                            data-step-type={s.type}
                                            title={t(
                                                mk(
                                                    `automations.steps.${s.type}.hint`,
                                                ),
                                            )}
                                            aria-label={t(
                                                'automations.palette.add',
                                                {
                                                    step: stepLabel(s.type, t),
                                                },
                                            )}
                                            onClick={() => onAdd(s.type)}
                                            onDragStart={(e) => {
                                                e.dataTransfer.setData(
                                                    DND_TYPE,
                                                    s.type,
                                                );
                                                e.dataTransfer.effectAllowed =
                                                    'copy';
                                            }}
                                            className="group flex w-full cursor-grab items-center gap-2 rounded-lg px-1.5 py-1.5 text-left text-sm transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none active:cursor-grabbing disabled:cursor-not-allowed disabled:opacity-50"
                                        >
                                            <span
                                                className={cn(
                                                    'flex size-7 shrink-0 items-center justify-center rounded-md',
                                                    CATEGORY_STYLE[c].chip,
                                                )}
                                            >
                                                <StepIcon
                                                    type={s.type}
                                                    className="size-3.5"
                                                />
                                            </span>
                                            <span className="min-w-0 flex-1 truncate">
                                                {stepLabel(s.type, t)}
                                            </span>
                                            <GripVertical className="size-3.5 shrink-0 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100" />
                                        </button>
                                    </li>
                                ),
                            )}
                        </ul>
                    </section>
                ))}
            </div>
        </nav>
    );
}
