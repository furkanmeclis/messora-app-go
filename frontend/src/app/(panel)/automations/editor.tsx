'use client';

import { useEffect, useMemo, useState, useTransition } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { AlertCircle, AlertTriangle, History } from 'lucide-react';
import { toast } from 'sonner';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import {
    Field,
    FieldDescription,
    FieldGroup,
    FieldLabel,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Spinner } from '@/components/ui/spinner';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import type { Automation, AutomationTriggerType } from '@/lib/types';
import { createAutomationAction, updateAutomationAction } from './actions';
import { DeleteButton } from './automations-client';
import type { EditorOptions } from './data';
import {
    DurationInput,
    FieldMessage,
    KeywordsInput,
    SelectField,
    SuggestInput,
} from './inputs';
import {
    countSteps,
    describeTrigger,
    editList,
    editStep,
    type EditorStep,
    mapErrors,
    MATCH_MODES,
    MAX_KEYWORDS,
    MAX_MUTE_SECONDS,
    MAX_STEPS,
    MESSAGE_STATUSES,
    type StepError,
    stepsFromApi,
    stepsToApi,
    TRIGGER_TYPES,
    type TriggerForm,
    triggerFromApi,
    triggerToApi,
} from './model';
import { EditorContext, type EditorContextValue, StepList } from './steps';

type Errors = {
    steps: Record<string, StepError>;
    other: Record<string, string>;
    message?: string;
};

const NO_ERRORS: Errors = { steps: {}, other: {} };

const TRIGGER_FIELDS = [
    'trigger.match',
    'trigger.keywords',
    'trigger.phone_number_id',
    'trigger.tag',
    'trigger.status',
    'trigger.campaign_id',
    'trigger.cooldown_seconds',
];

export function AutomationEditor({
    automation,
    options,
    canManage,
}: {
    automation: Automation | null;
    options: EditorOptions;
    canManage: boolean;
}) {
    const router = useRouter();
    const readOnly = !canManage;
    const [name, setName] = useState(automation?.name ?? '');
    const [description, setDescription] = useState(
        automation?.description ?? '',
    );
    const [active, setActive] = useState(automation?.active ?? false);
    const [triggerType, setTriggerType] = useState<AutomationTriggerType>(
        automation?.trigger_type ?? 'message_received',
    );
    const [trigger, setTrigger] = useState<TriggerForm>(() =>
        triggerFromApi(automation?.trigger),
    );
    const [steps, setSteps] = useState<EditorStep[]>(() =>
        stepsFromApi(automation?.steps),
    );
    const [errors, setErrors] = useState<Errors>(NO_ERRORS);
    const [pending, startTransition] = useTransition();

    const definition = () =>
        JSON.stringify({
            t: triggerType,
            o: triggerToApi(triggerType, trigger),
            s: stepsToApi(steps),
        });
    const details = () => JSON.stringify({ name, description, active });
    const [baseline, setBaseline] = useState(() => ({
        definition: definition(),
        details: details(),
    }));
    const redefined = definition() !== baseline.definition;
    const dirty = redefined || details() !== baseline.details;

    // Warn before leaving with unsaved changes.
    useEffect(() => {
        if (!dirty || readOnly) return;
        const onBeforeUnload = (e: BeforeUnloadEvent) => e.preventDefault();
        window.addEventListener('beforeunload', onBeforeUnload);
        return () => window.removeEventListener('beforeunload', onBeforeUnload);
    }, [dirty, readOnly]);

    const total = countSteps(steps);
    const ctx: EditorContextValue = useMemo(
        () => ({
            options,
            readOnly,
            errors: errors.steps,
            total,
            editStep: (key, fn) => setSteps((s) => editStep(s, key, fn)),
            editList: (list, fn) => setSteps((s) => editList(s, list, fn)),
        }),
        [options, readOnly, errors.steps, total],
    );

    const setT = (patch: Partial<TriggerForm>) =>
        setTrigger((t) => ({ ...t, ...patch }));
    const tErr = (f: string) => errors.other[`trigger.${f}`];
    // Trigger errors no option field shows (e.g. "trigger": invalid).
    const otherTriggerErrors = Object.entries(errors.other)
        .filter(
            ([k]) =>
                (k === 'trigger' || k.startsWith('trigger.')) &&
                !TRIGGER_FIELDS.includes(k) &&
                !k.startsWith('trigger.keywords.'),
        )
        .map(([, v]) => v);

    const save = () =>
        startTransition(async () => {
            const apiSteps = stepsToApi(steps);
            const apiTrigger = triggerToApi(triggerType, trigger);
            const r = automation
                ? await updateAutomationAction(automation.id, {
                      name: name.trim(),
                      description,
                      active,
                      ...(redefined
                          ? {
                                trigger_type: triggerType,
                                trigger: apiTrigger,
                                steps: apiSteps,
                            }
                          : {}),
                  })
                : await createAutomationAction({
                      name: name.trim(),
                      description,
                      active,
                      trigger_type: triggerType,
                      trigger: apiTrigger,
                      steps: apiSteps,
                  });
            if (r.ok) {
                setErrors(NO_ERRORS);
                setBaseline({ definition: definition(), details: details() });
                if (r.message) toast.success(r.message);
                if (!automation) router.replace(`/automations/${r.data.id}`);
                else router.refresh();
                return;
            }
            const mapped = mapErrors(steps, r.fieldErrors);
            // Keywords are reported per item by the schema check.
            for (const k of Object.keys(mapped.other)) {
                if (k.startsWith('trigger.keywords.')) {
                    mapped.other['trigger.keywords'] ??= mapped.other[k]!;
                }
            }
            // Errors no field shows go to the banner at the top.
            const shown = (k: string) =>
                ['name', 'description', 'steps', 'trigger_type'].includes(k) ||
                k === 'trigger' ||
                k.startsWith('trigger.');
            const unshown = Object.entries(mapped.other)
                .filter(([k]) => !shown(k))
                .map(([, v]) => v);
            const handled =
                unshown.length === 0 &&
                (Object.keys(mapped.steps).length > 0 ||
                    Object.keys(mapped.other).length > 0);
            setErrors({
                ...mapped,
                message: handled
                    ? undefined
                    : [r.message, ...unshown].join(' '),
            });
            toast.error(
                handled ? 'Please correct the highlighted fields.' : r.message,
            );
            const first = Object.keys(mapped.steps)[0];
            if (first) {
                requestAnimationFrame(() => {
                    const el = document.getElementById(`step-${first}`);
                    el?.scrollIntoView({ behavior: 'smooth', block: 'center' });
                    el?.focus({ preventScroll: true });
                });
            }
        });

    const phoneOptions = [
        { value: 'any', label: 'Any number' },
        ...options.phones.map((p) => ({ value: p.id, label: p.label })),
    ];
    if (
        trigger.phone_number_id &&
        !phoneOptions.some((p) => p.value === trigger.phone_number_id)
    ) {
        phoneOptions.push({
            value: trigger.phone_number_id,
            label: 'One number',
        });
    }
    const campaignOptions = [
        { value: 'any', label: 'Any message' },
        ...options.campaigns.map((c) => ({ value: c.id, label: c.label })),
    ];
    if (
        trigger.campaign_id &&
        !campaignOptions.some((c) => c.value === trigger.campaign_id)
    ) {
        campaignOptions.push({
            value: trigger.campaign_id,
            label: 'One campaign',
        });
    }
    const summary = describeTrigger(
        triggerType,
        triggerToApi(triggerType, trigger),
        {
            phones: new Map(options.phones.map((p) => [p.id, p.label])),
            campaigns: new Map(options.campaigns.map((c) => [c.id, c.label])),
        },
    );

    return (
        <form
            className="space-y-6 pb-24"
            aria-label={automation ? 'Edit automation' : 'New automation'}
            onSubmit={(e) => {
                e.preventDefault();
                if (!readOnly) save();
            }}
        >
            <div className="flex flex-wrap items-center gap-2">
                {automation && (
                    <Button asChild variant="outline">
                        <Link href={`/automations/${automation.id}/runs`}>
                            <History /> Run history
                        </Link>
                    </Button>
                )}
                {automation && canManage && (
                    <DeleteButton
                        automation={automation}
                        redirectTo="/automations"
                        withLabel
                    />
                )}
            </div>

            {readOnly && (
                <Alert>
                    <AlertCircle />
                    <AlertDescription>
                        You can view this automation but not change it.
                    </AlertDescription>
                </Alert>
            )}
            {errors.message && (
                <Alert variant="destructive">
                    <AlertCircle />
                    <AlertDescription>{errors.message}</AlertDescription>
                </Alert>
            )}

            <fieldset disabled={readOnly} className="min-w-0 space-y-6">
                <Card>
                    <CardHeader>
                        <CardTitle>Details</CardTitle>
                    </CardHeader>
                    <CardContent>
                        <FieldGroup>
                            <Field
                                data-invalid={
                                    errors.other.name ? true : undefined
                                }
                            >
                                <FieldLabel htmlFor="automation-name">
                                    Name
                                </FieldLabel>
                                <Input
                                    id="automation-name"
                                    required
                                    maxLength={120}
                                    value={name}
                                    placeholder="Price question"
                                    aria-invalid={
                                        errors.other.name ? true : undefined
                                    }
                                    onChange={(e) => setName(e.target.value)}
                                />
                                <FieldMessage
                                    id="automation-name"
                                    error={errors.other.name}
                                />
                            </Field>
                            <Field>
                                <FieldLabel htmlFor="automation-description">
                                    Description
                                </FieldLabel>
                                <Textarea
                                    id="automation-description"
                                    maxLength={500}
                                    rows={2}
                                    value={description}
                                    onChange={(e) =>
                                        setDescription(e.target.value)
                                    }
                                />
                                <FieldMessage
                                    id="automation-description"
                                    error={errors.other.description}
                                />
                            </Field>
                            <Field orientation="horizontal">
                                <Switch
                                    id="automation-active"
                                    checked={active}
                                    onCheckedChange={setActive}
                                />
                                <div>
                                    <FieldLabel htmlFor="automation-active">
                                        Active
                                    </FieldLabel>
                                    <FieldDescription>
                                        Inactive automations never start.
                                    </FieldDescription>
                                </div>
                            </Field>
                        </FieldGroup>
                    </CardContent>
                </Card>

                <Card>
                    <CardHeader>
                        <CardTitle>Trigger</CardTitle>
                        <CardDescription data-testid="trigger-summary">
                            {summary}
                        </CardDescription>
                    </CardHeader>
                    <CardContent>
                        <FieldGroup>
                            <SelectField
                                id="trigger-type"
                                label="When"
                                value={triggerType}
                                onChange={(v) =>
                                    setTriggerType(v as AutomationTriggerType)
                                }
                                options={TRIGGER_TYPES}
                                hint={
                                    TRIGGER_TYPES.find(
                                        (t) => t.value === triggerType,
                                    )?.description
                                }
                                error={errors.other.trigger_type}
                            />
                            {triggerType === 'message_received' && (
                                <>
                                    <SelectField
                                        id="trigger-match"
                                        label="Message"
                                        value={trigger.match}
                                        onChange={(match) => setT({ match })}
                                        options={MATCH_MODES}
                                        error={tErr('match')}
                                    />
                                    {trigger.match !== 'any' && (
                                        <KeywordsInput
                                            id="trigger-keywords"
                                            label="Keywords"
                                            value={trigger.keywords}
                                            onChange={(keywords) =>
                                                setT({ keywords })
                                            }
                                            max={MAX_KEYWORDS}
                                            readOnly={readOnly}
                                            error={tErr('keywords')}
                                            hint={`Any keyword matches; upper and lower case are ignored. Up to ${MAX_KEYWORDS}.`}
                                        />
                                    )}
                                </>
                            )}
                            {(triggerType === 'message_received' ||
                                triggerType === 'new_contact') && (
                                <SelectField
                                    id="trigger-phone"
                                    label="On number"
                                    value={trigger.phone_number_id || 'any'}
                                    onChange={(v) =>
                                        setT({
                                            phone_number_id:
                                                v === 'any' ? '' : v,
                                        })
                                    }
                                    options={phoneOptions}
                                    error={tErr('phone_number_id')}
                                />
                            )}
                            {triggerType === 'tag_added' && (
                                <SuggestInput
                                    id="trigger-tag"
                                    label="Tag"
                                    maxLength={60}
                                    value={trigger.tag}
                                    onChange={(tag) => setT({ tag })}
                                    suggestions={options.tags}
                                    error={tErr('tag')}
                                    hint="CSV imports do not fire this trigger."
                                />
                            )}
                            {triggerType === 'message_status' && (
                                <>
                                    <SelectField
                                        id="trigger-status"
                                        label="Status"
                                        value={trigger.status}
                                        onChange={(status) => setT({ status })}
                                        options={MESSAGE_STATUSES.map((s) => ({
                                            value: s,
                                            label:
                                                s[0]!.toUpperCase() +
                                                s.slice(1),
                                        }))}
                                        error={tErr('status')}
                                    />
                                    <SelectField
                                        id="trigger-campaign"
                                        label="Campaign"
                                        value={trigger.campaign_id || 'any'}
                                        onChange={(v) =>
                                            setT({
                                                campaign_id:
                                                    v === 'any' ? '' : v,
                                            })
                                        }
                                        options={campaignOptions}
                                        error={tErr('campaign_id')}
                                        hint="Limit to messages of one campaign."
                                    />
                                </>
                            )}
                            <DurationInput
                                id="trigger-cooldown"
                                label="Cooldown per contact"
                                seconds={trigger.cooldown_seconds}
                                onChange={(cooldown_seconds) =>
                                    setT({ cooldown_seconds })
                                }
                                min={0}
                                max={MAX_MUTE_SECONDS}
                                error={tErr('cooldown_seconds')}
                                hint="At most one run per contact within this time; 0 means no limit."
                            />
                            {otherTriggerErrors.map((m) => (
                                <p
                                    key={m}
                                    role="alert"
                                    className="text-sm text-destructive"
                                >
                                    {m}
                                </p>
                            ))}
                        </FieldGroup>
                    </CardContent>
                </Card>

                <Card>
                    <CardHeader>
                        <CardTitle>Steps</CardTitle>
                        <CardDescription>
                            Run from top to bottom. {total} of {MAX_STEPS} steps
                            used.
                        </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        {errors.other.steps && (
                            <Alert variant="destructive">
                                <AlertCircle />
                                <AlertDescription data-testid="steps-error">
                                    {errors.other.steps}
                                </AlertDescription>
                            </Alert>
                        )}
                        <EditorContext value={ctx}>
                            <StepList
                                steps={steps}
                                list="root"
                                depth={1}
                                labelPrefix="Step "
                            />
                        </EditorContext>
                    </CardContent>
                </Card>
            </fieldset>

            {!readOnly && (
                <div className="sticky bottom-0 z-10 -mx-4 flex flex-wrap items-center gap-3 border-t bg-background/95 px-4 py-3 backdrop-blur md:-mx-6 md:px-6">
                    {automation && redefined && (
                        <p
                            className="flex items-center gap-2 text-sm text-amber-700 dark:text-amber-400"
                            data-testid="redefine-warning"
                        >
                            <AlertTriangle className="size-4 shrink-0" />
                            Saving changed trigger or steps cancels runs in
                            progress.
                        </p>
                    )}
                    <div className="ml-auto flex gap-2">
                        <Button asChild variant="ghost">
                            <Link href="/automations">Cancel</Link>
                        </Button>
                        <Button type="submit" disabled={pending}>
                            {pending && <Spinner />}
                            {automation ? 'Save changes' : 'Create automation'}
                        </Button>
                    </div>
                </div>
            )}
        </form>
    );
}
