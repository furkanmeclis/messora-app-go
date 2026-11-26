import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
    describeTrigger,
    newStep,
    pendingSetup,
    stepFromApi,
    stepProblems,
    stepToApi,
    triggerFromApi,
    triggerProblems,
    triggerToApi,
    type EditorStep,
} from './model.ts';

// Messages are not needed: keys stand in for them.
const t = ((key: string) => key) as unknown as Parameters<
    typeof stepProblems
>[1];

const fields = (s: EditorStep, open?: Set<string>) =>
    stepProblems(s, t, open).map((p) => p.field);

test('AI steps round-trip through the API shape', () => {
    const raw = {
        id: 'x1',
        type: 'ai_extract_fields',
        fields: [{ key: 'name', label: 'Ad' }, { key: 'attr:city' }],
        prompt: 'only Turkish cities',
        retry: { max: 2, backoff: 30 },
    };
    const s = stepFromApi(raw, 'x1');
    assert.deepEqual(stepToApi(s), {
        id: 'x1',
        type: 'ai_extract_fields',
        fields: [{ key: 'name', label: 'Ad' }, { key: 'attr:city' }],
        prompt: 'only Turkish cities',
        retry: { max: 2, backoff: 30 },
    });
    const intent = stepFromApi(
        { id: 'i', type: 'ai_detect_intent', intents: ['order_status'] },
        'i',
    );
    assert.deepEqual(stepToApi(intent), {
        id: 'i',
        type: 'ai_detect_intent',
        intents: ['order_status'],
    });
    const note = stepFromApi(
        { id: 'n', type: 'add_internal_note', text: 'hi', for_ai: true },
        'n',
    );
    assert.deepEqual(stepToApi(note), {
        id: 'n',
        type: 'add_internal_note',
        text: 'hi',
        for_ai: true,
    });
});

test('a retry policy is only sent for retryable steps and when on', () => {
    const tag = { ...newStep('add_tag', 'a'), tag: 'x' };
    assert.equal(
        stepToApi({ ...tag, retry: { max: 3, backoff: 10 } }).retry,
        undefined,
    );
    const reply = { ...newStep('reply', 'r'), text: 'hi' };
    assert.equal(
        stepToApi({ ...reply, retry: { max: 0, backoff: 10 } }).retry,
        undefined,
    );
    assert.deepEqual(
        stepToApi({ ...reply, retry: { max: 2, backoff: 5 } }).retry,
        {
            max: 2,
            backoff: 5,
        },
    );
});

test('retry validation', () => {
    const reply = { ...newStep('reply', 'r'), text: 'hi' };
    assert.deepEqual(fields({ ...reply, retry: { max: 2, backoff: 5 } }), []);
    assert.deepEqual(fields({ ...reply, retry: { max: 11, backoff: 5 } }), [
        'retry.max',
    ]);
    assert.deepEqual(fields({ ...reply, retry: { max: 1, backoff: 0 } }), [
        'retry.backoff',
    ]);
    assert.deepEqual(fields({ ...reply, retry: { max: 1, backoff: 3601 } }), [
        'retry.backoff',
    ]);
});

test('intent validation', () => {
    const s = newStep('ai_detect_intent', 'i');
    assert.deepEqual(fields(s), ['intents']);
    assert.deepEqual(fields({ ...s, intents: ['order_status'] }), []);
    assert.deepEqual(fields({ ...s, intents: ['Order Status'] }), ['intents']);
    assert.deepEqual(fields({ ...s, intents: ['other'] }), ['intents']);
    assert.deepEqual(
        fields({
            ...s,
            intents: Array.from({ length: 21 }, (_, i) => `i${i}`),
        }),
        ['intents'],
    );
    assert.deepEqual(fields({ ...s, intents: ['ok'], variable: '1x' }), [
        'variable',
    ]);
});

test('other new step validation', () => {
    assert.deepEqual(fields(newStep('ai_generate_message', 'g')), ['prompt']);
    assert.deepEqual(fields(newStep('add_internal_note', 'n')), ['text']);
    assert.deepEqual(fields(newStep('ai_classify_segment', 'c')), ['tags']);
    assert.deepEqual(
        fields({ ...newStep('ai_classify_segment', 'c'), tags: ['vip'] }),
        [],
    );
    assert.deepEqual(
        fields({
            ...newStep('ai_classify_segment', 'c'),
            tags: ['vip'],
            group_ids: ['g'],
        }),
        ['tags'],
    );
    const ex = newStep('ai_extract_fields', 'e');
    assert.deepEqual(fields(ex), []);
    assert.deepEqual(
        fields({ ...ex, fields: [{ key: 'phone', label: '', rowKey: 'k' }] }),
        ['fields'],
    );
    assert.deepEqual(fields(newStep('ai_auto_reply', 'a')), []);
    assert.deepEqual(fields(newStep('handoff_to_human', 'h')), []);
});

test('setup: an open field does not block, and closes once filled', () => {
    const tpl = newStep('send_template', 'follow_template');
    assert.deepEqual(fields(tpl), ['template']);
    assert.deepEqual(fields(tpl, new Set(['template'])), []);
    const setup = [
        { step_id: 'follow_template', field: 'template', label: 'Choose' },
        { step_id: 'positive_assign', field: 'assignee_id', label: 'Agent' },
        { step_id: 'gone', field: 'template', label: 'Removed step' },
    ];
    const assign = newStep('assign', 'positive_assign');
    assert.deepEqual(
        pendingSetup(setup, [tpl, assign]).map((s) => s.step_id),
        ['follow_template', 'positive_assign'],
    );
    const filled = { ...tpl, template: { name: 'follow', language: 'tr' } };
    assert.deepEqual(
        pendingSetup(setup, [filled, { ...assign, assignee_id: 'u1' }]),
        [],
    );
});

test('new triggers convert to and from the API', () => {
    const w = triggerFromApi({ minutes: 30, include_answered: true });
    assert.deepEqual(triggerToApi('window_closing', w), {
        minutes: 30,
        include_answered: true,
    });
    // window_closing defaults to 60 minutes.
    assert.deepEqual(triggerToApi('window_closing', triggerFromApi({})), {
        minutes: 60,
    });
    const n = triggerFromApi({ no_reply_from: 'agent', minutes: 15 });
    assert.deepEqual(
        triggerToApi('no_reply', { ...n, include_campaigns: true }),
        {
            no_reply_from: 'agent',
            minutes: 15,
        },
    );
    assert.deepEqual(
        triggerToApi('reaction', triggerFromApi({ reaction: 'negative' })),
        { reaction: 'negative' },
    );
    assert.deepEqual(
        triggerToApi(
            'ai_escalation',
            triggerFromApi({ reasons: ['customer_request'] }),
        ),
        { reasons: ['customer_request'] },
    );
    assert.deepEqual(
        triggerProblems('window_closing', { ...w, minutes: 2 }, t).map(
            (p) => p.field,
        ),
        ['minutes'],
    );
    assert.deepEqual(
        triggerProblems('no_reply', { ...n, minutes: 50000 }, t).map(
            (p) => p.field,
        ),
        ['minutes'],
    );
    assert.equal(
        describeTrigger('window_closing', { minutes: 60 }, t),
        'automations.summary.windowClosing',
    );
});
