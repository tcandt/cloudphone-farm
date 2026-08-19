import React from 'react';

export interface BrandLogoProps {
  variant?: 'mark' | 'wordmark' | 'full';
  className?: string;
  size?: 'sm' | 'md' | 'lg';
}

export const BrandLogo: React.FC<BrandLogoProps> = ({ variant = 'full', className = '', size = 'md' }) => {
  // BLOCKED_BY_OWNER_ASSET: placeholder path
  // The final asset (cloudphonerental-mark.png) is pending owner upload.
  const assetPath = '/packages/brand/assets/cloudphonerental-mark.png'; 
  
  const sizeClasses = {
    sm: 'h-6',
    md: 'h-8',
    lg: 'h-10'
  };

  const mark = (
    <div className={`flex items-center justify-center shrink-0 ${className}`}>
      {/* We use an img tag pointing to the missing asset. A fallback background is added so it's visible while missing. */}
      <img 
        src={assetPath} 
        alt="CloudPhoneRental Mark" 
        className={`${sizeClasses[size]} w-auto object-contain bg-emerald-50 rounded-lg p-1 border border-emerald-100 border-dashed`} 
      />
    </div>
  );

  const wordmark = (
    <span className={`font-extrabold tracking-tight text-slate-800 ${size === 'sm' ? 'text-lg' : size === 'md' ? 'text-xl' : 'text-2xl'} ${className}`}>
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
