const failedJobs = {
    title: 'Failed jobs',
    description:
        'Background jobs that kept failing until their retries ran out. Retry one once the cause is fixed, or delete it.',
    showing: 'Showing the newest {shown} of {total}.',
    allQueues: 'All queues',
    queues: {
        critical: 'Critical',
        campaigns: 'Campaigns',
        ai: 'AI',
        default: 'Default',
        bulk: 'Bulk',
    },
    columns: {
        type: 'Job',
        queue: 'Queue',
        error: 'Last error',
        failedAt: 'Failed at',
        attempts: 'Attempts',
        refs: 'References',
    },
    attempts: '{retried} of {max} retries',
    retry: 'Retry',
    delete: 'Delete',
    retryTitle: 'Retry this job?',
    retryBody:
        'It runs once more now. If it fails again it comes straight back to this list.',
    deleteTitle: 'Delete this job?',
    deleteBody:
        'The job is discarded and will not run again. Records it was working on keep their current state.',
    retried: 'Job queued again',
    deleted: 'Job deleted',
    retriedMany: '{count} job queued again|{count} jobs queued again',
    deletedMany: '{count} job deleted|{count} jobs deleted',
    retryManyTitle: 'Retry {count} job?|Retry {count} jobs?',
    deleteManyTitle: 'Delete {count} job?|Delete {count} jobs?',
    empty: 'No failed jobs',
    emptyHint:
        'Every background job either finished or is still being retried.',
};
export default failedJobs;
