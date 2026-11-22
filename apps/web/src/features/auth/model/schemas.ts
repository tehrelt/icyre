import { z } from 'zod';

export const MIN_PASSWORD_LENGTH = 10;
export const MAX_PASSWORD_LENGTH = 128;

const emailField = z
  .string()
  .trim()
  .min(1, 'Enter your email')
  .max(254, 'Email must be at most 254 characters')
  .email('Enter a valid email address');

export const loginSchema = z.object({
  email: emailField,
  password: z.string().min(1, 'Enter your password'),
  remember: z.boolean(),
});

export const registerSchema = z
  .object({
    email: emailField,
    password: z
      .string()
      .min(MIN_PASSWORD_LENGTH, `Use at least ${MIN_PASSWORD_LENGTH} characters`)
      .max(MAX_PASSWORD_LENGTH, `Use at most ${MAX_PASSWORD_LENGTH} characters`)
      .refine((value) => value.trim().length > 0, 'Password must not be blank'),
    confirmPassword: z.string().min(1, 'Confirm your password'),
    acceptTerms: z.boolean().refine((v) => v, 'Accept the Terms and Privacy Policy to create an account.'),
  })
  .refine((data) => data.password === data.confirmPassword, {
    message: 'Passwords do not match',
    path: ['confirmPassword'],
  });

export type LoginValues = z.infer<typeof loginSchema>;
export type RegisterValues = z.infer<typeof registerSchema>;

export type PasswordStrengthLevel = 'empty' | 'weak' | 'fair' | 'good' | 'strong';

export function passwordStrength(value: string): { score: number; level: PasswordStrengthLevel; label: string } {
  let score = 0;
  if (value.length >= MIN_PASSWORD_LENGTH) score++;
  if (/[A-Z]/.test(value) && /[a-z]/.test(value)) score++;
  if (/\d/.test(value)) score++;
  if (/[^A-Za-z0-9]/.test(value)) score++;
  if (value.length > 0 && score === 0) score = 1;

  const levels: PasswordStrengthLevel[] = ['empty', 'weak', 'fair', 'good', 'strong'];
  const labels = ['Empty', 'Weak', 'Fair', 'Good', 'Strong'];
  return {
    score,
    level: levels[score] ?? 'empty',
    label: labels[score] ?? 'Empty',
  };
}
