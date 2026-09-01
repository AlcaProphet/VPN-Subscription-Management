/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{vue,ts}'],
  theme: {
    extend: {
      colors: {
        page: 'var(--ui-page)',
        surface: 'var(--ui-surface)',
        'surface-subtle': 'var(--ui-surface-subtle)',
        'surface-raised': 'var(--ui-surface-raised)',
        border: 'var(--ui-border)',
        'border-subtle': 'var(--ui-border-subtle)',
        text: 'var(--ui-text)',
        'text-secondary': 'var(--ui-text-secondary)',
        'text-tertiary': 'var(--ui-text-tertiary)',
        primary: 'var(--ui-primary)',
        'primary-hover': 'var(--ui-primary-hover)',
        'primary-soft': 'var(--ui-primary-soft)',
        success: 'var(--ui-success)',
        'success-soft': 'var(--ui-success-soft)',
        warning: 'var(--ui-warning)',
        'warning-soft': 'var(--ui-warning-soft)',
        danger: 'var(--ui-danger)',
        'danger-soft': 'var(--ui-danger-soft)',
        info: 'var(--ui-info)',
        'info-soft': 'var(--ui-info-soft)',
      },
      boxShadow: {
        card: '0 1px 2px rgb(15 23 42 / 6%)',
        overlay: '0 12px 32px rgb(15 23 42 / 12%)',
      },
    },
  },
  plugins: [],
}
