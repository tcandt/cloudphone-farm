import React from 'react';

export interface BrandLogoProps {
  variant?: 'mark' | 'wordmark' | 'full';
  className?: string;
  size?: 'sm' | 'md' | 'lg';
}

export const BrandLogo: React.FC<BrandLogoProps> = ({ variant = 'full', className = '', size = 'md' }) => {
  // BLOCKED_BY_OWNER_ASSET: The final asset (cloudphonerental-mark.png) is pending owner upload.
  // Once provided, import normally: import cloudPhoneRentalMark from '../assets/cloudphonerental-mark.png';
  const assetExists = false; 
  
  const sizeClasses = {
    sm: 'text-lg',
    md: 'text-xl',
    lg: 'text-2xl'
  };

  const mark = (
    <div className={`flex items-center justify-center shrink-0 ${className}`}>
      {assetExists ? (
        <>
          {/* Future: <img src={cloudPhoneRentalMark} alt="CloudPhoneRental Mark" className="w-auto h-full object-contain" /> */}
          <span />
        </>
      ) : (
        <div className="text-[10px] font-bold text-rose-500 bg-rose-50 p-2 rounded border border-rose-200 border-dashed max-w-[120px] text-center">
          BLOCKED_BY_OWNER_ASSET
        </div>
      )}
    </div>
  );

  const wordmark = (
    <span className={`font-extrabold tracking-tight text-slate-800 ${sizeClasses[size]} ${className}`}>
      CloudPhone<span className="text-emerald-600">Rental</span>
    </span>
  );

  if (variant === 'mark') return mark;
  if (variant === 'wordmark') return wordmark;

  // For 'full', if asset is missing, render wordmark only per instructions
  if (!assetExists) return wordmark;

  return (
    <div className={`flex items-center gap-2.5 select-none ${className}`}>
      {mark}
      {wordmark}
    </div>
  );
};
