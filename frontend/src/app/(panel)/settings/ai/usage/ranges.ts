/** Preset periods of the AI usage page. */
export const RANGES = ['7', '30', '90', 'month'] as const;
export type Range = (typeof RANGES)[number];
