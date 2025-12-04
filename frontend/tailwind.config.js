/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        aerospike: {
          primary: '#FACC15', // Bright Yellow
          secondary: '#FEF08A', // Lighter Yellow
          dark: '#EAB308', // Darker Yellow
          'light-blue': '#E5F4FA',
          'border-blue': '#4AA3DC',
        }
      }
    },
  },
  plugins: [],
}
