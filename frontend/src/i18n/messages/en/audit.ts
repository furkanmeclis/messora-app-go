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
    empty: 'No entries.',
    system: 'System',
    unknown: 'Unknown',
};
export default audit;
