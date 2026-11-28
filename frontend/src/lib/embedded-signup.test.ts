import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
    isMetaOrigin,
    isSignupPage,
    loginOptions,
    parseSignupMessage,
} from './embedded-signup.ts';

test('only https facebook.com origins are Meta', () => {
    assert.equal(isMetaOrigin('https://www.facebook.com'), true);
    assert.equal(isMetaOrigin('https://web.facebook.com'), true);
    assert.equal(isMetaOrigin('https://facebook.com'), true);
    assert.equal(isMetaOrigin('http://www.facebook.com'), false);
    assert.equal(isMetaOrigin('https://evilfacebook.com'), false);
    assert.equal(isMetaOrigin('https://facebook.com.evil.test'), false);
    assert.equal(isMetaOrigin('https://acme.messora.tech'), false);
    assert.equal(isMetaOrigin('null'), false);
});

test('FINISH carries the WABA and the number', () => {
    const msg = JSON.stringify({
        type: 'WA_EMBEDDED_SIGNUP',
        event: 'FINISH',
        data: { phone_number_id: '1098', waba_id: '2030', business_id: '77' },
    });
    assert.deepEqual(parseSignupMessage(msg), {
        kind: 'finish',
        wabaId: '2030',
        phoneNumberId: '1098',
        businessId: '77',
    });
    // An object works too; a non-numeric id is refused.
    assert.equal(
        parseSignupMessage({
            type: 'WA_EMBEDDED_SIGNUP',
            event: 'FINISH',
            data: { phone_number_id: '1', waba_id: '2 OR 1=1' },
        }),
        null,
    );
    assert.equal(
        parseSignupMessage({
            type: 'WA_EMBEDDED_SIGNUP',
            event: 'FINISH',
            data: { waba_id: '2' },
        })?.kind,
        'error',
    );
});

test('CANCEL, ERROR, coexistence and foreign messages', () => {
    assert.deepEqual(
        parseSignupMessage({
            type: 'WA_EMBEDDED_SIGNUP',
            event: 'CANCEL',
            data: { current_step: 'PHONE_NUMBER_SETUP' },
        }),
        { kind: 'cancel', step: 'PHONE_NUMBER_SETUP' },
    );
    assert.deepEqual(
        parseSignupMessage({
            type: 'WA_EMBEDDED_SIGNUP',
            event: 'ERROR',
            data: { error_message: 'boom', error_code: 524126 },
        }),
        { kind: 'error', message: 'boom', code: '524126' },
    );
    assert.equal(
        parseSignupMessage({
            type: 'WA_EMBEDDED_SIGNUP',
            event: 'FINISH_WHATSAPP_BUSINESS_APP_ONBOARDING',
            data: { waba_id: '9' },
        })?.kind,
        'finish_app',
    );
    assert.equal(parseSignupMessage('not json'), null);
    assert.equal(parseSignupMessage({ type: 'OTHER', event: 'FINISH' }), null);
    assert.equal(parseSignupMessage(null), null);
});

test('login options and the signup page', () => {
    const o = loginOptions('123');
    assert.equal(o.config_id, '123');
    assert.equal(o.response_type, 'code');
    assert.equal(o.override_default_response_type, true);
    assert.deepEqual(o.extras, { setup: {} });
    assert.equal(isSignupPage('/settings/meta'), true);
    assert.equal(isSignupPage('/settings/meta/x'), false);
    assert.equal(isSignupPage('/settings'), false);
});
