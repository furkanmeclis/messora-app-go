import { test } from 'node:test';
import assert from 'node:assert/strict';
import { estimateTokens, parseIntents } from './ai.ts';

test('estimateTokens is about four characters per token', () => {
    assert.equal(estimateTokens(''), 0);
    assert.equal(estimateTokens('   '), 0);
    assert.equal(estimateTokens('abcd'), 1);
    assert.equal(estimateTokens('abcde'), 2);
    assert.equal(estimateTokens('x'.repeat(4000)), 1000);
});

test('parseIntents normalises to unique snake_case', () => {
    assert.deepEqual(
        parseIntents('complaint, Refund request,complaint, ,  x-y '),
        ['complaint', 'refund_request', 'x_y'],
    );
    assert.deepEqual(parseIntents(''), []);
});
