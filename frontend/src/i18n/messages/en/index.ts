import type { DeepString } from '../types';
import appearance from './appearance';
import auth from './auth';
import overview from './overview';
import inbox from './inbox';
import common from './common';
import errors from './errors';
import nav from './nav';
import settings from './settings';
import wizard from './wizard';

/**
 * The English dictionary defines the shape every language must match.
 * Add a namespace file here and in ../tr, each typed Messages['name'].
 */
export const en = {
    common,
    nav,
    errors,
    appearance,
    settings,
    wizard,
    auth,
    overview,
    inbox,
};

export type Messages = DeepString<typeof en>;
