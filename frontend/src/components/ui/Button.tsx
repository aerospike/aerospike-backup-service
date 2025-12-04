import React from 'react';
import {Loader, LucideIcon} from 'lucide-react';
import {type ClassValue, clsx} from 'clsx';
import {twMerge} from 'tailwind-merge';

// Utility for merging tailwind classes
function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'action';

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  icon?: LucideIcon;
  loading?: boolean;
}

const variants: Record<ButtonVariant, string> = {
  primary: "bg-red-600 hover:bg-red-700 text-white focus:ring-red-500",
  secondary: "bg-white hover:bg-gray-50 text-gray-700 border border-gray-300 focus:ring-gray-200",
  ghost: "text-gray-500 hover:text-gray-900 hover:bg-gray-100",
  danger: "text-red-600 hover:text-red-700 hover:bg-red-50",
  action: "bg-aerospike-primary hover:bg-aerospike-secondary text-black focus:ring-aerospike-primary"
};

export const Button = ({ children, variant = 'primary', icon: Icon, className, loading, ...props }: ButtonProps) => {
  return (
    <button
      className={cn(
        "flex items-center gap-2 px-4 py-2 rounded-md font-medium text-sm transition-all focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-white disabled:opacity-50 disabled:cursor-not-allowed",
        variants[variant],
        className
      )}
      disabled={loading || props.disabled}
      {...props}
    >
      {loading ? <Loader size={16} className="animate-spin" /> : (Icon && <Icon size={16} />)}
      {children}
    </button>
  );
};