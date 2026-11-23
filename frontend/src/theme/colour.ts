/** sRGB colour with channels 0–1 and alpha 0–1. */
export type RGBA = { r: number; g: number; b: number; a: number };

const clamp = (v: number) => Math.min(1, Math.max(0, v));

function fromLinear(c: number) {
    return c <= 0.0031308 ? 12.92 * c : 1.055 * Math.pow(c, 1 / 2.4) - 0.055;
}
function toLinear(c: number) {
    return c <= 0.04045 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
}

export function oklchToRgb(l: number, c: number, h: number, a = 1): RGBA {
    const hr = (h * Math.PI) / 180;
    const A = c * Math.cos(hr);
    const B = c * Math.sin(hr);
    const l_ = l + 0.3963377774 * A + 0.2158037573 * B;
    const m_ = l - 0.1055613458 * A - 0.0638541728 * B;
    const s_ = l - 0.0894841775 * A - 1.291485548 * B;
    const L = l_ ** 3;
    const M = m_ ** 3;
    const S = s_ ** 3;
    return {
        r: clamp(
            fromLinear(4.0767416621 * L - 3.3077115913 * M + 0.2309699292 * S),
        ),
        g: clamp(
            fromLinear(-1.2684380046 * L + 2.6097574011 * M - 0.3413193965 * S),
        ),
        b: clamp(
            fromLinear(-0.0041960863 * L - 0.7034186147 * M + 1.707614701 * S),
        ),
        a,
    };
}

export function rgbToOklch({ r, g, b }: RGBA): [number, number, number] {
    const R = toLinear(r);
    const G = toLinear(g);
    const B = toLinear(b);
    const l = Math.cbrt(0.4122214708 * R + 0.5363325363 * G + 0.0514459929 * B);
    const m = Math.cbrt(0.2119034982 * R + 0.6806995451 * G + 0.1073969566 * B);
    const s = Math.cbrt(0.0883024619 * R + 0.2817188376 * G + 0.6299787005 * B);
    const L = 0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s;
    const a = 1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s;
    const bb = 0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s;
    const C = Math.sqrt(a * a + bb * bb);
    let H = (Math.atan2(bb, a) * 180) / Math.PI;
    if (H < 0) H += 360;
    return [L, C, H];
}

/** Parses #hex and oklch() (the two forms themes allow). */
export function parseColour(v: string): RGBA | null {
    const s = v.trim();
    const hex = /^#([0-9a-f]{3}|[0-9a-f]{6}|[0-9a-f]{8})$/i.exec(s);
    if (hex) {
        let h = hex[1]!;
        if (h.length === 3) h = [...h].map((c) => c + c).join('');
        const n = (i: number) => parseInt(h.slice(i, i + 2), 16) / 255;
        return { r: n(0), g: n(2), b: n(4), a: h.length === 8 ? n(6) : 1 };
    }
    const m =
        /^oklch\(\s*([\d.]+)(%?)\s+([\d.]+)\s+([\d.]+)(?:\s*\/\s*([\d.]+)(%?))?\s*\)$/i.exec(
            s,
        );
    if (m) {
        const l = Number(m[1]) / (m[2] ? 100 : 1);
        const a = m[5] === undefined ? 1 : Number(m[5]) / (m[6] ? 100 : 1);
        return oklchToRgb(l, Number(m[3]), Number(m[4]), a);
    }
    return null;
}

export function toHex({ r, g, b, a }: RGBA): string {
    const h = (n: number) =>
        Math.round(clamp(n) * 255)
            .toString(16)
            .padStart(2, '0');
    return `#${h(r)}${h(g)}${h(b)}${a < 1 ? h(a) : ''}`;
}

/** Composites a translucent colour over a background. */
function over(c: RGBA, bg: RGBA): RGBA {
    return {
        r: c.r * c.a + bg.r * (1 - c.a),
        g: c.g * c.a + bg.g * (1 - c.a),
        b: c.b * c.a + bg.b * (1 - c.a),
        a: 1,
    };
}

function luminance({ r, g, b }: RGBA) {
    return 0.2126 * toLinear(r) + 0.7152 * toLinear(g) + 0.0722 * toLinear(b);
}

/** WCAG contrast ratio of text on a surface (1–21). */
export function contrast(text: string, surface: string): number | null {
    const t = parseColour(text);
    const s = parseColour(surface);
    if (!t || !s) return null;
    const bg = over(s, { r: 1, g: 1, b: 1, a: 1 });
    const fg = over(t, bg);
    const [hi, lo] = [luminance(fg), luminance(bg)].sort((x, y) => y - x) as [
        number,
        number,
    ];
    return (hi + 0.05) / (lo + 0.05);
}

// HSV, for the picker's saturation/value square.
export type HSV = { h: number; s: number; v: number };

export function rgbToHsv({ r, g, b }: RGBA): HSV {
    const max = Math.max(r, g, b);
    const min = Math.min(r, g, b);
    const d = max - min;
    let h = 0;
    if (d) {
        if (max === r) h = ((g - b) / d) % 6;
        else if (max === g) h = (b - r) / d + 2;
        else h = (r - g) / d + 4;
        h *= 60;
        if (h < 0) h += 360;
    }
    return { h, s: max ? d / max : 0, v: max };
}

export function hsvToRgb({ h, s, v }: HSV, a = 1): RGBA {
    const c = v * s;
    const x = c * (1 - Math.abs(((h / 60) % 2) - 1));
    const m = v - c;
    const [r, g, b] =
        h < 60
            ? [c, x, 0]
            : h < 120
              ? [x, c, 0]
              : h < 180
                ? [0, c, x]
                : h < 240
                  ? [0, x, c]
                  : h < 300
                    ? [x, 0, c]
                    : [c, 0, x];
    return { r: r + m, g: g + m, b: b + m, a };
}
