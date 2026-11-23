'use client';

import { useId, useState } from 'react';
import { X } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { TagChip } from './tag-chip';

type TagOption = { name: string; color: string };

/**
 * Free-text tag list with suggestions from the existing tags. Enter or a
 * comma adds the typed tag; Backspace in an empty box removes the last one.
 * New names become new tags when the form is saved.
 */
export function TagsInput({
    id,
    value,
    onChange,
    suggestions,
    max = 50,
    disabled,
    invalid,
    describedBy,
}: {
    id: string;
    value: string[];
    onChange: (tags: string[]) => void;
    suggestions: TagOption[];
    max?: number;
    disabled?: boolean;
    invalid?: boolean;
    describedBy?: string;
}) {
    const listId = useId();
    const [text, setText] = useState('');
    const has = (name: string) =>
        value.some((v) => v.toLowerCase() === name.toLowerCase());
    const add = (raw: string) => {
        const name = raw.trim().slice(0, 60);
        if (!name || has(name) || value.length >= max) return;
        // Use the existing tag's spelling when it differs only in case.
        const known = suggestions.find(
            (s) => s.name.toLowerCase() === name.toLowerCase(),
        );
        onChange([...value, known?.name ?? name]);
    };
    const color = (name: string) =>
        suggestions.find((s) => s.name.toLowerCase() === name.toLowerCase())
            ?.color ?? 'gray';

    return (
        <div className="space-y-2">
            {value.length > 0 && (
                <ul className="flex flex-wrap gap-1" aria-label="Chosen tags">
                    {value.map((t) => (
                        <li key={t}>
                            <TagChip name={t} color={color(t)}>
                                {!disabled && (
                                    <button
                                        type="button"
                                        className="-mr-1 rounded-full outline-none hover:text-destructive focus-visible:ring-2 focus-visible:ring-ring"
                                        aria-label={`Remove tag ${t}`}
                                        onClick={() =>
                                            onChange(
                                                value.filter((v) => v !== t),
                                            )
                                        }
                                    >
                                        <X className="size-3" />
                                    </button>
                                )}
                            </TagChip>
                        </li>
                    ))}
                </ul>
            )}
            {!disabled && (
                <>
                    <Input
                        id={id}
                        list={listId}
                        value={text}
                        maxLength={60}
                        placeholder={
                            value.length >= max
                                ? `At most ${max} tags`
                                : 'Type a tag and press Enter'
                        }
                        disabled={value.length >= max}
                        aria-invalid={invalid || undefined}
                        aria-describedby={describedBy}
                        onChange={(e) => {
                            const v = e.target.value;
                            // Picking a suggestion replaces the whole text
                            // (no inputType, or insertReplacementText).
                            const kind =
                                (e.nativeEvent as InputEvent).inputType ?? '';
                            const picked =
                                (kind === '' ||
                                    kind === 'insertReplacementText') &&
                                suggestions.some((s) => s.name === v);
                            if (v.endsWith(',')) {
                                add(v.slice(0, -1));
                                setText('');
                            } else if (picked) {
                                add(v);
                                setText('');
                            } else {
                                setText(v);
                            }
                        }}
                        onKeyDown={(e) => {
                            if (e.key === 'Enter') {
                                e.preventDefault();
                                add(text);
                                setText('');
                            } else if (
                                e.key === 'Backspace' &&
                                text === '' &&
                                value.length > 0
                            ) {
                                onChange(value.slice(0, -1));
                            }
                        }}
                        onBlur={() => {
                            if (text.trim()) {
                                add(text);
                                setText('');
                            }
                        }}
                    />
                    <datalist id={listId}>
                        {suggestions
                            .filter((s) => !has(s.name))
                            .map((s) => (
                                <option key={s.name} value={s.name} />
                            ))}
                    </datalist>
                </>
            )}
        </div>
    );
}
