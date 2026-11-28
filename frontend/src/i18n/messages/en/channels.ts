const channels = {
    title: 'Channels',
    description:
        'WhatsApp Business accounts, their phone numbers and the webhook Meta calls.',
    loading: 'Loading',
    accounts: {
        title: 'WhatsApp accounts',
        description:
            'Access tokens and app secrets are stored encrypted and are never shown again.',
        label: 'WhatsApp accounts',
        empty: 'No WhatsApp account connected',
        emptyManage:
            'Connect a WhatsApp Business account to send messages, templates and campaigns.',
        emptyAsk:
            'Ask an administrator to connect a WhatsApp Business account.',
        statuses: {
            active: 'Active',
            error: 'Error',
            disabled: 'Disabled',
        },
        noAppSecret: 'No app secret',
        wabaId: 'WABA ID',
        appId: 'App ID',
        lastSynced: 'Last synced',
        connected: 'Connected',
        sync: 'Sync',
        synced: 'Synced {numbers} phone number(s) and {templates} template(s).',
        enable: 'Enable',
        disable: 'Disable',
        lastError: 'Last error',
        lastErrorHint:
            'Sending fails fast until the connection check passes or the token is replaced.',
        viewTemplates: 'View templates',
    },
    connect: {
        open: 'Connect account',
        title: 'Connect a WhatsApp Business account',
        description:
            'Use a permanent system-user token with the whatsapp_business_messaging and whatsapp_business_management permissions. Messora checks it with Meta before saving anything.',
        name: 'Name',
        namePlaceholder: 'Main brand',
        wabaId: 'WhatsApp Business Account ID',
        digitsOnly: 'Digits only',
        wabaIdHint:
            'Digits only, from Meta Business Suite → WhatsApp accounts.',
        accessToken: 'Access token',
        accessTokenHint: 'Stored encrypted; it is never displayed again.',
        appId: 'App ID',
        appIdHint: 'Your Meta app: App settings → Basic.',
        appSecret: 'App secret',
        appSecretHint:
            'Same page as the App ID. It verifies the signature of every webhook your app sends; stored encrypted, never shown again.',
        submit: 'Connect',
        done: 'Account connected. Phone numbers and templates were imported.',
    },
    edit: {
        open: 'Edit / replace token',
        title: 'Edit {name}',
        description:
            'Leave a secret empty to keep the current one. A new token is checked with Meta first.',
        name: 'Name',
        accessToken: 'New access token',
        accessTokenHint:
            'Checked with Meta first (validity, permissions, expiry). A working token reactivates the account immediately.',
        appIdHint:
            "With the App ID and secret, Messora can read the token's expiry and turn short-lived user tokens into 60-day tokens.",
        appSecret: 'New app secret',
        secretConfigured: 'An app secret is configured.',
        noSecret: 'No app secret yet: webhooks are rejected until you add one.',
        disabled: 'Account disabled.',
        enabled: 'Account enabled.',
        credentialsUpdated: 'Credentials updated.',
        updated: 'Account updated.',
    },
    token: {
        permanent: 'Permanent token',
        until: 'Valid until {date}',
        expired: 'Token expired',
        unknown: 'Token not checked',
        systemUser: 'System User token',
        type: '{type} token',
        checkedAt: 'Checked {date}',
        neverChecked: 'Never checked',
        check: 'Check connection',
        checkOk: 'The connection works.',
        checkFailed: 'Meta refused the token; see the account.',
        replace: 'Replace token',
        problemTitle: 'Meta refused this account',
        problemHint:
            'Nothing can be sent from it until the token is replaced. Messages already queued fail with this reason.',
    },
    guide: {
        title: 'Get a token that never expires',
        why: 'Tokens copied from the App Dashboard or Graph Explorer expire within hours or days, and the account then stops sending. A System User token does not expire.',
        step1: 'Open Meta Business Settings → Users → System users and add a system user with the Admin role.',
        step2: 'Assign assets: your WhatsApp account (Full control) and your Meta app.',
        step3: 'Click "Generate new token", choose your app and set Token expiration to "Never".',
        step4: 'Tick the permissions whatsapp_business_messaging and whatsapp_business_management.',
        step5: 'Copy the token and paste it here. Messora checks it with Meta before saving.',
        open: 'Open System users in Business Settings',
    },
    disconnect: {
        open: 'Disconnect',
        title: 'Disconnect {name}?',
        blockedTitle: 'This account cannot be removed',
        description:
            'Its phone numbers and templates are removed from Messora. Accounts with message history cannot be removed; disable them instead.',
        blockedDescription:
            '{reason} Conversations and delivery history refer to its numbers, so it stays; a disabled account sends nothing and can be enabled again later.',
        disableInstead: 'Disable instead',
        confirm: 'Disconnect',
        done: 'Account disconnected.',
    },
    numbers: {
        title: 'Phone numbers',
        description:
            'The default number sends whenever a request or campaign names no sender.',
        empty: 'No phone numbers yet. They are imported when an account is connected or synced.',
        number: 'Number',
        verifiedName: 'Verified name',
        account: 'Account',
        quality: 'Quality',
        messagingLimit: 'Messaging limit',
        throughput: 'Throughput: {level}',
        throughputMps: 'Throughput',
        perSecond: '{mps} msg/s',
        signalsChecked: 'Checked with Meta {when}',
        default: 'Default',
        setDefault: 'Set default',
        defaultChanged: 'Default number changed.',
        qualities: {
            unknown: 'Unknown',
            high: 'High',
            medium: 'Medium',
            low: 'Low',
        },
    },
    webhook: {
        title: 'Webhook setup',
        description:
            'Enter these values in the Meta App dashboard under WhatsApp → Configuration. The callback URL must be reachable over public HTTPS.',
        signatureOff: 'Signature verification is off',
        signatureOffHint:
            'No account has an App secret, so every webhook from Meta is rejected: no incoming messages and no delivery updates. Add the App secret to an account (Edit / replace token).',
        callbackUrl: 'Callback URL',
        verifyToken: 'Verify token',
        copyCallbackUrl: 'Copy callback url',
        copyVerifyToken: 'Copy verify token',
        copied: '{label} copied',
        copyFailed: 'Copying is not allowed here; select the text instead.',
        fields: 'Subscribe to these fields',
        moved: 'Your callback URL, verify token, connection test and webhook health are under Settings → WhatsApp / Meta.',
        open: 'Open the Meta connection',
    },
    meta: {
        tab: 'WhatsApp / Meta',
        title: 'WhatsApp / Meta connection',
        description:
            "Connect your company's own Meta app. Messora uses only your credentials: Meta's webhooks reach your own address and are verified with your App secret.",
        method: {
            title: 'Connection method',
            ownApp: 'Your own Meta app',
            ownAppHint:
                'Create a Meta app, add WhatsApp and enter its App ID, App secret and a System User token here. Full control; available now.',
            techProvider: 'Messora Tech Provider',
            soon: 'Coming soon',
            techProviderHint:
                'Connect with Meta in a few clicks, without creating an app. Available once Meta approves Messora as a Tech Provider.',
        },
        steps: {
            title: 'Connect your own Meta app',
            description: 'Follow the steps in order.',
            app: {
                title: 'Create a Meta app',
                body: 'At Meta for Developers → My Apps → Create app, choose the Business type and add the WhatsApp product. Under App settings → Basic you find the App ID and the App secret.',
                link: 'Open Meta for Developers',
            },
            token: {
                title: 'Create a System User token',
                body: 'In Business Settings → System users add an admin system user, assign it your app and your WhatsApp account, and generate a token that never expires with whatsapp_business_messaging and whatsapp_business_management.',
            },
            account: {
                title: 'Enter the credentials',
                body: 'App ID, App secret, the System User token and your WhatsApp Business Account ID. Messora checks them with Meta before saving; secrets are stored encrypted and never shown again.',
                connect: 'Connect account',
                another: 'Connect another account',
                manage: 'Replace credentials and manage numbers under Channels.',
            },
            webhook: {
                title: 'Set the webhook in your app',
                body: 'In your app open WhatsApp → Configuration → Webhook → Edit, paste the callback URL and the verify token below and click "Verify and save". Then subscribe to these fields:',
            },
            test: {
                title: 'Test the connection',
                body: "Messora checks the token, the phone numbers and your app's webhook subscription on the WhatsApp account (and subscribes it when it is missing), then shows whether webhooks arrive.",
            },
        },
        hostUrl: 'Alternative address',
        hostUrlHint:
            "Your workspace's own address accepts the same webhooks, for existing set-ups. Prefer the callback URL above: it does not change with your domain.",
        rotate: {
            open: 'New address',
            title: 'Create a new webhook address?',
            description:
                'Use this if the address leaked. Enter the new callback URL in your Meta app; the old one keeps working for the period you choose.',
            grace: 'The old address keeps working',
            graceNone: 'Stops at once',
            graceDay: 'for 24 hours',
            grace3: 'for 3 days',
            graceWeek: 'for 7 days',
            confirm: 'Create new address',
            done: 'New webhook address created. Enter it in your Meta app.',
            previousUntil: 'The previous address still works until {date}.',
        },
        verify: {
            open: 'New verify token',
            title: 'Replace the verify token?',
            description:
                'Meta asks for it only when the callback URL is verified. The old token stops working at once; enter the new one when you verify again.',
            confirm: 'Replace',
            done: 'Verify token replaced.',
        },
        health: {
            title: 'Connection health',
            description: 'What Messora sees of the webhooks your app sends.',
            lastWebhook: 'Last webhook',
            never: 'None yet',
            signatureFailures: 'Rejected signatures',
            lastAt: 'last {date}',
            mismatches: 'Events for accounts not connected',
            noSecret:
                'No account has an App secret yet, so every webhook from Meta is rejected.',
            signatureHint:
                'Recent webhooks had a wrong signature. Check that the App secret belongs to the app whose webhook points here.',
            mismatchHint:
                'Your app sent events for WhatsApp account {waba}, which is not connected here; they were dropped. Connect that account or unsubscribe it in your app.',
            tokens: 'Access tokens',
        },
        test: {
            button: 'Test connection',
            ok: 'The connection works.',
            failed: 'The test found a problem.',
            none: 'Connect an account first; the test runs per account.',
            keys: {
                app_secret: 'App credentials',
                token: 'Access token',
                phone_numbers: 'Phone numbers',
                subscription: 'Webhook subscription',
                webhook: 'Webhooks received',
            },
            codes: {
                app_secret_missing:
                    "No App secret is stored, so this account's webhooks are rejected. Add it with Edit / replace token.",
                app_id_missing:
                    'No App ID is stored; the token cannot be checked with your app and media templates cannot be created.',
                app_ok: 'App ID and App secret are stored.',
                token_invalid: 'Meta refused the token: {detail}',
                token_unchecked:
                    "Meta did not report the token's expiry. Check that App ID and App secret belong to the same app, or use a System User token.",
                token_other_app:
                    'The token belongs to app {token_app}, not to App ID {app_id}. Use a System User token of the same app.',
                token_expires:
                    'The token expires on {expires_at}. Replace it with a System User token that never expires.',
                token_ok: 'Valid and never expires.',
                numbers_error:
                    'Meta refused to list the phone numbers: {detail}',
                numbers_none:
                    'The WhatsApp account has no phone number yet; add one in WhatsApp Manager.',
                numbers_ok: '{count} phone number(s) found.',
                subscription_error:
                    'The webhook subscription could not be read or created: {detail}',
                subscribed_now:
                    "Your app was not subscribed to the account's webhooks; Messora subscribed it now.",
                subscribed: "Your app is subscribed to the account's webhooks.",
                not_subscribed:
                    "Your app is still not subscribed to the account's webhooks.",
                webhook_none:
                    'No webhook has arrived yet. Check the callback URL, the verify token and the subscribed fields in your app; the first event comes with the next message or status.',
                webhook_unknown: 'The webhook health could not be read.',
                webhook_signature:
                    'Webhooks arrive, but the latest one ({at}) had a wrong signature. Check the App secret.',
                webhook_ok: 'The last webhook arrived {at}.',
            },
        },
    },
};
export default channels;
