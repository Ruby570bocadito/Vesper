<template>
  <div class="glass-panel p-4 h-full flex flex-col gap-4 font-mono">
    <!-- Header -->
    <div class="flex items-center justify-between border-b border-gray-800 pb-3">
      <h2 class="text-lg text-purple font-bold">Vesper / PAYLOAD BUILDER</h2>
      <span class="text-xs text-gray-500">MSFVenom / Golang Compiler Wrapper</span>
    </div>

    <div class="flex flex-1 gap-4 overflow-hidden">
      <!-- Configuration Form -->
      <div class="w-1/3 flex flex-col gap-3 overflow-y-auto pr-2">
        <!-- Target OS -->
        <div class="form-group">
          <label class="block text-gray-500 text-xs mb-1">Target OS</label>
          <div class="flex gap-2">
            <button v-for="os in ['Windows', 'Linux', 'macOS']" :key="os"
              @click="config.os = os"
              class="flex-1 py-1 px-2 text-xs border border-gray-800 rounded transition-colors"
              :class="config.os === os ? 'bg-purple/20 text-purple border-purple' : 'text-gray-400 hover:border-gray-600'">
              {{ os }}
            </button>
          </div>
        </div>

        <!-- Architecture -->
        <div class="form-group">
          <label class="block text-gray-500 text-xs mb-1">Architecture</label>
          <select v-model="config.arch" class="w-full bg-dark border border-gray-800 rounded px-2 py-1.5 text-xs text-gray-300 focus:border-purple focus:outline-none">
            <option value="x64">x64 (64-bit)</option>
            <option value="x86">x86 (32-bit)</option>
            <option value="arm64">ARM64</option>
          </select>
        </div>

        <!-- Format -->
        <div class="form-group">
          <label class="block text-gray-500 text-xs mb-1">Payload Format / Type</label>
          <select v-model="config.format" class="w-full bg-dark border border-gray-800 rounded px-2 py-1.5 text-xs text-gray-300 focus:border-purple focus:outline-none">
            <optgroup label="Standard Shells">
              <option value="exe" v-if="config.os === 'Windows'">Executable (.exe)</option>
              <option value="dll" v-if="config.os === 'Windows'">Dynamic Link Library (.dll)</option>
              <option value="ps1" v-if="config.os === 'Windows'">PowerShell Script (.ps1)</option>
              <option value="elf" v-if="config.os === 'Linux'">ELF Binary</option>
              <option value="sh" v-if="config.os === 'Linux' || config.os === 'macOS'">Shell Script (.sh)</option>
              <option value="macho" v-if="config.os === 'macOS'">Mach-O Binary</option>
              <option value="shellcode">Raw Shellcode (.bin)</option>
              <option value="python">Python Script (.py)</option>
            </optgroup>
            <optgroup label="Advanced Implants">
              <option value="ransomware">Ransomware (Hybrid Crypto)</option>
              <option value="worm">Network Worm (Self-Propagating)</option>
              <option value="keylogger">Keylogger / Stealer</option>
            </optgroup>
          </select>
        </div>

        <!-- Listener Configuration -->
        <div class="p-3 border border-gray-800 rounded bg-black/30 space-y-3">
          <h3 class="text-xs text-neon uppercase tracking-wider mb-2">Listener Config (C2)</h3>
          <div>
            <label class="block text-gray-500 text-[10px] mb-1">LHOST (IP or Domain)</label>
            <input v-model="config.lhost" type="text" class="w-full bg-dark border border-gray-800 rounded px-2 py-1 text-xs text-gray-300 focus:border-neon focus:outline-none" placeholder="10.0.0.5 or c2.domain.com" />
          </div>
          <div>
            <label class="block text-gray-500 text-[10px] mb-1">LPORT</label>
            <input v-model="config.lport" type="number" class="w-full bg-dark border border-gray-800 rounded px-2 py-1 text-xs text-gray-300 focus:border-neon focus:outline-none" placeholder="8443" />
          </div>
        </div>

        <!-- Evasion / Obfuscation -->
        <div class="p-3 border border-gray-800 rounded bg-black/30 space-y-2">
          <h3 class="text-xs text-orange uppercase tracking-wider mb-2">Evasion & Obfuscation</h3>
          <label class="flex items-center gap-2 cursor-pointer text-xs text-gray-400">
            <input type="checkbox" v-model="config.amsi" class="accent-orange" :disabled="config.os !== 'Windows'" />
            AMSI / ETW Bypass
          </label>
          <label class="flex items-center gap-2 cursor-pointer text-xs text-gray-400">
            <input type="checkbox" v-model="config.unhook" class="accent-orange" :disabled="config.os !== 'Windows'" />
            EDR Unhooking (Direct Syscalls)
          </label>
          <div class="pt-2">
            <label class="block text-gray-500 text-[10px] mb-1">Obfuscation Encoder</label>
            <select v-model="config.encoder" class="w-full bg-dark border border-gray-800 rounded px-2 py-1 text-xs text-gray-300 focus:border-orange focus:outline-none">
              <option value="none">None (Raw)</option>
              <option value="shikata_ga_nai">Shikata Ga Nai (Polymorphic XOR)</option>
              <option value="aes256">AES-256 Encryption</option>
              <option value="rc4">RC4 Encryption</option>
            </select>
          </div>
        </div>

        <button @click="generatePayload" :disabled="compiling" class="btn w-full mt-2 py-2 text-sm uppercase tracking-wider font-bold shadow-[0_0_10px_rgba(153,51,255,0.2)] hover:shadow-[0_0_15px_rgba(153,51,255,0.5)]">
          {{ compiling ? 'Compiling...' : 'Generate Payload' }}
        </button>
      </div>

      <!-- Output / Logs -->
      <div class="w-2/3 flex flex-col gap-2">
        <div class="flex-1 bg-black rounded border border-gray-800 p-3 overflow-y-auto font-mono text-xs">
          <div v-if="logs.length === 0" class="text-gray-600 italic text-center h-full flex items-center justify-center">
            Configure payload settings and click Generate...
          </div>
          <div v-for="(log, i) in logs" :key="i" class="mb-1 leading-relaxed whitespace-pre-wrap" :class="logClass(log.type)">
            {{ log.text }}
          </div>
          <div v-if="compiling" class="text-purple animate-pulse mt-2">
            [+] Building payload...
          </div>
        </div>

        <!-- Result Actions -->
        <div v-if="payloadReady" class="bg-panel border border-neon/30 p-3 rounded flex items-center justify-between animate-fade-in">
          <div>
            <div class="text-neon text-sm font-bold mb-1">Payload Compilation Successful</div>
            <div class="text-gray-400 text-xs">vesper_implant_{{ config.os.toLowerCase() }}_{{ config.arch }}.{{ config.format }} ({{ payloadSize }})</div>
          </div>
          <div class="flex gap-2">
            <button @click="copyBase64" class="btn text-xs bg-dark hover:bg-gray-800">Copy B64</button>
            <button @click="download" class="btn text-xs bg-neon text-black hover:bg-green-400">Download</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'

const config = ref({
  os: 'Windows',
  arch: 'x64',
  format: 'exe',
  lhost: '',
  lport: 8443,
  amsi: true,
  unhook: false,
  encoder: 'none'
})

// Auto-adjust format when OS changes
watch(() => config.value.os, (newOS) => {
  if (newOS === 'Windows') config.value.format = 'exe'
  if (newOS === 'Linux') config.value.format = 'elf'
  if (newOS === 'macOS') config.value.format = 'macho'
  if (newOS !== 'Windows') {
    config.value.amsi = false
    config.value.unhook = false
  }
})

const logs = ref([])
const compiling = ref(false)
const payloadReady = ref(false)
const payloadSize = ref('0 KB')
const b64Data = ref('')

const logClass = (type) => {
  if (type === 'info') return 'text-gray-300'
  if (type === 'success') return 'text-neon'
  if (type === 'error') return 'text-red-500'
  if (type === 'cmd') return 'text-purple'
  if (type === 'warn') return 'text-orange'
  return 'text-gray-300'
}

const addLog = (text, type = 'info') => {
  logs.value.push({ text, type })
}

const generatePayload = async () => {
  if (!config.value.lhost) {
    addLog('[!] LHOST is required', 'error')
    return
  }

  logs.value = []
  payloadReady.value = false
  compiling.value = true

  addLog(`[*] Initializing payload builder engine...`, 'info')
  addLog(`[*] OS: ${config.value.os} | Arch: ${config.value.arch} | Format: ${config.value.format}`, 'info')
  addLog(`[*] C2 Endpoint: wss://${config.value.lhost}:${config.value.lport}`, 'info')

  if (config.value.amsi) addLog(`[+] Injecting AMSI/ETW bypass stubs`, 'warn')
  if (config.value.unhook) addLog(`[+] Resolving direct syscalls for EDR unhooking (Halo's Gate)`, 'warn')
  if (config.value.encoder !== 'none') addLog(`[+] Applying ${config.value.encoder} obfuscation...`, 'warn')

  try {
    const res = await fetch('/api/payload/generate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(config.value)
    })

    if (!res.ok) {
      const err = await res.text()
      addLog(`[!] Compilation failed: ${err}`, 'error')
      compiling.value = false
      return
    }

    const data = await res.json()
    
    // Simulate compilation delay for dramatic effect
    setTimeout(() => {
      addLog(`[+] Generating cryptographic identity (X25519)`, 'info')
    }, 800)
    
    setTimeout(() => {
      addLog(`[+] Linking binary modules...`, 'cmd')
      data.logs.forEach(l => addLog(`compiler: ${l}`, 'gray'))
    }, 1500)

    setTimeout(() => {
      payloadSize.value = data.size
      b64Data.value = data.b64
      addLog(`\n[✓] Build completed successfully. Size: ${data.size}`, 'success')
      payloadReady.value = true
      compiling.value = false
    }, 2800)

  } catch (err) {
    addLog(`[!] Network error: ${err.message}`, 'error')
    compiling.value = false
  }
}

const copyBase64 = () => {
  navigator.clipboard.writeText(b64Data.value)
  addLog('[*] Base64 payload copied to clipboard', 'info')
}

const download = () => {
  const link = document.createElement('a')
  link.href = 'data:application/octet-stream;base64,' + b64Data.value
  link.download = `vesper_implant_${config.value.os.toLowerCase()}_${config.value.arch}.${config.value.format}`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  addLog('[*] Download started', 'info')
}
</script>
