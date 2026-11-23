const settings = {
    title: 'Settings',
    description: 'Workspace-wide configuration.',
    tabs: {
        general: 'General',
        appearance: 'Appearance',
        email: 'E-mail',
        security: 'Security',
        webhooks: 'Webhooks',
    },
    general: {
        organization: 'Organisation',
        organizationName: 'Organisation name',
        organizationHint:
            'Shown in the panel, in e-mails and on the sign-in page.',
        regional: 'Language and region',
        regionalHint:
            'Defaults for new users; everyone can choose their own in their profile.',
        defaultLocale: 'Default language',
        defaultTimezone: 'Default time zone',
        dateFormat: 'Date format',
        timeFormat: 'Time format',
        timeFormats: { '24h': '24-hour (14:30)', '12h': '12-hour (2:30 PM)' },
        example: 'Example: {value}',
        saved: 'Settings saved',
    },
    security: {
        twoFactor: 'Two-factor authentication',
        twoFactorHint:
            'Roles selected here must set up an authenticator app before they can use the panel. Nobody is required by default.',
        requiredFor: 'Required for',
        noneRequired: 'Not required for any role',
        sessions: 'Sessions',
        idleHours: 'Sign out after inactivity (hours)',
        lifetimeDays: 'Maximum session length (days)',
        roles: {
            owner: 'Owner',
            admin: 'Admin',
            manager: 'Manager',
            agent: 'Agent',
        },
    },
    email: {
        title: 'Outgoing e-mail',
        description:
            'Used for invitations, password resets and automation e-mails. Without it, invitation links are shown in the panel to copy.',
        managedByEnv:
            'E-mail is configured with SMTP_* environment variables on the server. Remove SMTP_HOST to manage it here.',
        enabled: 'E-mail is on',
        disabled: 'E-mail is off',
        host: 'SMTP server',
        hostPlaceholder: 'smtp.example.com',
        port: 'Port',
        portHint: 'Leave empty for the default of the security mode.',
        security: 'Security',
        tls: {
            starttls: 'STARTTLS (port 587)',
            tls: 'TLS (port 465)',
            none: 'None (local test servers only)',
        },
        username: 'Username',
        password: 'Password',
        passwordKeep: 'Stored — leave empty to keep it',
        from: 'Sender',
        fromPlaceholder: 'Acme <no-reply@acme.com>',
        test: 'Send a test e-mail',
        testTo: 'Send the test to',
        testSent: 'Test e-mail sent to {to}',
        turnOff: 'Turn e-mail off',
        saved: 'E-mail settings saved',
    },
};
export default settings;
