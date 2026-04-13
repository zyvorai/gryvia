/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      borderRadius: {
        lg: 'var(--radius)',
        md: 'calc(var(--radius) - 2px)',
        sm: 'calc(var(--radius) - 4px)',
      },
      colors: {
        metal: {
          900: '#0a0e14',
          850: '#0d1219',
          800: '#111820',
          750: '#151d28',
          700: '#1a2332',
          600: '#243044',
          500: '#344e6a',
          400: '#5a7a9e',
          300: '#8ba4c0',
          200: '#b0c4d8',
          100: '#d0dae6',
        },
        copper: {
          500: '#d4764e',
          400: '#e8a87c',
          300: '#f0c4a0',
        },
        chrome: {
          500: '#5fa8d3',
          400: '#7ecbf5',
          300: '#aee0fc',
        },
        titanium: {
          500: '#8090a8',
          400: '#9caabe',
          300: '#c0cce0',
        },
      },
    },
  },
  plugins: [],
}
