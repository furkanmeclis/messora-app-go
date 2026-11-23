'use client';

import { useTransition } from 'react';

/**
 * Submits a form to a useActionState dispatcher without React's automatic
 * form reset, so a rejected submission keeps what the user typed.
 */
export function useSubmit(dispatch: (form: FormData) => void) {
    const [, start] = useTransition();
    return (e: React.FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        const form = new FormData(e.currentTarget);
        start(() => dispatch(form));
    };
}
