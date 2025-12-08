import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { CronInput } from './CronInput'

describe('CronInput', () => {
  it('renders correctly with empty value', () => {
    render(<CronInput value="" onChange={() => {}} label="Test Cron" />)
    expect(screen.getByText('Test Cron')).toBeInTheDocument()
    // Should default to "Not scheduled" text or similar in preview
    expect(screen.getByText('Not scheduled')).toBeInTheDocument()
    // Should have "None (Disabled)" selected
    expect(screen.getByRole('combobox')).toHaveValue('none')
  })

  it('renders correctly with a Daily preset value', () => {
    // 0 30 14 * * ? -> Daily at 14:30
    const { container } = render(<CronInput value="0 30 14 * * ?" onChange={() => {}} />)
    expect(screen.getByRole('combobox')).toHaveValue('daily')
    // Check if time input is set correctly
    const timeInput = container.querySelector('input[type="time"]') as HTMLInputElement
    expect(timeInput).not.toBeNull()
    expect(timeInput.value).toBe('14:30')
  })

  it('switches to Raw mode for complex expressions', () => {
    // A complex cron that doesn't match our presets
    const complexCron = "0 0 12 1/5 * ?"
    render(<CronInput value={complexCron} onChange={() => {}} />)
    // Should find an input with the raw value
    expect(screen.getByDisplayValue(complexCron)).toBeInTheDocument()
    // Should NOT find the preset combobox (or it should be hidden/swapped)
    // Actually our component swaps the UI entirely.
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
  })

  it('calls onChange when preset is changed', () => {
    const handleChange = vi.fn()
    render(<CronInput value="" onChange={handleChange} />)
    
    const select = screen.getByRole('combobox')
    fireEvent.change(select, { target: { value: 'hourly' } })
    
    // Hourly preset: 0 0 * * * ?
    expect(handleChange).toHaveBeenCalledWith('0 0 * * * ?')
  })

  it('updates cron string when time changes in Daily mode', () => {
    const handleChange = vi.fn()
    // Start with Daily 00:00
    const { container } = render(<CronInput value="0 0 0 * * ?" onChange={handleChange} />)
    
    const timeInput = container.querySelector('input[type="time"]') as HTMLInputElement
    fireEvent.change(timeInput, { target: { value: '15:45' } })
    
    // Expect: 0 45 15 * * ?
    expect(handleChange).toHaveBeenCalledWith('0 45 15 * * ?')
  })
})
