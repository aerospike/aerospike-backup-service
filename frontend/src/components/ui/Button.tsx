import React from 'react';
import { LucideIcon } from 'lucide-react';
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

// Utility for merging tailwind classes
function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'action';

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  icon?: LucideIcon;
}

const variants: Record<ButtonVariant, string> = {
  primary: "bg-red-600 hover:bg-red-700 text-white focus:ring-red-500",
  secondary: "bg-gray-800 hover:bg-gray-700 text-gray-200 border border-gray-700 focus:ring-gray-500",
  ghost: "text-gray-400 hover:text-white hover:bg-gray-800",
  danger: "text-red-400 hover:text-red-300 hover:bg-red-900/20",
  action: "bg-blue-600 hover:bg-blue-700 text-white focus:ring-blue-500"
};

export const Button = ({ children, variant = 'primary', icon: Icon, className, ...props }: ButtonProps) => {
  return (
    <button
      className={cn(
        "flex items-center gap-2 px-4 py-2 rounded-md font-medium text-sm transition-all focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-gray-900 disabled:opacity-50 disabled:cursor-not-allowed",
        variants[variant],
        className
      )}
      {...props}
    >
      {Icon && <Icon size={16} />}
      {children}
    </button>
  );
};