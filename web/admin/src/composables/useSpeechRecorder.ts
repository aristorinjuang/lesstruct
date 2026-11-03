import { onBeforeUnmount, ref } from 'vue'
import {
  SPEECH_MAX_SECONDS,
  SPEECH_SAMPLE_RATE,
  downmixToMono,
  encodeWav,
  mapMicrophoneError,
  normalizeCaptureRate,
  resampleLinear,
} from '@/utils/audio'

export type SpeechRecorderStatus = 'idle' | 'recording'

export interface UseSpeechRecorderOptions {
  maxSeconds?: number
  onAutoStop?: (audio: Blob) => void
}

// Inline AudioWorklet processor: forwards a copy of every input block to the
// main thread. Shipped as a blob URL so no bundler worklet config is needed.
const RECORDER_WORKLET = `
class LesstructRecorderProcessor extends AudioWorkletProcessor {
  process(inputs) {
    const channels = inputs[0] || []
    if (channels.length > 0) {
      this.port.postMessage(channels.map((channel) => channel.slice(0)))
    }
    return true
  }
}
registerProcessor('lesstruct-recorder', LesstructRecorderProcessor)
`

function getAudioContextConstructor(): typeof AudioContext | null {
  if (typeof window === 'undefined') {
    return null
  }
  return (
    window.AudioContext ??
    (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext ??
    null
  )
}

export function useSpeechRecorder(options: UseSpeechRecorderOptions = {}) {
  const maxSeconds = options.maxSeconds ?? SPEECH_MAX_SECONDS
  const status = ref<SpeechRecorderStatus>('idle')
  const elapsedSeconds = ref(0)
  const error = ref<string | null>(null)

  let stream: MediaStream | null = null
  let audioContext: AudioContext | null = null
  let sourceNode: MediaStreamAudioSourceNode | null = null
  let workletNode: AudioWorkletNode | null = null
  let scriptNode: ScriptProcessorNode | null = null
  let workletUrl: string | null = null
  let timer: ReturnType<typeof setInterval> | null = null
  let chunks: Float32Array[] = []
  let channelCount = 1
  let finishing = false

  function clearTimer() {
    if (timer !== null) {
      clearInterval(timer)
      timer = null
    }
  }

  function teardown() {
    clearTimer()
    if (workletNode) {
      workletNode.port.onmessage = null
      workletNode.disconnect()
      workletNode = null
    }
    if (scriptNode) {
      scriptNode.onaudioprocess = null
      scriptNode.disconnect()
      scriptNode = null
    }
    if (sourceNode) {
      sourceNode.disconnect()
      sourceNode = null
    }
    if (stream) {
      for (const track of stream.getTracks()) {
        track.stop()
      }
      stream = null
    }
    if (workletUrl) {
      URL.revokeObjectURL(workletUrl)
      workletUrl = null
    }
    const context = audioContext
    audioContext = null
    if (context && context.state !== 'closed') {
      void context.close().catch(() => {
        // Teardown is best-effort; a closing context rejects safely ignored.
      })
    }
  }

  function pushChunk(data: Float32Array[]) {
    if (data.length === 0) {
      return
    }
    channelCount = Math.max(channelCount, data.length)
    // Regroup per-channel blocks into one mono block each — the worklet
    // posts all channels of one quantum together.
    if (data.length === 1) {
      const first = data[0]
      if (first) {
        chunks.push(first)
      }
      return
    }
    const length = Math.min(...data.map((channel) => channel.length))
    const mono = new Float32Array(length)
    for (let i = 0; i < length; i++) {
      let sum = 0
      for (const channel of data) {
        sum += channel[i] ?? 0
      }
      mono[i] = sum / data.length
    }
    chunks.push(mono)
  }

  function assembleWav(): Blob {
    const total = chunks.reduce((sum, chunk) => sum + chunk.length, 0)
    const flat = new Float32Array(total)
    let offset = 0
    for (const chunk of chunks) {
      flat.set(chunk, offset)
      offset += chunk.length
    }
    chunks = []
    const actualRate = audioContext?.sampleRate ?? SPEECH_SAMPLE_RATE
    const targetRate = normalizeCaptureRate(actualRate)
    const samples = targetRate === actualRate ? flat : resampleLinear(flat, actualRate, targetRate)
    return encodeWav(samples, targetRate)
  }

  function finishRecording(): Blob {
    const audio = assembleWav()
    teardown()
    status.value = 'idle'
    elapsedSeconds.value = 0
    return audio
  }

  function handleAutoStop() {
    if (finishing || status.value !== 'recording') {
      return
    }
    finishing = true
    try {
      const audio = finishRecording()
      options.onAutoStop?.(audio)
    } finally {
      finishing = false
    }
  }

  function attachScriptProcessorFallback(context: AudioContext) {
    // Deprecated but universal: covers browsers without AudioWorklet.
    // The zero-gain sink keeps the processor running without audible output.
    scriptNode = context.createScriptProcessor(4096, channelCount, 1)
    const sink = context.createGain()
    sink.gain.value = 0
    scriptNode.onaudioprocess = (event) => {
      const input = event.inputBuffer
      const data: Float32Array[] = []
      for (let i = 0; i < input.numberOfChannels; i++) {
        data.push(input.getChannelData(i).slice(0))
      }
      const mono = downmixToMono(data)
      if (mono.length > 0) {
        pushChunk([mono])
      }
    }
    sourceNode?.connect(scriptNode)
    scriptNode.connect(sink)
    sink.connect(context.destination)
  }

  async function startRecording(): Promise<void> {
    if (status.value === 'recording') {
      return
    }
    error.value = null
    channelCount = 1
    chunks = []

    const AudioContextCtor = getAudioContextConstructor()
    if (!AudioContextCtor) {
      const message = 'Audio recording is not supported in this browser.'
      error.value = message
      throw new Error(message)
    }

    try {
      stream = await navigator.mediaDevices.getUserMedia({
        audio: {
          channelCount: 1,
          sampleRate: SPEECH_SAMPLE_RATE,
          echoCancellation: true,
          noiseSuppression: true,
        },
      })
    } catch (err) {
      const message = mapMicrophoneError(err)
      error.value = message
      throw new Error(message)
    }

    try {
      audioContext = new AudioContextCtor({ sampleRate: SPEECH_SAMPLE_RATE })
      if (audioContext.state === 'suspended') {
        await audioContext.resume()
      }
      sourceNode = audioContext.createMediaStreamSource(stream)

      let workletReady = false
      try {
        if (audioContext.audioWorklet) {
          workletUrl = URL.createObjectURL(new Blob([RECORDER_WORKLET], { type: 'application/javascript' }))
          await audioContext.audioWorklet.addModule(workletUrl)
          workletNode = new AudioWorkletNode(audioContext, 'lesstruct-recorder')
          workletNode.port.onmessage = (event) => {
            pushChunk(event.data as Float32Array[])
          }
          sourceNode.connect(workletNode)
          workletReady = true
        }
      } catch {
        workletNode?.disconnect()
        workletNode = null
        if (workletUrl) {
          URL.revokeObjectURL(workletUrl)
          workletUrl = null
        }
      }

      if (!workletReady) {
        attachScriptProcessorFallback(audioContext)
      }

      status.value = 'recording'
      elapsedSeconds.value = 0
      timer = setInterval(() => {
        elapsedSeconds.value += 1
        if (elapsedSeconds.value >= maxSeconds) {
          handleAutoStop()
        }
      }, 1000)
    } catch (err) {
      teardown()
      const message = err instanceof Error ? err.message : mapMicrophoneError(err)
      error.value = message
      throw new Error(message)
    }
  }

  async function stopRecording(): Promise<Blob> {
    if (status.value !== 'recording') {
      throw new Error('Not recording.')
    }
    return finishRecording()
  }

  function cancelRecording(): void {
    chunks = []
    teardown()
    status.value = 'idle'
    elapsedSeconds.value = 0
    error.value = null
  }

  onBeforeUnmount(() => {
    if (status.value === 'recording') {
      cancelRecording()
    } else {
      teardown()
    }
  })

  return {
    status,
    elapsedSeconds,
    error,
    maxSeconds,
    startRecording,
    stopRecording,
    cancelRecording,
  }
}
