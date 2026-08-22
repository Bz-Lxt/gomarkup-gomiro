/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{vue,ts}'],
  theme: {
    screens: {
      xs: '480px',
      md: '768px',
      lg: '1024px',
    },
    extend: {
      colors: {
        paper: '#f3eadc',
        ink: '#1c1915',
        brass: '#c4a35a',
        seal: '#9b2c2c',
        tealink: '#2f5d56',
        dusk: '#161411',
        panel: '#211c16',
      },
      fontFamily: {
        display: ['Syne', 'ui-sans-serif', 'sans-serif'],
        sans: ['Figtree', 'ui-sans-serif', 'sans-serif'],
      },
      boxShadow: {
        sheet: '0 18px 40px -24px rgba(28, 25, 21, 0.45)',
        pill: '0 10px 28px -16px rgba(28, 25, 21, 0.55)',
      },
    },
  },
  plugins: [],
}
