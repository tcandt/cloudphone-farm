import React from 'react';
import cloudPhoneRentalMark from '../assets/cloudphonerental-mark.png';

export interface BrandLogoProps {
  variant?: 'mark' | 'wordmark' | 'full';
  className?: string;
  size?: 'sm' | 'md' | 'lg';
}

export const BrandLogo: React.FC<BrandLogoProps> = ({ variant = 'full', className = '', size = 'md' }) => {
  const sizeClasses = {
    sm: 'text-lg',
    md: 'text-xl',
    lg: 'text-2xl'
  };

  const imgSizeClasses = {
    sm: 'h-6',
    md: 'h-8',
    lg: 'h-10'
  };

  const mark = (
    <div className={`flex items-center justify-center shrink-0 ${className}`}>
      <img src={cloudPhoneRentalMark} alt="CloudPhoneRental Mark" className={`${imgSizeClasses[size]} w-auto object-contain`} />
    </div>
  );

  const wordmark = (
    <span className={`font-extrabold tracking-tight text-slate-800 ${sizeClasses[size]} ${className}`}>
      CloudPhone<span className="text-emerald-600">Rental</span>
    </span>
  );

  if (variant === 'mark') return mark;
  if (variant === 'wordmark') return wordmark;

  return (
    <div className={`flex items-center gap-2.5 select-none ${className}`}>
      {mark}
      {wordmark}
    </div>
  );
};
