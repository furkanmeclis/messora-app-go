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
import contextMenu from './contextMenu';
import pwa from './pwa';
import license from './license';
import params from './params';
import insights from './insights';
import account from './account';
import apiKeys from './api-keys';
import audit from './audit';
import failedJobs from './failed-jobs';
import campaigns from './campaigns';
import channels from './channels';
import templates from './templates';
import users from './users';
import webhooks from './webhooks';
import consent from './consent';
import offers from './offers';

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
    contextMenu,
    pwa,
    license,
    params,
    insights,
    account,
    apiKeys,
    audit,
    failedJobs,
    campaigns,
    channels,
    templates,
    users,
    webhooks,
    consent,
    offers,
    auth,
    overview,
    inbox,
    contacts,
    automations,
};

export type Messages = DeepString<typeof en>;
