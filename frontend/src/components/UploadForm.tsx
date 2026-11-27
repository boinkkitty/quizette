import { FileUpIcon } from 'lucide-react'
import { type DragEvent, type FormEvent, useState } from 'react'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { cn } from '@/lib/utils'

import type { UploadInput } from '../api/queries'
import type { Topic } from '../api/schemas'
import { countOptions, formatBytes } from '../lib/format'
import { MAX_BANK_SIZE, validateFile } from '../lib/upload'
import { Field } from './Field'
import { Spinner } from './Spinner'

const NEW_TOPIC = '__new__'

interface Props {
  topics: readonly Topic[]
  defaultTopicId?: string | undefined
  pending: boolean
  onSubmit: (input: UploadInput) => void
}

export function UploadForm({ topics, defaultTopicId, pending, onSubmit }: Props) {
  const initialTopic =
    defaultTopicId !== undefined && topics.some((t) => t.id === defaultTopicId)
      ? defaultTopicId
      : (topics[0]?.id ?? NEW_TOPIC)
  const [topicChoice, setTopicChoice] = useState(initialTopic)
  const [newTopic, setNewTopic] = useState('')
  const [title, setTitle] = useState('')
  const [count, setCount] = useState(10)
  const [file, setFile] = useState<File | null>(null)
  const [fileError, setFileError] = useState<string | undefined>()
  const [topicError, setTopicError] = useState<string | undefined>()
  const [dragging, setDragging] = useState(false)

  const pickFile = (f: File | undefined) => {
    if (f === undefined) return
    const err = validateFile(f)
    setFileError(err)
    setFile(err === undefined ? f : null)
    if (err === undefined && title === '') setTitle(f.name.replace(/\.pdf$/i, ''))
  }

  const onDrop = (e: DragEvent<HTMLLabelElement>) => {
    e.preventDefault()
    setDragging(false)
    pickFile(e.dataTransfer.files[0])
  }

  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (file === null) {
      setFileError('Choose a PDF to upload.')
      return
    }
    if (topicChoice === NEW_TOPIC && newTopic.trim() === '') {
      setTopicError('Enter a name for the new topic.')
      return
    }
    setTopicError(undefined)
    onSubmit({
      file,
      count,
      title: title.trim(),
      topic:
        topicChoice === NEW_TOPIC
          ? { kind: 'new', name: newTopic.trim() }
          : { kind: 'existing', id: topicChoice },
    })
  }

  return (
    <form onSubmit={submit} className="space-y-5" noValidate>
      <label
        onDragOver={(e) => {
          e.preventDefault()
          setDragging(true)
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={onDrop}
        className={cn(
          'flex cursor-pointer flex-col items-center justify-center rounded-xl border-2 border-dashed px-6 py-10 text-center transition-colors',
          dragging
            ? 'border-primary bg-secondary'
            : 'border-border hover:border-primary hover:bg-secondary/50',
        )}
      >
        <FileUpIcon className="mb-3 size-8 text-primary" aria-hidden />
        <input
          type="file"
          accept="application/pdf,.pdf"
          className="sr-only"
          aria-label="PDF file"
          onChange={(e) => pickFile(e.target.files?.[0])}
        />
        {file === null ? (
          <>
            <span className="font-medium">Drop a PDF here or click to browse</span>
            <span className="mt-1 text-xs text-muted-foreground">
              Up to 15 MB. The file is only read to generate questions — it is never stored.
            </span>
          </>
        ) : (
          <>
            <span className="font-medium">{file.name}</span>
            <span className="mt-1 text-xs text-muted-foreground">
              {formatBytes(file.size)} · click to change
            </span>
          </>
        )}
      </label>
      {fileError !== undefined && (
        <p role="alert" className="text-sm text-destructive">
          {fileError}
        </p>
      )}

      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Topic">
          {(c) => (
            <NativeSelect
              className="w-full"
              {...c}
              value={topicChoice}
              onChange={(e) => setTopicChoice(e.target.value)}
            >
              {topics.map((t) => (
                <NativeSelectOption key={t.id} value={t.id}>
                  {t.name}
                </NativeSelectOption>
              ))}
              <NativeSelectOption value={NEW_TOPIC}>+ New topic…</NativeSelectOption>
            </NativeSelect>
          )}
        </Field>
        {topicChoice === NEW_TOPIC && (
          <Field label="New topic name" error={topicError}>
            {(c) => (
              <Input
                {...c}
                value={newTopic}
                onChange={(e) => setNewTopic(e.target.value)}
                placeholder="e.g. Biology"
                maxLength={120}
              />
            )}
          </Field>
        )}
        <Field label="Chapter title" hint="Defaults to the file name">
          {(c) => (
            <Input
              {...c}
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              maxLength={120}
            />
          )}
        </Field>
        <Field label="Questions to generate" hint="The bank you'll practice from">
          {(c) => (
            <NativeSelect
              className="w-full"
              {...c}
              value={count}
              onChange={(e) => setCount(Number(e.target.value))}
            >
              {countOptions(MAX_BANK_SIZE).map((n) => (
                <NativeSelectOption key={n} value={n}>
                  {n}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          )}
        </Field>
      </div>

      <Button type="submit" size="lg" disabled={pending} className="w-full sm:w-auto">
        {pending && <Spinner />} Generate questions
      </Button>
    </form>
  )
}
