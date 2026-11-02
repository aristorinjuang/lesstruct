export const SPEECH_SAMPLE_RATE = 16000
export const SPEECH_SUPPORTED_RATES = [16000, 24000]
export const SPEECH_MAX_SECONDS = 300

function writeAscii(view: DataView, offset: number, text: string): void {
  for (let i = 0; i < text.length; i++) {
    view.setUint8(offset + i, text.charCodeAt(i))
  }
}

export function floatTo16BitPCM(samples: Float32Array): Int16Array {
  const pcm = new Int16Array(samples.length)
  for (let i = 0; i < samples.length; i++) {
    const clamped = Math.max(-1, Math.min(1, samples[i] ?? 0))
    pcm[i] = clamped < 0 ? clamped * 0x8000 : clamped * 0x7fff
  }
  return pcm
}

export function resampleLinear(samples: Float32Array, fromRate: number, toRate: number): Float32Array {
  if (fromRate === toRate || samples.length === 0) {
    return samples.slice()
  }
  const ratio = fromRate / toRate
  const targetLength = Math.max(1, Math.floor(samples.length / ratio))
  const resampled = new Float32Array(targetLength)
  for (let i = 0; i < targetLength; i++) {
    const position = i * ratio
    const before = Math.floor(position)
    const after = Math.min(before + 1, samples.length - 1)
    const fraction = position - before
    resampled[i] = (samples[before] ?? 0) * (1 - fraction) + (samples[after] ?? 0) * fraction
  }
  return resampled
}

export function downmixToMono(channels: Float32Array[]): Float32Array {
  if (channels.length === 0) {
    return new Float32Array(0)
  }
  if (channels.length === 1) {
    return (channels[0] ?? new Float32Array(0)).slice()
  }
  let length = channels[0]?.length ?? 0
  for (const channel of channels) {
    length = Math.min(length, channel.length)
  }
  const mono = new Float32Array(length)
  for (let i = 0; i < length; i++) {
    let sum = 0
    for (const channel of channels) {
      sum += channel[i] ?? 0
    }
    mono[i] = sum / channels.length
  }
  return mono
}

export function encodeWav(samples: Float32Array, sampleRate: number): Blob {
  const pcm = floatTo16BitPCM(samples)
  const buffer = new ArrayBuffer(44 + pcm.length * 2)
  const view = new DataView(buffer)

  writeAscii(view, 0, 'RIFF')
  view.setUint32(4, 36 + pcm.length * 2, true)
  writeAscii(view, 8, 'WAVE')
  writeAscii(view, 12, 'fmt ')
  view.setUint32(16, 16, true)
  view.setUint16(20, 1, true)
  view.setUint16(22, 1, true)
  view.setUint32(24, sampleRate, true)
  view.setUint32(28, sampleRate * 2, true)
  view.setUint16(32, 2, true)
  view.setUint16(34, 16, true)
  writeAscii(view, 36, 'data')
  view.setUint32(40, pcm.length * 2, true)

  for (let i = 0; i < pcm.length; i++) {
    view.setInt16(44 + i * 2, pcm[i] ?? 0, true)
  }

  return new Blob([buffer], { type: 'audio/wav' })
}

export function normalizeCaptureRate(actualRate: number): number {
  if (SPEECH_SUPPORTED_RATES.includes(actualRate)) {
    return actualRate
  }
  return SPEECH_SAMPLE_RATE
}

export function mapMicrophoneError(err: unknown): string {
  const name = err instanceof DOMException ? err.name : ''
  if (name === 'NotAllowedError' || name === 'SecurityError') {
    return 'Microphone access was denied. Allow microphone access and try again.'
  }
  if (name === 'NotFoundError' || name === 'OverconstrainedError') {
    return 'No microphone found. Connect a microphone and try again.'
  }
  if (name === 'NotReadableError' || name === 'AbortError') {
    return 'The microphone is busy. Close other apps using it and try again.'
  }
  if (typeof navigator !== 'undefined' && !navigator.mediaDevices?.getUserMedia) {
    return 'Microphone requires a secure connection (HTTPS or localhost).'
  }
  return 'Could not start recording. Check your microphone and try again.'
}
