import { useEffect } from 'react'

/** Sets the browser tab/history title to "<title> · Gryvia" while the page is mounted. */
export function useDocumentTitle(title: string): void {
  useEffect(() => {
    const previous = document.title
    document.title = title ? `${title} · Gryvia` : 'Gryvia'
    return () => {
      document.title = previous
    }
  }, [title])
}
