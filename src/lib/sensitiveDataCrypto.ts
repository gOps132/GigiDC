import { createCipheriv, createDecipheriv, randomBytes } from 'node:crypto';

const KEY_BYTES = 32;
const NONCE_BYTES = 12;
const AUTH_TAG_BYTES = 16;
const ENCRYPTION_ALGORITHM = 'aes-256-gcm';

export interface EncryptedSensitiveValue {
  ciphertext: string;
  nonce: string;
}

export function parseSensitiveDataKey(rawValue: string | undefined): Buffer | null {
  if (!rawValue) {
    return null;
  }

  const trimmed = rawValue.trim();
  if (trimmed.length === 0) {
    return null;
  }

  const base64Candidate = decodeKey(trimmed, 'base64');
  if (base64Candidate) {
    return base64Candidate;
  }

  const hexCandidate = decodeKey(trimmed, 'hex');
  if (hexCandidate) {
    return hexCandidate;
  }

  throw new Error('SENSITIVE_DATA_ENCRYPTION_KEY must be 32 bytes encoded as base64 or hex.');
}

export function encryptSensitiveValue(plaintext: string, key: Buffer): EncryptedSensitiveValue {
  const nonce = randomBytes(NONCE_BYTES);
  const cipher = createCipheriv(ENCRYPTION_ALGORITHM, key, nonce);
  const ciphertext = Buffer.concat([
    cipher.update(plaintext, 'utf8'),
    cipher.final(),
    cipher.getAuthTag()
  ]);

  return {
    ciphertext: ciphertext.toString('base64'),
    nonce: nonce.toString('base64')
  };
}

export function decryptSensitiveValue(ciphertext: string, nonce: string, key: Buffer): string {
  const ciphertextBuffer = Buffer.from(ciphertext, 'base64');
  const nonceBuffer = Buffer.from(nonce, 'base64');

  if (nonceBuffer.length !== NONCE_BYTES) {
    throw new Error('Sensitive data nonce had an invalid length.');
  }

  if (ciphertextBuffer.length <= AUTH_TAG_BYTES) {
    throw new Error('Sensitive data ciphertext was invalid.');
  }

  const authTag = ciphertextBuffer.subarray(ciphertextBuffer.length - AUTH_TAG_BYTES);
  const encryptedPayload = ciphertextBuffer.subarray(0, ciphertextBuffer.length - AUTH_TAG_BYTES);
  const decipher = createDecipheriv(ENCRYPTION_ALGORITHM, key, nonceBuffer);
  decipher.setAuthTag(authTag);

  return Buffer.concat([
    decipher.update(encryptedPayload),
    decipher.final()
  ]).toString('utf8');
}

function decodeKey(value: string, encoding: 'base64' | 'hex'): Buffer | null {
  try {
    const buffer = Buffer.from(value, encoding);
    return buffer.length === KEY_BYTES ? buffer : null;
  } catch {
    return null;
  }
}
