<template>
  <div class="glass-panel p-3 h-full flex flex-col relative">
    <h3 class="text-sm font-mono text-purple mb-2 flex justify-between items-center">
      <span>EMBEDDED TERMINAL</span>
      <span class="text-xs" :class="connected ? 'text-neon' : 'text-alert'">
        {{ connected ? '● connected' : '○ disconnected' }}
      </span>
    </h3>
    <div class="flex-1 bg-black rounded p-2 overflow-hidden" ref="terminalContainer"></div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import '@xterm/xterm/css/xterm.css'

const terminalContainer = ref(null)
const connected = ref(false)

let term = null
let fitAddon = null
let ws = null
let resizeObserver = null

onMounted(() => {
  initTerminal()
  connectWebSocket()

  // Auto-resize when container changes
  resizeObserver = new ResizeObserver(() => {
    if (fitAddon) fitAddon.fit()
  })
  resizeObserver.observe(terminalContainer.value)
})

onUnmounted(() => {
  if (resizeObserver) resizeObserver.disconnect()
  if (ws) ws.close()
  if (term) term.dispose()
})

const initTerminal = () => {
  term = new Terminal({
    theme: {
      background: '#000000',
      foreground: '#e5e7eb',
      cursor: '#00ff41',
      selectionBackground: 'rgba(108, 99, 255, 0.3)',
      black: '#0a0a0f',
      red: '#ff4444',
      green: '#00ff41',
      yellow: '#f59e0b',
      blue: '#6c63ff',
      magenta: '#d946ef',
      cyan: '#06b6d4',
      white: '#f3f4f6',
      brightBlack: '#374151',
      brightRed: '#ef4444',
      brightGreen: '#10b981',
      brightYellow: '#fbbf24',
      brightBlue: '#8b5cf6',
      brightMagenta: '#e879f9',
      brightCyan: '#22d3ee',
      brightWhite: '#ffffff'
    },
    fontFamily: '"JetBrains Mono", "Fira Code", monospace',
    fontSize: 13,
    cursorBlink: true,
    cursorStyle: 'block',
    scrollback: 10000,
  })

  fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.loadAddon(new WebLinksAddon())

  term.open(terminalContainer.value)
  fitAddon.fit()

  // Send keystrokes to backend
  term.onData((data) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(data)
    }
  })
}

const connectWebSocket = () => {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  // Use current host to avoid hardcoded ports
  const wsUrl = `${protocol}//${window.location.host}/ws/terminal`
  
  ws = new WebSocket(wsUrl)

  ws.onopen = () => {
    connected.value = true
    term.write('\r\n\x1b[38;5;46m[*] Connected to Vesper Backend Console\x1b[0m\r\n')
  }

  ws.onmessage = (event) => {
    // Parse text and feed to xterm
    term.write(event.data)
  }

  ws.onclose = () => {
    connected.value = false
    term.write('\r\n\x1b[38;5;196m[!] Connection to Backend Console lost. Reconnecting...\x1b[0m\r\n')
    setTimeout(connectWebSocket, 3000)
  }

  ws.onerror = (err) => {
    console.error('Terminal WS Error', err)
    ws.close()
  }
}
</script>

<style scoped>
/* Ensure xterm takes full height and hides native scrollbar cleanly */
:deep(.xterm) {
  height: 100%;
  padding: 4px;
}
:deep(.xterm-viewport) {
  background-color: transparent !important;
}
:deep(.xterm-viewport::-webkit-scrollbar) {
  width: 6px;
}
:deep(.xterm-viewport::-webkit-scrollbar-track) {
  background: transparent;
}
:deep(.xterm-viewport::-webkit-scrollbar-thumb) {
  background: rgba(108, 99, 255, 0.3);
  border-radius: 3px;
}
:deep(.xterm-viewport::-webkit-scrollbar-thumb:hover) {
  background: rgba(108, 99, 255, 0.6);
}
</style>
