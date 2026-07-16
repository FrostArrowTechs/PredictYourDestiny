// Theme switcher. A single button that flips between dark and light
// and persists the choice via the theme helper.
import { useEffect, useState } from 'react'
import { Moon, Sun } from 'lucide-react'
import { getTheme, setTheme, type Theme } from '../theme'

export default function ThemeSwitcher() {
  const [theme, setLocalTheme] = useState<Theme>('dark')

  useEffect(() => {
    setLocalTheme(getTheme())
  }, [])

  const handleClick = () => {
    const next: Theme = theme === 'dark' ? 'light' : 'dark'
    setTheme(next)
    setLocalTheme(next)
  }

  return (
    <button
      type="button"
      onClick={handleClick}
      aria-label={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
      className="inline-flex items-center justify-center w-8 h-8 rounded-md text-muted hover:text-fg hover:bg-surface transition-colors"
    >
      {theme === 'dark' ? <Sun className="w-4 h-4" /> : <Moon className="w-4 h-4" />}
    </button>
  )
}