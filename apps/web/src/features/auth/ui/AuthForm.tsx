import { zodResolver } from '@hookform/resolvers/zod';
import { useState } from 'react';
import { Controller, useForm, useWatch } from 'react-hook-form';

import { cx } from '@/shared/lib/cx';
import { Button, Checkbox, Icon, IconButton, Input } from '@/shared/ui';

import { mapAuthError, useLoginMutation, useRegisterMutation } from '../api/authApi';
import {
  loginSchema,
  MIN_PASSWORD_LENGTH,
  passwordStrength,
  registerSchema,
  type LoginValues,
  type PasswordStrengthLevel,
  type RegisterValues,
} from '../model/schemas';

import styles from './AuthForm.module.css';

export interface AuthFormProps {
  mode: 'login' | 'register';
  onSuccess: () => void;
}

export function AuthForm({ mode, onSuccess }: AuthFormProps) {
  return mode === 'login' ? <LoginForm onSuccess={onSuccess} /> : <RegisterForm onSuccess={onSuccess} />;
}

function ErrorBanner({ message }: { message: string | null }) {
  if (!message) return null;
  return (
    <div role="alert" className={styles.banner}>
      <Icon name="alert" size={16} className={styles.bannerIcon} />
      <span>{message}</span>
    </div>
  );
}

function PasswordToggle({ visible, onToggle }: { visible: boolean; onToggle: () => void }) {
  return (
    <IconButton
      icon={visible ? 'eye-off' : 'eye'}
      label={visible ? 'Hide password' : 'Show password'}
      size="sm"
      pressed={visible}
      onClick={onToggle}
    />
  );
}

const STRENGTH_CLASS: Record<PasswordStrengthLevel, string | undefined> = {
  empty: undefined,
  weak: styles.pwWeak,
  fair: styles.pwFair,
  good: styles.pwGood,
  strong: styles.pwStrong,
};

function PasswordStrengthMeter({ value }: { value: string }) {
  const { score, level, label } = passwordStrength(value);
  const toneClass = STRENGTH_CLASS[level];
  return (
    <div className={styles.pwMeter}>
      <div className={styles.pwBars} aria-hidden="true">
        {[0, 1, 2, 3].map((idx) => (
          <span key={idx} className={cx(styles.pwBar, idx < score && toneClass)} />
        ))}
      </div>
      <div className={styles.pwMeta}>
        <span>Use {MIN_PASSWORD_LENGTH}+ characters with a number or symbol</span>
        <span className={styles.pwLabel}>{label}</span>
      </div>
    </div>
  );
}

function LoginForm({ onSuccess }: { onSuccess: () => void }) {
  const [formError, setFormError] = useState<string | null>(null);
  const [showPassword, setShowPassword] = useState(false);
  const login = useLoginMutation();
  const {
    register,
    control,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<LoginValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: '', password: '', remember: true },
  });

  const onSubmit = handleSubmit(async (values) => {
    setFormError(null);
    try {
      await login.mutateAsync({ email: values.email, password: values.password });
      onSuccess();
    } catch (err) {
      const mapped = mapAuthError(err);
      if (mapped.fieldErrors?.email) setError('email', { message: mapped.fieldErrors.email });
      if (mapped.fieldErrors?.password) setError('password', { message: mapped.fieldErrors.password });
      if (mapped.formError) setFormError(mapped.formError);
    }
  });

  return (
    <form className={styles.form} onSubmit={onSubmit} noValidate>
      <ErrorBanner message={formError} />
      <Input
        label="Email address"
        type="email"
        size="lg"
        iconStart="mail"
        autoComplete="email"
        placeholder="rin@icyre.audio"
        error={errors.email?.message}
        {...register('email')}
      />
      <Input
        label="Password"
        type={showPassword ? 'text' : 'password'}
        size="lg"
        iconStart="lock"
        autoComplete="current-password"
        placeholder="Enter your password"
        error={errors.password?.message}
        endAdornment={<PasswordToggle visible={showPassword} onToggle={() => setShowPassword((v) => !v)} />}
        {...register('password')}
      />
      <Controller
        control={control}
        name="remember"
        render={({ field }) => (
          <Checkbox
            label="Keep me signed in on this device"
            checked={field.value}
            onChange={field.onChange}
          />
        )}
      />
      <div className={styles.submit}>
        <Button type="submit" variant="primary" size="lg" fullWidth loading={isSubmitting || login.isPending}>
          Sign in
        </Button>
      </div>
    </form>
  );
}

function RegisterForm({ onSuccess }: { onSuccess: () => void }) {
  const [formError, setFormError] = useState<string | null>(null);
  const [showPassword, setShowPassword] = useState(false);
  const registerMutation = useRegisterMutation();
  const {
    register,
    control,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<RegisterValues>({
    resolver: zodResolver(registerSchema),
    defaultValues: { email: '', password: '', confirmPassword: '', acceptTerms: true },
  });

  const passwordValue = useWatch({ control, name: 'password' });

  const onSubmit = handleSubmit(async (values) => {
    setFormError(null);
    try {
      await registerMutation.mutateAsync({ email: values.email, password: values.password });
      onSuccess();
    } catch (err) {
      const mapped = mapAuthError(err);
      if (mapped.fieldErrors?.email) setError('email', { message: mapped.fieldErrors.email });
      if (mapped.fieldErrors?.password) setError('password', { message: mapped.fieldErrors.password });
      if (mapped.formError) setFormError(mapped.formError);
    }
  });

  return (
    <form className={styles.form} onSubmit={onSubmit} noValidate>
      <ErrorBanner message={formError} />
      <Input
        label="Email address"
        type="email"
        size="lg"
        iconStart="mail"
        autoComplete="email"
        placeholder="rin@icyre.audio"
        error={errors.email?.message}
        {...register('email')}
      />
      <div>
        <Input
          label="Password"
          type={showPassword ? 'text' : 'password'}
          size="lg"
          iconStart="lock"
          autoComplete="new-password"
          placeholder="At least 10 characters"
          error={errors.password?.message}
          endAdornment={<PasswordToggle visible={showPassword} onToggle={() => setShowPassword((v) => !v)} />}
          {...register('password')}
        />
        <PasswordStrengthMeter value={passwordValue} />
      </div>
      <Input
        label="Confirm password"
        type={showPassword ? 'text' : 'password'}
        size="lg"
        iconStart="lock"
        autoComplete="new-password"
        placeholder="Repeat your password"
        error={errors.confirmPassword?.message}
        {...register('confirmPassword')}
      />
      <div>
        <Controller
          control={control}
          name="acceptTerms"
          render={({ field }) => (
            <Checkbox
              label="I agree to the Terms and Privacy Policy"
              checked={field.value}
              onChange={field.onChange}
            />
          )}
        />
        {errors.acceptTerms?.message && (
          <div className={styles.fieldError} role="alert">
            {errors.acceptTerms.message}
          </div>
        )}
      </div>
      <div className={styles.submit}>
        <Button type="submit" variant="primary" size="lg" fullWidth loading={isSubmitting || registerMutation.isPending}>
          Create account
        </Button>
      </div>
    </form>
  );
}
