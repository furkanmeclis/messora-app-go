const audit = {
    title: 'Audit log',
    description:
        'Every security-relevant action, newest first. Entries cannot be changed or deleted.',
    filteredBy: 'Filtered by',
    clearFilter: 'Clear filter',
    filterBy: 'Show only {action}',
    columns: {
        when: 'When',
        who: 'Who',
        action: 'Action',
        details: 'Details',
    },
    support: {
        title: 'Messora support access',
        description:
            'Times Messora support opened this workspace, always for a limited time and with a reason.',
        when: 'When',
        who: 'Who',
        reason: 'Reason',
        access: 'Access',
        active: 'Now',
    },
    empty: 'No entries.',
    system: 'System',
    unknown: 'Unknown',
};
export default audit;
