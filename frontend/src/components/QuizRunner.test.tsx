import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { attempt } from '../test/server'
import { QuizRunner, type Responses } from './QuizRunner'

describe('QuizRunner', () => {
  it('uses radios for MCQ/TF, checkboxes for MRQ, and submits responses', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn<(r: Responses) => void>()
    render(<QuizRunner questions={attempt().questions} submitting={false} onSubmit={onSubmit} />)

    expect(screen.getAllByRole('radio')).toHaveLength(4)
    await user.click(screen.getByRole('radio', { name: 'Mitochondria' }))
    await user.click(screen.getByRole('button', { name: 'Next' }))

    expect(screen.getAllByRole('radio')).toHaveLength(2)
    await user.click(screen.getByRole('radio', { name: 'True' }))
    await user.click(screen.getByRole('button', { name: 'Next' }))

    expect(screen.queryByRole('radio')).not.toBeInTheDocument()
    expect(screen.getByText(/wrong picks cancel out/i)).toBeInTheDocument()
    await user.click(screen.getByRole('checkbox', { name: 'Ribosome' }))
    await user.click(screen.getByRole('checkbox', { name: 'Nucleus' }))
    await user.click(screen.getByRole('checkbox', { name: 'Ribosome' })) // untick
    await user.click(screen.getByRole('checkbox', { name: 'Ribosome' })) // tick again
    await user.click(screen.getByRole('button', { name: 'Submit quiz' }))

    const responses = onSubmit.mock.calls[0]?.[0]
    expect(responses === undefined ? undefined : Object.fromEntries(responses)).toEqual({ q1: [1], q2: [0], q3: [0, 2] })
  })

  it('keeps answers when navigating back and warns about unanswered questions', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn<(r: Responses) => void>()
    render(<QuizRunner questions={attempt().questions} submitting={false} onSubmit={onSubmit} />)
    await user.click(screen.getByRole('radio', { name: 'Golgi' }))
    await user.click(screen.getByRole('button', { name: 'Next' }))
    await user.click(screen.getByRole('button', { name: 'Previous' }))
    expect(screen.getByRole('radio', { name: 'Golgi' })).toBeChecked()

    await user.click(screen.getByRole('button', { name: 'Next' }))
    await user.click(screen.getByRole('button', { name: 'Next' }))
    await user.click(screen.getByRole('button', { name: 'Submit quiz' }))
    const dialog = await screen.findByRole('alertdialog')
    expect(dialog).toHaveTextContent('2 unanswered questions')
    await user.click(screen.getByRole('button', { name: 'Keep answering' }))
    expect(onSubmit).not.toHaveBeenCalled()

    await user.click(screen.getByRole('button', { name: 'Submit quiz' }))
    await user.click(await screen.findByRole('button', { name: 'Submit anyway' }))
    expect(onSubmit).toHaveBeenCalledOnce()
  })
})
