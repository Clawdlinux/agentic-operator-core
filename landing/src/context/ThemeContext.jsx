/* eslint-disable react-refresh/only-export-components */
import React, { createContext, useState, useEffect } from 'react';

const ThemeContext = createContext();

// Define light and dark theme palettes
export const themes = {
  light: {
    bg: {
      primary: '#ffffff',
      secondary: '#f8fafc',
      tertiary: '#f1f5f9',
      overlay: 'rgba(248, 250, 252, 0.95)',
    },
    text: {
      primary: '#0f172a',
      secondary: '#475569',
      tertiary: '#64748b',
      muted: '#94a3b8',
    },
    border: {
      light: 'rgba(15, 23, 42, 0.06)',
      medium: 'rgba(15, 23, 42, 0.12)',
    },
    accent: {
      teal: '#00d4aa',
      indigo: '#6366f1',
    },
  },
  dark: {
    bg: {
      primary: '#05080f',
      secondary: '#0f1420',
      tertiary: '#1a202c',
      overlay: 'rgba(5, 8, 15, 0.95)',
    },
    text: {
      primary: '#e2e8f0',
      secondary: '#cbd5e1',
      tertiary: '#94a3b8',
      muted: '#64748b',
    },
    border: {
      light: 'rgba(255, 255, 255, 0.06)',
      medium: 'rgba(255, 255, 255, 0.12)',
    },
    accent: {
      teal: '#00d4aa',
      indigo: '#6366f1',
    },
  },
};

export function ThemeProvider({ children }) {
  const [theme, setTheme] = useState(() => {
    // Check localStorage or system preference
    if (typeof window !== 'undefined') {
      const stored = localStorage.getItem('theme');
      if (stored) return stored;
      return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
    }
    return 'dark';
  });

  useEffect(() => {
    // Update localStorage and document attribute
    localStorage.setItem('theme', theme);
    if (typeof document !== 'undefined') {
      document.documentElement.setAttribute('data-theme', theme);
      document.documentElement.style.colorScheme = theme;
    }
  }, [theme]);

  const toggleTheme = () => {
    setTheme((prev) => (prev === 'dark' ? 'light' : 'dark'));
  };

  const currentTheme = themes[theme];

  return (
    <ThemeContext.Provider value={{ theme, toggleTheme, currentTheme, themes }}>
      {children}
    </ThemeContext.Provider>
  );
}

export default ThemeContext;
