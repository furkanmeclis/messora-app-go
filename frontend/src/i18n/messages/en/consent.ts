const consent = {
    statuses: {
        opted_in: 'Opted in',
        unknown: 'Unknown',
        opt_out_requested: 'Opt-out requested',
        opted_out: 'Opted out',
        resubscribed: 'Resubscribed',
    },
    categories: {
        marketing: 'Marketing',
        service: 'Service',
        all: 'Both',
    },
    categoryHints: {
        marketing: 'Campaigns, offers and other marketing templates.',
        service:
            'Utility and authentication templates, such as order updates and codes.',
        all: 'Marketing and service at once.',
    },
    flags: {
        metaStopped: 'Stopped in WhatsApp',
        metaStoppedHint:
            'The customer chose “Stop” for your marketing messages in WhatsApp. Marketing stays blocked until they choose “Resume” there.',
        undeliverable: 'Undeliverable',
        undeliverableHint:
            'Meta could not deliver to this number. Nothing is sent until the customer writes again.',
        undeliverableCode: 'Meta error {code}',
    },
    reasons: {
        opted_out: 'Opted out',
        meta_marketing_stopped: 'Stopped marketing in WhatsApp',
        undeliverable: 'Undeliverable number',
        opt_out_requested: 'Asked to stop',
        consent_unknown: 'No recorded consent',
        replied_after_opt_out: 'Wrote again after opting out',
    },
    reasonHelp: {
        opted_out: 'They opted out of this kind of message.',
        meta_marketing_stopped:
            'They turned off your marketing messages in WhatsApp.',
        undeliverable:
            'Meta could not deliver to the number; it is reachable again once the customer writes.',
        opt_out_requested:
            'They asked to stop. Marketing waits until an agent confirms the opt-out or clears the request.',
        consent_unknown: 'There is no record that they agreed to marketing.',
        replied_after_opt_out:
            'They opted out but wrote again, so a reply in that conversation is allowed.',
    },
    events: {
        opt_in: 'Opted in',
        opt_out: 'Opted out',
        opt_out_requested: 'Asked to stop',
        resubscribed: 'Resubscribed',
        unknown: 'Set to unknown',
        meta_marketing_stopped: 'Stopped marketing in WhatsApp',
        meta_marketing_resumed: 'Resumed marketing in WhatsApp',
        undeliverable: 'Marked undeliverable by Meta',
        deliverable: 'Reachable again',
    },
    sources: {
        manual: 'by hand',
        bulk: 'bulk action',
        api: 'API',
        import: 'import',
        keyword: 'keyword in a message',
        meta_user_preferences: 'WhatsApp setting',
        meta_error: 'Meta error',
        automation: 'automation',
        inbound: 'customer message',
    },
    card: {
        title: 'Subscription & consent',
        description:
            'Marketing templates need marketing consent; utility and authentication templates need service consent.',
        change: 'Change consent',
        confirmOptOut: 'Confirm opt-out',
        clearRequest: 'Keep subscribed',
        requestPending:
            'They asked to stop receiving messages. Marketing is paused until you confirm the opt-out or clear the request.',
        updated: 'updated {time}',
        reason: 'Reason: {reason}',
    },
    dialog: {
        title: 'Change consent',
        description:
            'Each change is kept in the consent history with your name and reason.',
        category: 'Category',
        status: 'New status',
        reason: 'Reason',
        reasonPlaceholder: 'e.g. Asked by phone to stop offers',
        reasonRequired: 'Write why the contact opted out.',
        hardNote:
            'A customer who stopped marketing in WhatsApp, or a number Meta cannot deliver to, stays blocked whatever is chosen here.',
        save: 'Save',
        saved: 'Consent updated.',
    },
    history: {
        title: 'Consent history',
        empty: 'No consent changes yet.',
        loadFailed: 'The consent history could not be loaded.',
        by: 'by {name}',
        automatic: 'automatically',
        via: 'via {source}',
        both: 'both',
    },
    filters: {
        marketing: 'Marketing consent',
        service: 'Service consent',
        reachability: 'Reachability',
    },
    preflight: {
        title: 'Who receives this message',
        checking: 'Checking consent for the audience…',
        failed: 'Consent could not be checked: {message}',
        audience: 'Your audience has {n} person|Your audience has {n} people',
        refused: '; {n} said they don’t want messages.',
        blockedOnly: '; {n} can’t receive this message.',
        allClear: '; all of them can receive this message.',
        eligible: '{n} will receive it',
        eligibleWarned: '{n} will receive it, {warned} of them with a warning',
        blocked: 'Not sent ({n})',
        warned: 'Sent with a warning ({n})',
        hard: 'Always blocked',
        category: {
            marketing:
                'This is a marketing template: it needs marketing consent.',
            utility:
                'This is a utility template: it needs service consent, not marketing consent.',
            authentication:
                'This is an authentication template: it needs service consent, not marketing consent.',
        },
        hardNote:
            'Opted-out contacts, customers who stopped your marketing in WhatsApp and undeliverable numbers can never be sent to: no setting, API call or automation overrides this.',
        unknownPolicy: {
            allow: 'Your company sends marketing to contacts without recorded consent, with a warning.',
            block: 'Your company does not send marketing to contacts without recorded consent.',
        },
        changePolicy: 'Change in consent settings',
        noneEligible: 'Nobody in this audience can receive this template.',
        confirmEligible:
            '{n} contact receives “{template}” right away. |{n} contacts receive “{template}” right away. ',
    },
    settings: {
        tab: 'Subscription & consent',
        title: 'Subscription & consent',
        description:
            'Who may receive which messages, and how customers can stop or restart them by writing to you.',
        hardTitle: 'Always blocked',
        hardBody:
            'Contacts who opted out of a category, customers who stopped your marketing in WhatsApp and numbers Meta could not deliver to never receive those messages. Settings, the API and automations cannot override this.',
        unknownTitle: 'Marketing without recorded consent',
        unknownDescription:
            'Contacts added by hand, imported or who wrote first often have no recorded marketing consent.',
        unknown: {
            allow: 'Send with a warning',
            allowHint:
                'Recommended to start with. The campaign review shows how many have no recorded consent.',
            block: 'Don’t send',
            blockHint:
                'Only contacts who opted in or resubscribed receive marketing.',
        },
        requestNote:
            'An opt-out request (a phrase like “please stop”) pauses marketing to that contact until an agent confirms the opt-out or clears the request.',
        keywordsTitle: 'Keywords',
        keywordsDescription:
            'Messages from customers are checked against these lists. Upper and lower case and accents are ignored; up to 50 entries per list.',
        stop: 'Stop words',
        stopHint:
            'A message that is exactly one of these opts the contact out of marketing at once.',
        request: 'Opt-out request phrases',
        requestHint:
            'A message containing one of these is flagged for an agent to confirm; marketing pauses meanwhile.',
        resume: 'Resubscribe words',
        resumeHint:
            'A message that is exactly one of these subscribes the contact to marketing again.',
        confirmTitle: 'Confirmation replies',
        confirm: 'Reply with a confirmation',
        confirmHint:
            'After a stop or resubscribe word, the contact gets this reply in their language.',
        stopMessage: 'After a stop word',
        resumeMessage: 'After a resubscribe word',
        tr: 'Turkish',
        en: 'English',
        saved: 'Consent settings saved.',
    },
    notification: {
        optOutRequested: 'A customer asked to stop receiving messages.',
        open: 'Open conversation',
    },
    failureClasses: {
        rejected_by_meta: 'Rejected by Meta',
        temporary: 'Temporary error',
        permanent: 'Permanent error',
        user_stopped_marketing: 'Customer stopped marketing',
        undeliverable: 'Undeliverable',
        ecosystem_limit: 'Held back by Meta',
        blocked_by_consent: 'Blocked by consent',
        cancelled: 'Cancelled',
    },
};

export default consent;
