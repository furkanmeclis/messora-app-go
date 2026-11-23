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
            'Sending fails fast until a successful sync or new credentials.',
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
        appId: 'App ID (optional)',
        appSecret: 'App secret (recommended)',
        appSecretHint:
            'Needed to verify webhook signatures. Without it every webhook from Meta is rejected.',
        submit: 'Connect',
        done: 'Account connected. Phone numbers and templates were imported.',
    },
    edit: {
        open: 'Edit / rotate',
        title: 'Edit {name}',
        description:
            'Leave a secret empty to keep the current one. A new token is checked with Meta first.',
        name: 'Name',
        accessToken: 'New access token',
        appSecret: 'New app secret',
        secretConfigured: 'An app secret is configured.',
        noSecret: 'No app secret yet: webhooks are rejected until you add one.',
        disabled: 'Account disabled.',
        enabled: 'Account enabled.',
        credentialsUpdated: 'Credentials updated.',
        updated: 'Account updated.',
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
            'No app secret is configured, so every webhook from Meta is rejected: no incoming messages and no delivery updates. Add the app secret to an account (Edit / rotate) or set META_APP_SECRET.',
        callbackUrl: 'Callback URL',
        verifyToken: 'Verify token',
        copyCallbackUrl: 'Copy callback url',
        copyVerifyToken: 'Copy verify token',
        copied: '{label} copied',
        copyFailed: 'Copying is not allowed here; select the text instead.',
        fields: 'Subscribe to these fields',
    },
};
export default channels;
