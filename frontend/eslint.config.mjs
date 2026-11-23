import nextVitals from 'eslint-config-next/core-web-vitals';
import nextTs from 'eslint-config-next/typescript';

/**
 * Messora draws its own controls; the browser's native widgets look
 * different on every platform. Use the components in src/components/ui.
 */
const nativeControls = [
    {
        selector: "JSXOpeningElement[name.name='select']",
        message: 'Use <Select> or <Combobox> from components/ui.',
    },
    {
        selector:
            "JSXOpeningElement[name.name=/^(input|Input|TextField)$/] > JSXAttribute[name.name='type'][value.value=/^(date|time|datetime-local|month|week|color|checkbox|radio|range|number|file)$/]",
        message:
            'Use DatePicker, DateTimePicker, ColorPicker, Checkbox, RadioCards, Segmented, Slider, NumberField or FileDrop from components/ui.',
    },
    {
        selector:
            "CallExpression[callee.name=/^(confirm|alert|prompt)$/], CallExpression[callee.object.name='window'][callee.property.name=/^(confirm|alert|prompt)$/]",
        message: 'Use useConfirm() or a toast instead of browser dialogs.',
    },
];

const config = [
    ...nextVitals,
    ...nextTs,
    {
        files: ['src/**/*.{ts,tsx}'],
        rules: { 'no-restricted-syntax': ['error', ...nativeControls] },
    },
    { ignores: ['.next/**', 'node_modules/**', 'next-env.d.ts'] },
];

export default config;
