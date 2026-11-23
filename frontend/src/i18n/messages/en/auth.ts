const auth = {
    email: 'E-mail',
    password: 'Password',
    newPassword: 'New password',
    yourName: 'Your name',
    passwordHint:
        'At least 12 characters. Avoid common passwords and your e-mail address.',
    login: {
        title: 'Sign in',
        description: 'Welcome back to Messora.',
        submit: 'Sign in',
        forgot: 'Forgot your password?',
        passwordChanged: 'Your password was changed. Sign in with the new one.',
        expired: 'The sign-in attempt expired. Please sign in again.',
    },
    verify: {
        title: 'Two-factor authentication',
        description: 'Enter the 6-digit code from your authenticator app.',
        recoveryCode: 'Recovery code',
        authenticatorCode: 'Authenticator code',
        submit: 'Verify',
        useApp: 'Use the authenticator app instead',
        useRecovery: 'Use a recovery code instead',
    },
    setup: {
        title: 'Set up Messora',
        description: 'Create the owner account for this installation.',
        organizationName: 'Organization name',
        organizationPlaceholder: 'Acme Inc.',
        submit: 'Create owner account',
        openLinkTitle: 'Open the setup link',
        openLinkLogs:
            'The one-time link is printed in the API logs on first start:',
        newLink: 'For a new link run:',
    },
    invite: {
        metaTitle: 'Accept invitation',
        title: 'Join Messora',
        invitedAs: 'You were invited as {role}.',
        accept: 'Accept your invitation.',
        incomplete: 'This invitation link is incomplete.',
        askAdmin: 'Ask an administrator for a new invitation.',
        submit: 'Join',
    },
    forgot: {
        metaTitle: 'Reset password',
        title: 'Reset your password',
        description: 'We will e-mail you a link to choose a new password.',
        submit: 'Send reset link',
        noEmail:
            'E-mail is not configured on this installation. Ask an owner or administrator to reset your access.',
        backToSignIn: 'Back to sign in',
        sent: 'If an account exists for this address, we sent a link to reset the password. Check your inbox.',
    },
    reset: {
        title: 'Choose a new password',
        description: 'You will be signed out on every device.',
        submit: 'Set new password',
    },
};
export default auth;
