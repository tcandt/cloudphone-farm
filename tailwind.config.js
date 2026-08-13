/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        pcp: {
          bg: '#f3f4f8',
          surface: '#ffffff',
          primary: '#2563eb',
          'primary-hover': '#1d4ed8',
          accent: '#7c3aed',
          text: '#0f172a',
          muted: '#64748b',
          border: '#e2e8f0',
          activeBg: '#fff9eb',
          activeText: '#d97706',
          activeBorder: '#f59e0b',
          success: '#16a34a',
          warning: '#f59e0b',
          danger: '#dc2626',
        }
      },
      borderRadius: {
        '4xl': '2rem',
        '3xl': '1.5rem',
        '2xl': '1rem',
      },
      boxShadow: {
        'pcp-card': '0 4px 20px -2px rgba(15, 23, 42, 0.06), 0 2px 6px -1px rgba(15, 23, 42, 0.03)',
        'pcp-floating': '0 10px 30px -5px rgba(15, 23, 42, 0.08), 0 4px 12px -2px rgba(15, 23, 42, 0.04)',
      }
    },
  },
  plugins: [],
}
