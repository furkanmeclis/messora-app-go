/**
 * State changes for the six-box one-time code input, kept free of React so
 * they can be tested: typing, pasting a whole code, and backspace.
 */

export const OTP_LENGTH = 6;

export type OtpChange = { digits: string[]; focus: number };

export function emptyOtp(length = OTP_LENGTH): string[] {
    return Array.from({ length }, () => '');
}

/**
 * Applies what arrived in box `index`. One digit fills the box and moves
 * on; several (a paste, or the phone's one-time-code autofill) spread over
 * the following boxes. Anything that is not a digit is ignored.
 */
export function applyOtpInput(
    digits: readonly string[],
    index: number,
    raw: string,
): OtpChange {
    const next = [...digits];
    const incoming = raw.replace(/\D/g, '');
    if (!incoming) return { digits: next, focus: index };
    // Typing over a filled box: the browser hands over old + new digit.
    const chars =
        incoming.length === 2 && next[index] && incoming[0] === next[index]
            ? [incoming[1]!]
            : [...incoming];
    let i = index;
    for (const c of chars) {
        if (i >= next.length) break;
        next[i] = c;
        i++;
    }
    return { digits: next, focus: Math.min(i, next.length - 1) };
}

/** A pasted code; a complete one always starts at the first box. */
export function applyOtpPaste(
    digits: readonly string[],
    index: number,
    text: string,
): OtpChange {
    const incoming = text.replace(/\D/g, '');
    const start = incoming.length >= digits.length ? 0 : index;
    return applyOtpInput(digits, start, incoming);
}

/** Backspace empties the box, or the one before it when already empty. */
export function applyOtpBackspace(
    digits: readonly string[],
    index: number,
): OtpChange {
    const next = [...digits];
    if (next[index]) {
        next[index] = '';
        return { digits: next, focus: index };
    }
    const prev = Math.max(0, index - 1);
    next[prev] = '';
    return { digits: next, focus: prev };
}

export function otpComplete(digits: readonly string[]): boolean {
    return digits.every((d) => /^\d$/.test(d));
}
