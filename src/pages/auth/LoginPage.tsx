import React, { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { useNavigate, Link } from 'react-router-dom';
import { User, Lock, Eye, EyeOff, ChevronDown } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { AuthLayout } from '../../components/auth/AuthLayout';

const loginSchema = z.object({
  email: z.string().min(1, 'The field is required!').email('Địa chỉ email không hợp lệ'),
  password: z.string().min(1, 'The Password field is required!'),
  agreeTerms: z.boolean().optional(),
});

type LoginForm = z.infer<typeof loginSchema>;

export const LoginPage: React.FC = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [showPassword, setShowPassword] = useState(false);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<LoginForm>({
    resolver: zodResolver(loginSchema),
    defaultValues: {
      email: '',
      password: '',
      agreeTerms: true,
    },
  });

  const onSubmit = async (data: LoginForm) => {
    // Prototype login simulation
    await new Promise((res) => setTimeout(res, 600));
    navigate('/app');
  };

  return (
    <AuthLayout mode="login">
      <div className="space-y-6">
        {/* Title Header */}
        <div className="text-center space-y-1">
          <h1 className="text-2xl sm:text-3xl font-extrabold text-amber-600 tracking-tight">
            {t('auth.welcomeBack')}
          </h1>
          <p className="text-xs sm:text-sm font-medium text-slate-400">
            {t('auth.enterCredentials')}
          </p>
        </div>

        {/* Login Form */}
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          {/* Email Field */}
          <div>
            <label className="block text-xs font-bold text-slate-700 mb-1">
              <span className="text-red-500 mr-0.5">*</span> {t('auth.emailLabel')}
            </label>
            <div className="relative">
              <User size={16} className={`absolute left-3.5 top-3.5 ${errors.email ? 'text-red-400' : 'text-rose-400'}`} />
              <input
                {...register('email')}
                type="email"
                placeholder={t('auth.emailPlaceholder')}
                className={`w-full pl-10 pr-4 py-2.5 bg-white border rounded-xl text-sm font-medium outline-none transition-all ${
                  errors.email
                    ? 'border-red-400 focus:border-red-500 focus:ring-2 focus:ring-red-100 text-red-900 placeholder:text-red-300'
                    : 'border-red-200 focus:border-amber-500 focus:ring-2 focus:ring-amber-100 text-slate-900 placeholder:text-slate-300'
                }`}
              />
            </div>
            {errors.email && (
              <p className="text-xs text-red-500 font-semibold mt-1 animate-fadeIn">
                {errors.email.message}
              </p>
            )}
          </div>

          {/* Password Field */}
          <div>
            <label className="block text-xs font-bold text-slate-700 mb-1">
              <span className="text-red-500 mr-0.5">*</span> {t('auth.passwordLabel')}
            </label>
            <div className="relative">
              <Lock size={16} className={`absolute left-3.5 top-3.5 ${errors.password ? 'text-red-400' : 'text-rose-400'}`} />
              <input
                {...register('password')}
                type={showPassword ? 'text' : 'password'}
                placeholder={t('auth.passwordPlaceholder')}
                className={`w-full pl-10 pr-10 py-2.5 bg-white border rounded-xl text-sm font-medium outline-none transition-all ${
                  errors.password
                    ? 'border-red-400 focus:border-red-500 focus:ring-2 focus:ring-red-100 text-red-900 placeholder:text-red-300'
                    : 'border-red-200 focus:border-amber-500 focus:ring-2 focus:ring-amber-100 text-slate-900 placeholder:text-slate-300'
                }`}
              />
              <button
                type="button"
                onClick={() => setShowPassword(!showPassword)}
                className="absolute right-3.5 top-3.5 text-rose-400 hover:text-slate-600 transition-colors"
              >
                {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
              </button>
            </div>
            {errors.password && (
              <p className="text-xs text-red-500 font-semibold mt-1 animate-fadeIn">
                {errors.password.message}
              </p>
            )}
          </div>

          {/* Terms Checkbox & Forgot Password Row */}
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 pt-1 text-xs">
            <label className="flex items-start gap-2 cursor-pointer text-slate-600 font-medium select-none">
              <input
                {...register('agreeTerms')}
                type="checkbox"
                className="mt-0.5 w-4 h-4 text-amber-600 border-slate-300 rounded focus:ring-amber-500 cursor-pointer accent-amber-600"
              />
              <span className="leading-tight">
                When sign in, you agree to our{' '}
                <a href="#terms" className="text-slate-700 underline font-semibold hover:text-amber-600">
                  terms and conditions.
                </a>
              </span>
            </label>

            <Link
              to="/forgot-password"
              className="text-amber-600 font-bold hover:underline whitespace-nowrap self-end sm:self-auto"
            >
              {t('auth.forgotPasswordLink')}
            </Link>
          </div>

          {/* Primary Action Button (Amber/Orange) */}
          <button
            type="submit"
            disabled={isSubmitting}
            className="w-full py-3 bg-[#df7f00] hover:bg-[#c97200] active:scale-[0.99] text-white font-extrabold text-sm rounded-xl shadow-md transition-all disabled:opacity-50 mt-2"
          >
            {isSubmitting ? 'Processing...' : t('auth.signInBtn')}
          </button>
        </form>

        {/* Or Divider */}
        <div className="relative flex items-center justify-center my-4">
          <div className="w-full border-t border-slate-200"></div>
          <span className="absolute bg-white px-3 text-xs font-semibold text-slate-400">
            {t('auth.or')}
          </span>
        </div>

        {/* Google SSO Button */}
        <div className="flex justify-center">
          <button
            type="button"
            onClick={() => navigate('/app')}
            className="flex items-center gap-2 px-4 py-2 bg-white border border-slate-200 hover:border-slate-300 rounded-lg shadow-sm text-xs font-semibold text-slate-700 transition-all hover:bg-slate-50"
          >
            {/* Google SVG Icon */}
            <svg className="w-4 h-4" viewBox="0 0 24 24">
              <path
                fill="#4285F4"
                d="M23.745 12.27c0-.7-.06-1.4-.19-2.07H12v4.51h6.6c-.29 1.52-1.14 2.82-2.4 3.68v3.05h3.88c2.27-2.09 3.665-5.17 3.665-9.17z"
              />
              <path
                fill="#34A853"
                d="M12 24c3.24 0 5.95-1.08 7.93-2.91l-3.88-3.05c-1.08.72-2.45 1.16-4.05 1.16-3.12 0-5.77-2.11-6.72-4.96H1.26v3.15C3.25 21.31 7.31 24 12 24z"
              />
              <path
                fill="#FBBC05"
                d="M5.28 14.24c-.25-.72-.38-1.49-.38-2.24s.13-1.52.38-2.24V6.61H1.26C.46 8.23 0 10.06 0 12s.46 3.77 1.26 5.39l4.02-3.15z"
              />
              <path
                fill="#EA4335"
                d="M12 4.75c1.77 0 3.35.61 4.6 1.8l3.42-3.42C17.95 1.19 15.24 0 12 0 7.31 0 3.25 2.69 1.26 6.61l4.02 3.15c.95-2.85 3.6-4.96 6.72-4.96z"
              />
            </svg>
            <span>{t('auth.googleSignInAs')}</span>
            <span className="text-[10px] text-slate-400 font-mono">(tinhyt2018@gmail.com)</span>
            <ChevronDown size={14} className="text-slate-400 ml-1" />
          </button>
        </div>

        {/* Footer Navigation Link */}
        <div className="text-center pt-2 text-xs font-semibold text-slate-600">
          <span>{t('auth.noAccountText')} </span>
          <Link to="/register" className="text-amber-600 hover:underline font-bold">
            {t('auth.signUpNow')}
          </Link>
        </div>
      </div>
    </AuthLayout>
  );
};
