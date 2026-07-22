/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // 与 Element Plus primary 对齐，便于过渡期混用
        primary: { DEFAULT: '#409eff', dark: '#409eff' },
      },
    },
  },
  plugins: [],
}
