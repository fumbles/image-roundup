import { useState } from 'react'
import { Button } from '@carbon/react'
import { Copy, Checkmark } from '@carbon/icons-react'

interface CopyButtonProps {
  text: string
  label?: string
}

export default function CopyButton({ text, label = 'Copy' }: CopyButtonProps) {
  const [copied, setCopied] = useState(false)

  const handleClick = async () => {
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    } catch {
      // ignore — clipboard not available in all contexts
    }
  }

  return (
    <Button
      kind="ghost"
      size="sm"
      renderIcon={copied ? Checkmark : Copy}
      iconDescription={label}
      hasIconOnly
      onClick={handleClick}
      tooltipAlignment="end"
    />
  )
}
