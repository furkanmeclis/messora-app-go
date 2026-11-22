import { createHmac } from 'node:crypto';

/** RFC 4648 base32 decode (no padding required). */
function base32Decode(input: string): Buffer {
    const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
    const clean = input.toUpperCase().replace(/=+$/, '').replace(/\s/g, '');
    let bits = 0;
    let value = 0;
    const out: number[] = [];
    for (const ch of clean) {
        const idx = alphabet.indexOf(ch);
        if (idx < 0) throw new Error(`invalid base32 character ${ch}`);
        value = (value << 5) | idx;
        bits += 5;
        if (bits >= 8) {
            out.push((value >>> (bits - 8)) & 0xff);
            bits -= 8;
        }
    }
    return Buffer.from(out);
}

/** RFC 6238 TOTP (SHA-1, 6 digits, 30 s), as used by authenticator apps. */
export function totp(secret: string, at = Date.now()): string {
    const counter = Math.floor(at / 1000 / 30);
    const msg = Buffer.alloc(8);
    msg.writeBigUInt64BE(BigInt(counter));
    const mac = createHmac('sha1', base32Decode(secret)).update(msg).digest();
    const offset = mac[mac.length - 1]! & 0x0f;
    const code = (mac.readUInt32BE(offset) & 0x7fffffff) % 1_000_000;
    return code.toString().padStart(6, '0');
}
