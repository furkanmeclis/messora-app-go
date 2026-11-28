const apiKeys = {
    title: 'API keys',
    description:
        'Credentials for integrations. Use them as `Authorization: Bearer msk_live_…`.',
    empty: 'No API keys yet',
    emptyHint: 'Create a key to connect another system to Messora.',
    columns: {
        name: 'Name',
        scopes: 'Scopes',
        lastUsed: 'Last used',
        expires: 'Expires',
    },
    base: {
        title: 'Your API address',
        description:
            'Every workspace has its own API address. Keys of this workspace work only here; at any other address they answer 401.',
        copy: 'Copy base URL',
        copied: 'Base URL copied',
        sample: 'Try a key',
    },
    never: 'Never',
    revoke: 'Revoke',
    revoked: 'API key revoked.',
    revokedMany: '{count} API key revoked.|{count} API keys revoked.',
    revokeManyTitle: 'Revoke {count} API key?|Revoke {count} API keys?',
    create: {
        button: 'Create key',
        title: 'Create an API key',
        description: 'Grant only the scopes the integration needs.',
        copyNow:
            'Copy the key now. For your security it will not be shown again.',
        copyKey: 'Copy key',
        keyCopied: 'Key copied',
        done: 'Done',
        name: 'Name',
        namePlaceholder: 'CRM sync',
        expiresAfter: 'Expires after (days)',
        expiresHint: 'Leave empty for a key that does not expire.',
        scopes: 'Scopes',
    },
};
export default apiKeys;
