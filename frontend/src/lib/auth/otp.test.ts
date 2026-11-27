import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
    applyOtpBackspace,
    applyOtpInput,
    applyOtpPaste,
    emptyOtp,
    otpComplete,
} from './otp.ts';

test('a typed digit fills the box and moves to the next', () => {
    const r = applyOtpInput(emptyOtp(), 0, '4');
    assert.deepEqual(r.digits, ['4', '', '', '', '', '']);
    assert.equal(r.focus, 1);
});

test('letters and spaces are ignored', () => {
    const r = applyOtpInput(emptyOtp(), 2, 'a');
    assert.deepEqual(r.digits, emptyOtp());
    assert.equal(r.focus, 2);
});

test('typing over a filled box replaces its digit', () => {
    const start = ['1', '2', '', '', '', ''];
    const r = applyOtpInput(start, 0, '17');
    assert.deepEqual(r.digits, ['7', '2', '', '', '', '']);
    assert.equal(r.focus, 1);
});

test('the last box keeps focus once filled', () => {
    const start = ['1', '2', '3', '4', '5', ''];
    const r = applyOtpInput(start, 5, '6');
    assert.equal(r.focus, 5);
    assert.ok(otpComplete(r.digits));
});

test('autofill into the first box spreads over all six', () => {
    const r = applyOtpInput(emptyOtp(), 0, '123456');
    assert.deepEqual(r.digits, ['1', '2', '3', '4', '5', '6']);
    assert.equal(r.focus, 5);
});

test('a pasted full code starts at the first box wherever it lands', () => {
    const r = applyOtpPaste(['9', '9', '', '', '', ''], 3, ' 123 456 ');
    assert.deepEqual(r.digits, ['1', '2', '3', '4', '5', '6']);
    assert.ok(otpComplete(r.digits));
});

test('a partial paste fills from the focused box and stops at the end', () => {
    const r = applyOtpPaste(emptyOtp(), 4, '7890');
    assert.deepEqual(r.digits, ['', '', '', '', '7', '8']);
    assert.equal(r.focus, 5);
});

test('backspace clears the box, then steps back', () => {
    const full = ['1', '2', '3', '', '', ''];
    const a = applyOtpBackspace(full, 2);
    assert.deepEqual(a.digits, ['1', '2', '', '', '', '']);
    assert.equal(a.focus, 2);
    const b = applyOtpBackspace(a.digits, 2);
    assert.deepEqual(b.digits, ['1', '', '', '', '', '']);
    assert.equal(b.focus, 1);
    const c = applyOtpBackspace(emptyOtp(), 0);
    assert.equal(c.focus, 0);
});

test('incomplete codes are not complete', () => {
    assert.equal(otpComplete(['1', '2', '3', '4', '5', '']), false);
    assert.equal(otpComplete(emptyOtp()), false);
});
