import {
    Ban,
    CalendarClock,
    CircleCheck,
    CircleDashed,
    CircleX,
    Clock,
    FilePen,
    Layers,
    Loader,
    PauseCircle,
    Send,
    TriangleAlert,
    type LucideIcon,
} from 'lucide-react';

/** Icons for campaign states. */
export const CAMPAIGN_STATUS_ICON: Record<string, LucideIcon> = {
    all: Layers,
    draft: FilePen,
    scheduled: CalendarClock,
    preparing: Loader,
    running: Send,
    paused: PauseCircle,
    completed: CircleCheck,
    cancelled: Ban,
    failed: TriangleAlert,
};

/** Icons for template review states. */
export const TEMPLATE_STATUS_ICON: Record<string, LucideIcon> = {
    all: Layers,
    APPROVED: CircleCheck,
    PENDING: Clock,
    REJECTED: CircleX,
    PAUSED: PauseCircle,
    DISABLED: Ban,
};

/** Colour of each state's icon. */
export const STATUS_TONE: Record<string, string> = {
    running: 'text-primary',
    completed: 'text-success',
    APPROVED: 'text-success',
    scheduled: 'text-chart-2',
    preparing: 'text-chart-2',
    PENDING: 'text-warning',
    paused: 'text-warning',
    PAUSED: 'text-warning',
    failed: 'text-destructive',
    REJECTED: 'text-destructive',
};

export const FallbackStatusIcon = CircleDashed;
