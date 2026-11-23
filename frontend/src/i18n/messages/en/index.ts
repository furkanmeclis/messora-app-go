import type { DeepString } from '../types';
import appearance from './appearance';
import auth from './auth';
import overview from './overview';
import inbox from './inbox';
import contacts from './contacts';
import automations from './automations';
import common from './common';
import errors from './errors';
import nav from './nav';
import settings from './settings';
import wizard from './wizard';
import topbar from './topbar';
import shortcuts from './shortcuts';
import table from './table';
import desktop from './desktop';
import account from './account';
import apiKeys from './api-keys';
import audit from './audit';
import campaigns from './campaigns';
import channels from './channels';
import templates from './templates';
import users from './users';
import webhooks from './webhooks';

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
    topbar,
    shortcuts,
    table,
    desktop,
    account,
    apiKeys,
    audit,
    campaigns,
    channels,
    templates,
    users,
    webhooks,
    auth,
    overview,
    inbox,
    contacts,
    automations,
};

export type Messages = DeepString<typeof en>;
