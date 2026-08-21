import { useCallback, useEffect, useRef, useState } from 'react'
import { errorMessage } from './utils'

export function useLoad<T>(loader: () => Promise<T>) {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState('')
  const generation = useRef(0)

  const load = useCallback(async () => {
    const current = ++generation.current
    try {
      setError('')
      const result = await loader()
      if (current !== generation.current) return
      setData(result)
    } catch (reason) {
      if (current !== generation.current) return
      setError(errorMessage(reason))
    }
  }, [loader])

  useEffect(() => {
    void load()
  }, [load])

  return { data, error, load }
}
