import React, { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { useNavigate, Link } from 'react-router-dom';
import { Mail, Lock, Eye, EyeOff } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { AuthLayout } from '../../components/auth/AuthLayout';

const registerSchema = z.object({
  email: z.string().min(1, 'The field is required!').email('Địa chỉ email không hợp lệ'),
  password: z.string().min(6, 'Mật khẩu tối thiểu 6 ký tự'),
  confirmPassword: z.string().min(1, 'Vui lòng xác nhận mật khẩu'),
}).refine((data) => data.password === data.confirmPassword, {
  message: 'Mật khẩu xác nhận không khớp',
  path: ['confirmPassword'],
});

type RegisterForm = z.infer<typeof registerSchema>;

export const RegisterPage: React.FC = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();

  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<RegisterForm>({
    resolver: zodResolver(registerSchema),
  });

  const onSubmit = async (data: RegisterForm) => {
    await new Promise((res) => setTimeout(res, 600));
    navigate('/verify-email');
  };

  return (
    <AuthLayout mode="register">
      <div className="space-y-6">
        {/* Title Header */}
        <div className="text-center space-y-1">
          <h1 className="text-2xl sm:text-3xl font-extrabold text-amber-600 tracking-tight">
            {t('auth.signUpHeading')}
          </h1>
          <p className="text-xs sm:text-sm font-medium text-slate-400">
            {t('auth.signUpCredentials')}
          </p>
        </div>

        {/* Register Form */}
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          {/* Email Field */}
          <div>
            <label className="block text-xs font-bold text-slate-700 mb-1">
              <span className="text-red-500 mr-0.5">*</span> {t('auth.emailLabel')}
            </label>
            <div className="relative">
              <Mail size={16} className="absolute left-3.5 top-3.5 text-slate-400" />
              <input
                {...register('email')}
                type="email"
                placeholder="Enter your email"
                className={`w-full pl-10 pr-4 py-2.5 bg-slate-50/50 border rounded-xl text-sm font-medium outline-none transition-all ${
                  errors.email
                    ? 'border-red-400 focus:border-red-500 focus:ring-2 focus:ring-red-100 text-red-900 placeholder:text-red-300'
                    : 'border-slate-200 focus:border-amber-500 focus:bg-white focus:ring-2 focus:ring-amber-100 text-slate-900 placeholder:text-slate-400'
                }`}
              />
            </div>
            {errors.email && (
              <p className="text-xs text-red-500 font-semibold mt-1 animate-fadeIn">
                {errors.email.message}
              </p>
            )}
          </div>

          {/* 2-Column Passwords Row */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {/* Password */}
            <div>
              <label className="block text-xs font-bold text-slate-700 mb-1">
                <span className="text-red-500 mr-0.5">*</span> {t('auth.passwordLabel')}
              </label>
              <div className="relative">
                <Lock size={16} className="absolute left-3 top-3.5 text-slate-400" />
                <input
                  {...register('password')}
                  type={showPassword ? 'text' : 'password'}
                  placeholder="Enter your passw..."
                  className={`w-full pl-9 pr-9 py-2.5 bg-slate-50/50 border rounded-xl text-xs font-medium outline-none transition-all ${
                    errors.password
                      ? 'border-red-400 focus:border-red-500 focus:ring-2 focus:ring-red-100 text-red-900 placeholder:text-red-300'
                      : 'border-slate-200 focus:border-amber-500 focus:bg-white focus:ring-2 focus:ring-amber-100 text-slate-900 placeholder:text-slate-400'
                  }`}
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute right-3 top-3.5 text-slate-400 hover:text-slate-600 transition-colors"
                >
                  {showPassword ? <EyeOff size={14} /> : <Eye size={14} />}
                </button>
              </div>
              {errors.password && (
                <p className="text-[11px] text-red-500 font-semibold mt-1 animate-fadeIn">
                  {errors.password.message}
                </p>
              )}
            </div>

            {/* Confirm Password */}
            <div>
              <label className="block text-xs font-bold text-slate-700 mb-1">
                <span className="text-red-500 mr-0.5">*</span> {t('auth.confirmPasswordLabel')}
              </label>
              <div className="relative">
                <Lock size={16} className="absolute left-3 top-3.5 text-slate-400" />
                <input
                  {...register('confirmPassword')}
                  type={showConfirmPassword ? 'text' : 'password'}
                  placeholder="Confirm your pas..."
                  className={`w-full pl-9 pr-9 py-2.5 bg-slate-50/50 border rounded-xl text-xs font-medium outline-none transition-all ${
                    errors.confirmPassword
                      ? 'border-red-400 focus:border-red-500 focus:ring-2 focus:ring-red-100 text-red-900 placeholder:text-red-300'
                      : 'border-slate-200 focus:border-amber-500 focus:bg-white focus:ring-2 focus:ring-amber-100 text-slate-900 placeholder:text-slate-400'
                  }`}
                />
                <button
                  type="button"
                  onClick={() => setShowConfirmPassword(!showConfirmPassword)}
                  className="absolute right-3 top-3.5 text-slate-400 hover:text-slate-600 transition-colors"
                >
                  {showConfirmPassword ? <EyeOff size={14} /> : <Eye size={14} />}
                </button>
              </div>
              {errors.confirmPassword && (
                <p className="text-[11px] text-red-500 font-semibold mt-1 animate-fadeIn">
                  {errors.confirmPassword.message}
                </p>
              )}
            </div>
          </div>

          {/* Primary Action Button (Amber/Orange) */}
          <button
            type="submit"
            disabled={isSubmitting}
            className="w-full py-3 bg-[#df7f00] hover:bg-[#c97200] active:scale-[0.99] text-white font-extrabold text-sm rounded-xl shadow-md transition-all disabled:opacity-50 mt-4"
          >
            {isSubmitting ? 'Creating account...' : t('auth.signUpBtn')}
          </button>
        </form>

        {/* Footer Navigation Link */}
        <div className="text-center pt-4 text-xs font-semibold text-slate-600">
          <span>{t('auth.alreadyHaveAccount')} </span>
          <Link to="/login" className="text-amber-600 hover:underline font-bold">
            {t('auth.signInNow')}
          </Link>
        </div>
      </div>
    </AuthLayout>
  );
};
