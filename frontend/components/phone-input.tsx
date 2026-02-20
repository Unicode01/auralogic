'use client'

import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { phoneCodeList } from '@/lib/phone-codes'

interface PhoneInputProps {
  countryCode: string
  onCountryCodeChange: (code: string) => void
  phone: string
  onPhoneChange: (phone: string) => void
  placeholder?: string
  className?: string
}

export function PhoneInput({ countryCode, onCountryCodeChange, phone, onPhoneChange, placeholder, className = 'h-10' }: PhoneInputProps) {
  return (
    <div className="flex gap-2">
      <Select value={countryCode} onValueChange={onCountryCodeChange}>
        <SelectTrigger className={`w-[100px] shrink-0 ${className}`}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent className="max-h-60">
          {phoneCodeList.map((c) => (
            <SelectItem key={c.code} value={c.code}>
              {c.code}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Input
        type="tel"
        placeholder={placeholder}
        className={`flex-1 ${className}`}
        value={phone}
        onChange={(e) => onPhoneChange(e.target.value.replace(/[^\d]/g, ''))}
      />
    </div>
  )
}
