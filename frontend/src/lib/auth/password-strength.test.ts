import { test } from 'node:test';
import assert from 'node:assert/strict';
import { passwordStrength } from './password-strength.ts';

test('empty password scores zero', () => {
    assert.deepEqual(passwordStrength(''), {
        score: 0,
        label: 'empty',
        tooShort: true,
    });
});

test('anything under twelve characters is weak and too short', () => {
    const s = passwordStrength('Xk9!qm2#');
    assert.equal(s.tooShort, true);
    assert.equal(s.score, 1);
    assert.equal(s.label, 'weak');
});

test('common words and sequences stay weak however long', () => {
    for (const p of [
        'password1234',
        'Sifre12345678',
        'qwertyuiop12',
        '123456789012',
    ]) {
        assert.ok(passwordStrength(p).score <= 1, p);
    }
    assert.ok(passwordStrength('aaaaaaaaaaaaaaaa').score <= 1);
});

test('long and varied passwords are strong', () => {
    assert.equal(passwordStrength('Correct-horse-staple-91').label, 'strong');
    assert.equal(passwordStrength('Another-strong-pass-73').score, 4);
});

test('length alone reaches good, not strong', () => {
    const s = passwordStrength('meadow river lantern');
    assert.equal(s.tooShort, false);
    assert.ok(s.score >= 2 && s.score <= 3, String(s.score));
});

test('personal words count as guessable', () => {
    const pw = 'Olivia-Owner-2026!x';
    assert.ok(passwordStrength(pw).score >= 3);
    assert.ok(
        passwordStrength(pw, ['Olivia Owner', 'olivia@example.com']).score <= 1,
    );
});

test('the score never goes down as a good password grows', () => {
    let last = 0;
    for (const p of ['Tq7!', 'Tq7!vRw2', 'Tq7!vRw2#nLp', 'Tq7!vRw2#nLp9$Zx']) {
        const s = passwordStrength(p).score;
        assert.ok(s >= last, p);
        last = s;
    }
});
