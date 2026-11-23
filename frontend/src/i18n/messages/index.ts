import type { Locale } from '../config';
import { en, type Messages } from './en';
import { tr } from './tr';

export type { Messages };

export const dictionaries: Record<Locale, Messages> = { en, tr };
