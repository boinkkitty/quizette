import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import type { UploadInput } from '../api/queries'
import { topics } from '../test/server'
import { UploadForm } from './UploadForm'

const pdf = () => new File(['%PDF-1.4'], 'cells.pdf', { type: 'application/pdf' })

describe('UploadForm', () => {
  it('offers question counts 5-30 in steps of 5', () => {
    render(<UploadForm topics={topics} pending={false} onSubmit={vi.fn<(input: UploadInput) => void>()} />)
    const select = screen.getByRole('combobox', { name: 'Questions to generate' })
    expect(within(select).getAllByRole('option').map((o) => o.textContent)).toEqual(['5', '10', '15', '20', '25', '30'])
  })

  it('rejects non-PDF files and requires a file', async () => {
    const user = userEvent.setup({ applyAccept: false })
    const onSubmit = vi.fn<(input: UploadInput) => void>()
    render(<UploadForm topics={topics} pending={false} onSubmit={onSubmit} />)
    await user.click(screen.getByRole('button', { name: /generate questions/i }))
    expect(screen.getByRole('alert')).toHaveTextContent('Choose a PDF')
    await user.upload(screen.getByLabelText('PDF file'), new File(['x'], 'notes.docx', { type: 'application/msword' }))
    expect(screen.getByRole('alert')).toHaveTextContent('Only PDF files are supported.')
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('submits an existing topic with title defaulted from the filename', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn<(input: UploadInput) => void>()
    render(<UploadForm topics={topics} pending={false} onSubmit={onSubmit} />)
    await user.upload(screen.getByLabelText('PDF file'), pdf())
    await user.selectOptions(screen.getByRole('combobox', { name: 'Questions to generate' }), '20')
    await user.click(screen.getByRole('button', { name: /generate questions/i }))
    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ count: 20, title: 'cells', topic: { kind: 'existing', id: 't1' } }))
  })

  it('requires a name for a new topic', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn<(input: UploadInput) => void>()
    render(<UploadForm topics={[]} pending={false} onSubmit={onSubmit} />)
    await user.upload(screen.getByLabelText('PDF file'), pdf())
    await user.click(screen.getByRole('button', { name: /generate questions/i }))
    expect(screen.getByText('Enter a name for the new topic.')).toBeInTheDocument()
    await user.type(screen.getByRole('textbox', { name: 'New topic name' }), 'Chemistry')
    await user.click(screen.getByRole('button', { name: /generate questions/i }))
    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ topic: { kind: 'new', name: 'Chemistry' } }))
  })
})
