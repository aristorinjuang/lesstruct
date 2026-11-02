import { describe, it, expect, vi } from 'vitest'
import {
  SPEECH_MAX_SECONDS,
  SPEECH_SAMPLE_RATE,
  downmixToMono,
  encodeWav,
  floatTo16BitPCM,
  mapMicrophoneError,
  normalizeCaptureRate,
  resampleLinear,
} from './audio'

async function blobBytes(blob: Blob): Promise<Uint8Array> {
  return new Uint8Array(await blob.arrayBuffer())
}

describe('floatTo16BitPCM', () => {
  it('maps silence to zero', () => {
    expect(Array.from(floatTo16BitPCM(new Float32Array([0, 0])))).toEqual([0, 0])
  })

  it('clips out-of-range samples', () => {
    expect(Array.from(floatTo16BitPCM(new Float32Array([-2, 2])))).toEqual([-32768, 32767])
  })

  it('scales full-scale samples', () => {
    expect(Array.from(floatTo16BitPCM(new Float32Array([-1, 1])))).toEqual([-32768, 32767])
  })
})

describe('resampleLinear', () => {
  it('returns a copy when rates match', () => {
    const input = new Float32Array([0.1, 0.2])
    const output = resampleLinear(input, 16000, 16000)
    expect(output[0]).toBeCloseTo(0.1, 5)
    expect(output[1]).toBeCloseTo(0.2, 5)
    expect(output).not.toBe(input)
  })

  it('downsamples 48 kHz to 16 kHz', () => {
    const input = new Float32Array([0, 0.5, 1, 0.5, 0, -0.5, -1, -0.5, 0])
    const output = resampleLinear(input, 48000, 16000)
    expect(output.length).toBe(3)
    expect(output[0]).toBeCloseTo(0, 5)
  })

  it('upsamples 16 kHz to 24 kHz', () => {
    const input = new Float32Array([0, 1])
    const output = resampleLinear(input, 16000, 24000)
    expect(output.length).toBe(3)
    expect(output[0]).toBeCloseTo(0, 5)
    expect(output[2]).toBeCloseTo(1, 5)
  })
})

describe('downmixToMono', () => {
  it('returns empty output for no channels', () => {
    expect(downmixToMono([]).length).toBe(0)
  })

  it('copies a single channel', () => {
    const input = new Float32Array([0.1, 0.2])
    const output = downmixToMono([input])
    expect(output[0]).toBeCloseTo(0.1, 5)
    expect(output[1]).toBeCloseTo(0.2, 5)
    expect(output).not.toBe(input)
  })

  it('averages stereo channels', () => {
    const output = downmixToMono([new Float32Array([1, -1]), new Float32Array([-1, 1])])
    expect(Array.from(output)).toEqual([0, 0])
  })
})

describe('encodeWav', () => {
  it('writes a valid mono 16-bit PCM header', async () => {
    const blob = encodeWav(new Float32Array([0, 0.5, -0.5]), SPEECH_SAMPLE_RATE)
    expect(blob.type).toBe('audio/wav')
    const bytes = await blobBytes(blob)
    const ascii = (offset: number, length: number) =>
      String.fromCharCode(...bytes.slice(offset, offset + length))
    expect(ascii(0, 4)).toBe('RIFF')
    expect(ascii(8, 4)).toBe('WAVE')
    expect(ascii(12, 4)).toBe('fmt ')
    expect(ascii(36, 4)).toBe('data')

    const view = new DataView(bytes.buffer)
    expect(view.getUint16(20, true)).toBe(1)
    expect(view.getUint16(22, true)).toBe(1)
    expect(view.getUint32(24, true)).toBe(SPEECH_SAMPLE_RATE)
    expect(view.getUint16(34, true)).toBe(16)
    expect(view.getUint32(40, true)).toBe(6)
    expect(bytes.length).toBe(44 + 6)
  })
})

describe('normalizeCaptureRate', () => {
  it('keeps supported rates untouched', () => {
    expect(normalizeCaptureRate(16000)).toBe(16000)
    expect(normalizeCaptureRate(24000)).toBe(24000)
  })

  it('falls back to 16 kHz for anything else', () => {
    expect(normalizeCaptureRate(48000)).toBe(SPEECH_SAMPLE_RATE)
    expect(normalizeCaptureRate(44100)).toBe(SPEECH_SAMPLE_RATE)
  })
})

describe('mapMicrophoneError', () => {
  it('explains denied access', () => {
    expect(mapMicrophoneError(new DOMException('denied', 'NotAllowedError'))).toContain('denied')
  })

  it('explains a missing microphone', () => {
    expect(mapMicrophoneError(new DOMException('missing', 'NotFoundError'))).toContain('No microphone found')
  })

  it('explains a busy microphone', () => {
    expect(mapMicrophoneError(new DOMException('busy', 'NotReadableError'))).toContain('busy')
  })

  it('falls back to a generic message', () => {
    vi.stubGlobal('navigator', { mediaDevices: { getUserMedia: async () => ({}) } })
    try {
      expect(mapMicrophoneError(new Error('boom'))).toContain('Could not start recording')
    } finally {
      vi.unstubAllGlobals()
    }
  })

  it('requires a secure connection when the media API is missing', () => {
    vi.stubGlobal('navigator', {})
    try {
      expect(mapMicrophoneError(new Error('boom'))).toContain('secure connection')
    } finally {
      vi.unstubAllGlobals()
    }
  })
})

describe('speech capture budget', () => {
  it('caps dictation at five minutes', () => {
    expect(SPEECH_MAX_SECONDS).toBe(300)
  })
})
