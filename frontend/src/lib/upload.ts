export const MAX_UPLOAD_BYTES = 15 * 1024 * 1024

/** Largest question bank a user can request; mirrors domain.MaxSetSize in the backend. */
export const MAX_BANK_SIZE = 20

/** Returns an error message, or undefined when the file is acceptable. */
export function validateFile(file: File): string | undefined {
  const isPdf = file.type === 'application/pdf' || file.name.toLowerCase().endsWith('.pdf')
  if (!isPdf) return 'Only PDF files are supported.'
  if (file.size === 0) return 'This file is empty.'
  if (file.size > MAX_UPLOAD_BYTES) return 'PDFs must be 15 MB or smaller.'
  return undefined
}
