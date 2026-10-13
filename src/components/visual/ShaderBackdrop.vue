<script setup lang="ts">
/**
 * ShaderBackdrop — a bespoke WebGL2 aurora field.
 *
 * Deliberately dependency-free: hand-written GLSL, a full-screen triangle and
 * no buffers at all (gl_VertexID). Renders one frame and idles when the user
 * prefers reduced motion, when the tab is hidden, or when scrolled out of view.
 * Falls back to a CSS gradient whenever WebGL2 is unavailable.
 */
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useTheme } from '@/composables/useTheme'

const props = withDefaults(
  defineProps<{
    /** 0..1 — scales the aurora brightness. */
    intensity?: number
    /** Freeze on a single frame (e.g. for a small inline variant). */
    still?: boolean
  }>(),
  { intensity: 1, still: false },
)

const canvas = ref<HTMLCanvasElement | null>(null)
const failed = ref(false)

const VERT = `#version 300 es
const vec2 VERTS[3] = vec2[3](vec2(-1.0, -1.0), vec2(3.0, -1.0), vec2(-1.0, 3.0));
void main() {
  gl_Position = vec4(VERTS[gl_VertexID], 0.0, 1.0);
}`

const FRAG = `#version 300 es
precision highp float;

uniform vec2  uRes;
uniform float uTime;
uniform vec3  uBase;
uniform vec3  uAccent;
uniform vec3  uAccent2;
uniform float uIntensity;

out vec4 fragColor;

float hash(vec2 p) {
  p = fract(p * vec2(123.34, 456.21));
  p += dot(p, p + 45.32);
  return fract(p.x * p.y);
}

float noise(vec2 p) {
  vec2 i = floor(p);
  vec2 f = fract(p);
  vec2 u = f * f * (3.0 - 2.0 * f);
  float a = hash(i);
  float b = hash(i + vec2(1.0, 0.0));
  float c = hash(i + vec2(0.0, 1.0));
  float d = hash(i + vec2(1.0, 1.0));
  return mix(mix(a, b, u.x), mix(c, d, u.x), u.y);
}

float fbm(vec2 p) {
  float v = 0.0;
  float amp = 0.5;
  for (int i = 0; i < 5; i++) {
    v += amp * noise(p);
    p *= 2.02;
    amp *= 0.5;
  }
  return v;
}

void main() {
  vec2 uv = gl_FragCoord.xy / uRes;
  vec2 p  = (gl_FragCoord.xy - 0.5 * uRes) / max(1.0, min(uRes.x, uRes.y));

  float t = uTime * 0.055;

  float n1 = fbm(p * 1.55 + vec2(t, -t * 0.7));
  float n2 = fbm(p * 2.30 - vec2(t * 0.8, t * 0.5) + n1 * 0.65);

  float band  = smoothstep(0.34, 0.88, n1 * 0.70 + n2 * 0.50);
  float band2 = smoothstep(0.56, 0.96, n2);

  vec3 col = uBase;
  col += uAccent  * band  * 0.60 * uIntensity;
  col += uAccent2 * band2 * 0.38 * uIntensity;

  // Keep the lower half quiet so headlines stay readable.
  float fall = smoothstep(1.10, 0.05, uv.y);
  col = mix(uBase, col, fall);

  // Vignette
  float vig = smoothstep(1.35, 0.30, length(p));
  col *= mix(0.50, 1.0, vig);

  // Film grain
  col += (hash(gl_FragCoord.xy + fract(uTime) * 137.0) - 0.5) * 0.032;

  // Ordered dither kills 8-bit banding in the wide gradients.
  float d = fract(sin(dot(gl_FragCoord.xy, vec2(12.9898, 78.233))) * 43758.5453);
  col += (d - 0.5) / 255.0;

  fragColor = vec4(col, 1.0);
}`

let gl: WebGL2RenderingContext | null = null
let program: WebGLProgram | null = null
let uniforms: Record<string, WebGLUniformLocation | null> = {}
let raf = 0
let running = false
let visible = true
let startTime = 0
let resizeObserver: ResizeObserver | null = null
let intersectionObserver: IntersectionObserver | null = null

const { effective } = useTheme()

function compile(glc: WebGL2RenderingContext, type: number, source: string): WebGLShader | null {
  const shader = glc.createShader(type)
  if (!shader) return null
  glc.shaderSource(shader, source)
  glc.compileShader(shader)
  if (!glc.getShaderParameter(shader, glc.COMPILE_STATUS)) {
    console.error('[ShaderBackdrop]', glc.getShaderInfoLog(shader))
    glc.deleteShader(shader)
    return null
  }
  return shader
}

function palette(): { base: [number, number, number]; a: [number, number, number]; b: [number, number, number] } {
  // Light mode gets a much softer field so the page still reads as light.
  if (effective.value === 'light') {
    return { base: [0.965, 0.965, 0.973], a: [0.925, 0.286, 0.600], b: [0.396, 0.4, 0.945] }
  }
  return { base: [0.035, 0.035, 0.043], a: [0.925, 0.286, 0.600], b: [0.545, 0.361, 0.965] }
}

function resize(): void {
  const el = canvas.value
  if (!el || !gl) return
  const dpr = Math.min(window.devicePixelRatio || 1, 2)
  const w = Math.max(1, Math.floor(el.clientWidth * dpr))
  const h = Math.max(1, Math.floor(el.clientHeight * dpr))
  if (el.width === w && el.height === h) return
  el.width = w
  el.height = h
  gl.viewport(0, 0, w, h)
  if (!running) drawFrame(0)
}

let elapsed = 0

function drawFrame(time: number): void {
  if (!gl || !program) return
  const el = canvas.value
  if (!el) return

  gl.useProgram(program)
  gl.uniform2f(uniforms.uRes ?? null, el.width, el.height)
  gl.uniform1f(uniforms.uTime ?? null, time)
  gl.uniform1f(uniforms.uIntensity ?? null, props.intensity)

  const p = palette()
  gl.uniform3f(uniforms.uBase ?? null, p.base[0], p.base[1], p.base[2])
  gl.uniform3f(uniforms.uAccent ?? null, p.a[0], p.a[1], p.a[2])
  gl.uniform3f(uniforms.uAccent2 ?? null, p.b[0], p.b[1], p.b[2])

  gl.drawArrays(gl.TRIANGLES, 0, 3)
}

function loop(now: number): void {
  if (!running) return
  if (startTime === 0) startTime = now
  elapsed = (now - startTime) / 1000
  drawFrame(elapsed)
  raf = requestAnimationFrame(loop)
}

function start(): void {
  if (running || props.still) return
  if (!visible || document.hidden) return
  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return
  running = true
  raf = requestAnimationFrame(loop)
  canvas.value?.classList.add('is-running')
}

function stop(): void {
  running = false
  cancelAnimationFrame(raf)
  raf = 0
  canvas.value?.classList.remove('is-running')
}

function onVisibility(): void {
  if (document.hidden) stop()
  else start()
}

function init(): void {
  const el = canvas.value
  if (!el) return

  gl = el.getContext('webgl2', {
    alpha: false,
    antialias: false,
    depth: false,
    stencil: false,
    powerPreference: 'low-power',
    preserveDrawingBuffer: false,
  })

  if (!gl) {
    failed.value = true
    return
  }

  const vs = compile(gl, gl.VERTEX_SHADER, VERT)
  const fs = compile(gl, gl.FRAGMENT_SHADER, FRAG)
  if (!vs || !fs) {
    failed.value = true
    return
  }

  program = gl.createProgram()
  if (!program) {
    failed.value = true
    return
  }
  gl.attachShader(program, vs)
  gl.attachShader(program, fs)
  gl.linkProgram(program)
  if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
    console.error('[ShaderBackdrop]', gl.getProgramInfoLog(program))
    failed.value = true
    return
  }

  gl.deleteShader(vs)
  gl.deleteShader(fs)

  uniforms = {
    uRes: gl.getUniformLocation(program, 'uRes'),
    uTime: gl.getUniformLocation(program, 'uTime'),
    uBase: gl.getUniformLocation(program, 'uBase'),
    uAccent: gl.getUniformLocation(program, 'uAccent'),
    uAccent2: gl.getUniformLocation(program, 'uAccent2'),
    uIntensity: gl.getUniformLocation(program, 'uIntensity'),
  }

  resizeObserver = new ResizeObserver(() => resize())
  resizeObserver.observe(el)

  intersectionObserver = new IntersectionObserver(
    (entries) => {
      visible = entries.some((entry) => entry.isIntersecting)
      if (visible) start()
      else stop()
    },
    { threshold: 0 },
  )
  intersectionObserver.observe(el)

  document.addEventListener('visibilitychange', onVisibility)
  resize()
  drawFrame(0)
  start()
}

onMounted(init)

// Repaint on theme change so the palette follows instantly.
watch(effective, () => {
  if (!running) drawFrame(elapsed)
})

onBeforeUnmount(() => {
  stop()
  resizeObserver?.disconnect()
  intersectionObserver?.disconnect()
  document.removeEventListener('visibilitychange', onVisibility)
  if (gl && program) gl.deleteProgram(program)
  gl = null
  program = null
})
</script>

<template>
  <div class="backdrop" :class="{ 'is-failed': failed }" aria-hidden="true">
    <canvas ref="canvas" class="backdrop__canvas" />
  </div>
</template>

<style scoped>
.backdrop {
  position: absolute;
  inset: 0;
  z-index: 0;
  overflow: hidden;
  pointer-events: none;
  /* CSS fallback — also the base while WebGL warms up. */
  background:
    radial-gradient(120% 90% at 15% 0%, var(--accent-subtle) 0%, transparent 55%),
    radial-gradient(90% 70% at 85% 10%, rgb(139 92 246 / 22%) 0%, transparent 60%),
    var(--bg-base);
}

.backdrop__canvas {
  width: 100%;
  height: 100%;
  opacity: 0;
  transition: opacity var(--dur-slower) var(--ease-out);
}

.backdrop__canvas.is-running,
.backdrop.is-failed .backdrop__canvas {
  opacity: 1;
}

.backdrop.is-failed .backdrop__canvas {
  display: none;
}
</style>
