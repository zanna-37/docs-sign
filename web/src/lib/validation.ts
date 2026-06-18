// Minimum password length enforced client-side for snappy feedback; the server is the
// authority and enforces the same rule.
export const MIN_PASSWORD_LENGTH = 8;

// validateNewPassword checks a new password against its confirmation and returns the i18n
// key of the first failing rule, or null when the password is acceptable.
export function validateNewPassword(
  password: string,
  confirm: string,
): string | null {
  if (password.length < MIN_PASSWORD_LENGTH) return "common.passwordTooShort";
  if (password !== confirm) return "common.passwordsDontMatch";
  return null;
}
