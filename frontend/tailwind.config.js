/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js,ts}'],
  theme: {
    extend: {
      colors: {
        // Foundation (black & white)
        bg: {
          primary: '#0A0A0A',      // Near-black
          secondary: '#FFFFFF',    // Pure white
          tertiary: '#F5F5F5',     // Off-white
        },
        // Accent - Electric Blue
        accent: {
          primary: '#0066FF',      // Electric blue
          hover: '#0052CC',        // Darker blue for hover
        },
        // Typography
        text: {
          primary: '#0A0A0A',      // Near-black on white
          secondary: '#FFFFFF',    // White on black
          muted: '#666666',        // Mid-gray
        },
        // Borders (visible, structural)
        border: {
          thin: '#E0E0E0',         // Light gray grid lines
          thick: '#0A0A0A',        // Near-black strong borders
          accent: '#0066FF',       // Accent color borders
        },
        // Semantic (minimal)
        success: '#00FF00',
        error: '#FF0000',
      },
      fontFamily: {
        display: ['"Space Grotesk"', 'sans-serif'],
        mono: ['"JetBrains Mono"', 'monospace'],
        accent: ['"Archivo Black"', 'sans-serif'],
      },
      fontSize: {
        'xs': '0.75rem',      // 12px - metadata
        'sm': '0.875rem',     // 14px - secondary
        'base': '1rem',       // 16px - body
        'lg': '1.125rem',     // 18px - emphasis
        'xl': '1.5rem',       // 24px - card titles
        '2xl': '2rem',        // 32px - section headings
        '3xl': '3rem',        // 48px - page headings
        '4xl': '5rem',        // 80px - hero display
        '5xl': '7rem',        // 112px - massive display
      },
      spacing: {
        '18': '4.5rem',   // 72px
        '22': '5.5rem',   // 88px
        '26': '6.5rem',   // 104px
        '30': '7.5rem',   // 120px
      },
      borderRadius: {
        'none': '0',
        'sm': '2px',      // Minimal rounding for small elements only
        'DEFAULT': '0',   // Default to sharp
      },
      borderWidth: {
        '4': '4px',       // Thick borders for brutalist effect
        '8': '8px',       // Extra thick for emphasis
      },
      boxShadow: {
        'brutal-sm': '4px 4px 0px 0px #0A0A0A',
        'brutal-md': '8px 8px 0px 0px #0A0A0A',
        'brutal-lg': '12px 12px 0px 0px #0A0A0A',
        'brutal-accent': '4px 4px 0px 0px #0066FF',
        'none': 'none',
      },
      transitionDuration: {
        'fast': '100ms',   // Snappy interactions
        'base': '150ms',   // Standard hover
        'slow': '300ms',   // Layout changes
      },
      transitionTimingFunction: {
        'harsh': 'cubic-bezier(0, 0, 0.2, 1)',     // Mechanical feel
        'snappy': 'cubic-bezier(0.4, 0, 0.6, 1)',
      },
      animation: {
        'snap-in': 'snapIn 0.15s cubic-bezier(0, 0, 0.2, 1)',
        'stagger': 'snapIn 0.15s cubic-bezier(0, 0, 0.2, 1)',
      },
      keyframes: {
        snapIn: {
          '0%': { opacity: '0', transform: 'translateY(4px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
      },
      letterSpacing: {
        'tighter': '-0.05em',
        'wide': '0.1em',
      },
    },
  },
  plugins: [],
}

