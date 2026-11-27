/**
 * A quick, local estimate of how guessable a new password is, for the meter
 * under "new password" fields. The API has the final say (length, common
 * password list); this only nudges people before they submit.
 */

export const PASSWORD_MIN = 12;

export type StrengthScore = 0 | 1 | 2 | 3 | 4;
export type StrengthLabel = 'empty' | 'weak' | 'fair' | 'good' | 'strong';

export type Strength = {
    score: StrengthScore;
    label: StrengthLabel;
    /** Shorter than the API's minimum; the form will not submit. */
    tooShort: boolean;
};

const LABELS: StrengthLabel[] = ['empty', 'weak', 'fair', 'good', 'strong'];

/** Words that make a password easy to guess wherever they appear. */
const COMMON = [
    'password',
    'parola',
    'sifre',
    'qwerty',
    'asdf',
    'letmein',
    'welcome',
    'hosgeldin',
    'admin',
    'iloveyou',
    'monkey',
    'dragon',
    'master',
    'sunshine',
    'football',
    'galatasaray',
    'fenerbahce',
    'besiktas',
    'messora',
    'whatsapp',
    'abc123',
    '123456',
    '111111',
    '000000',
];

const SEQUENCES = [
    'abcdefghijklmnopqrstuvwxyz',
    '01234567890',
    'qwertyuiop',
    'asdfghjkl',
    'zxcvbnm',
];

function hasSequence(lower: string, run = 4): boolean {
    for (const seq of SEQUENCES) {
        for (const s of [seq, [...seq].reverse().join('')]) {
            for (let i = 0; i + run <= s.length; i++) {
                if (lower.includes(s.slice(i, i + run))) return true;
            }
        }
    }
    return false;
}

/**
 * Scores a password from 1 (weak) to 4 (strong); 0 means empty. Personal
 * words (name, e-mail, company) count as common words.
 */
export function passwordStrength(
    password: string,
    personal: readonly string[] = [],
): Strength {
    if (!password) return { score: 0, label: 'empty', tooShort: true };
    const lower = password.toLocaleLowerCase('tr');
    const length = [...password].length;
    const tooShort = length < PASSWORD_MIN;

    let score = length >= 20 ? 3 : length >= 16 ? 2.5 : length >= 12 ? 2 : 1;
    const classes = [/[a-z]/, /[A-Z]/, /\d/, /[^A-Za-z0-9]/].filter((r) =>
        r.test(password),
    ).length;
    score += classes >= 4 ? 1.5 : classes === 3 ? 1 : classes === 2 ? 0.5 : 0;
    if (new Set(lower).size <= Math.max(3, length / 4)) score -= 1.5;
    if (/(.)\1\1/.test(lower)) score -= 0.5;
    if (hasSequence(lower)) score -= 1;

    const words = personal
        .flatMap((p) => p.toLocaleLowerCase('tr').split(/[^\p{L}\p{N}]+/u))
        .filter((w) => w.length >= 3);
    if ([...COMMON, ...words].some((w) => lower.includes(w))) {
        score = Math.min(score, 1.5);
    }

    let final = Math.max(1, Math.min(4, Math.floor(score))) as StrengthScore;
    if (tooShort) final = 1;
    return { score: final, label: LABELS[final]!, tooShort };
}
