<template>
  <div class="glass-panel p-3 h-full flex flex-col">
    <!-- Header -->
    <div class="flex items-center justify-between mb-3">
      <h3 class="text-sm font-mono text-purple">AI CONSOLE — Specter</h3>
      <div class="flex items-center gap-2 text-xs font-mono">
        <span class="inline-block w-2 h-2 rounded-full bg-neon animate-pulse-slow"></span>
        <button @click="showConfig = true" class="text-gray-500 hover:text-purple flex items-center gap-1 cursor-pointer" title="AI Configuration">
          <span>{{ modelLabel }}</span>
          <span class="text-[10px]">⚙️</span>
        </button>
      </div>
    </div>

    <!-- Chat area -->
    <div class="flex-1 overflow-y-auto font-mono text-xs space-y-2 mb-3" ref="aiChat">
      <div v-for="(msg, i) in messages" :key="i" class="leading-relaxed">
        <span :class="labelClass(msg.role)" class="mr-1">[{{ labelText(msg.role) }}]</span>
        <span class="text-gray-300 whitespace-pre-wrap">{{ msg.content }}</span>
      </div>
      <div v-if="store.loading" class="text-gray-500 italic animate-pulse">[Specter] Processing...</div>
    </div>

    <!-- Quick commands -->
    <div class="flex flex-wrap gap-1 mb-2">
      <button v-for="cmd in quickCmds" :key="cmd" @click="sendQuick(cmd)"
        class="text-xs px-2 py-0.5 rounded border border-gray-800 text-gray-500 hover:text-purple hover:border-purple transition-colors font-mono"
        :disabled="store.loading">
        {{ cmd }}
      </button>
    </div>

    <!-- Input bar -->
    <div class="flex gap-2">
      <input v-model="prompt" @keyup.enter="send"
        class="flex-1 bg-dark border border-gray-800 rounded px-3 py-2 text-xs font-mono text-gray-300 focus:border-purple focus:outline-none"
        placeholder="Ask Specter anything..." :disabled="store.loading" />
      <button @click="send" class="btn text-xs px-4" :disabled="!prompt.trim() || store.loading">Send</button>
    </div>

    <!-- Config Modal -->
    <div v-if="showConfig" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div class="bg-dark border border-gray-800 p-4 rounded w-96 shadow-2xl">
        <div class="flex justify-between items-center mb-4">
          <h4 class="text-purple font-mono text-sm">AI Configuration</h4>
          <button @click="showConfig = false" class="text-gray-500 hover:text-white">✕</button>
        </div>
        
        <div class="space-y-3 font-mono text-xs">
          <div>
            <label class="block text-gray-500 mb-1">Model (Ollama local)</label>
            <input v-model="config.model" class="w-full bg-black border border-gray-800 rounded px-2 py-1 text-gray-300 focus:border-purple focus:outline-none" placeholder="llama3.1:8b" />
          </div>
          <!-- provider / api-key selects removed: the backend only
               supports the local Ollama provider and silently dropped
               everything else — showing them was placebo -->
          <div>
            <label class="block text-gray-500 mb-1">Temperature</label>
            <input type="number" step="0.1" min="0" max="2" v-model.number="config.temperature" class="w-full bg-black border border-gray-800 rounded px-2 py-1 text-gray-300 focus:border-purple focus:outline-none" />
          </div>
          <div class="pt-2">
            <button @click="saveConfig" class="btn w-full" :disabled="saving">
              {{ saving ? 'Saving...' : 'Save Configuration' }}
            </button>
          </div>
          <div class="text-[10px] text-gray-600 mt-2 text-center">
            Ollama {{ config.ollama_host }}:{{ config.ollama_port }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick, onMounted } from 'vue'
import { useAIStore } from '../stores/index.js'

const store = useAIStore()
const prompt = ref('')
const aiChat = ref(null)
const modelLabel = ref('connecting...')
const messages = store.messages

const quickCmds = ['suggest', 'analyze target', 'privesc', 'lateral', 'persist', 'help']

const labelText = (role) => {
  if (role === 'user') return 'You'
  if (role === 'system') return 'Specter'
  return 'Specter'
}
const labelClass = (role) => {
  if (role === 'user') return 'text-purple'
  if (role === 'system') return 'text-gray-500'
  return 'text-neon'
}

const scrollBottom = async () => {
  await nextTick()
  if (aiChat.value) aiChat.value.scrollTop = aiChat.value.scrollHeight
}

// Seed with welcome message if empty
if (messages.length === 0) {
  messages.push({ role: 'system', content: 'Specter online. Type a prompt, use quick commands, or type "help".' })
}

const send = async () => {
  if (!prompt.value.trim() || store.loading) return
  const text = prompt.value.trim()
  prompt.value = ''
  await store.chat(text)
  await scrollBottom()
}

const sendQuick = async (cmd) => {
  if (store.loading) return
  await store.chat(cmd)
  await scrollBottom()
}

const showConfig = ref(false)
const saving = ref(false)
const config = ref({
  model: 'llama3.2',
  temperature: 0.7,
  ollama_host: 'localhost',
  ollama_port: 11434,
})

// Probe config
onMounted(async () => {
  try {
    const res = await fetch('/api/config/ai')
    if (res.ok) {
      const data = await res.json()
      config.value = { ...config.value, ...data }
      modelLabel.value = data.enabled ? data.model : 'local-fallback'
    } else {
      modelLabel.value = 'offline'
    }
  } catch {
    modelLabel.value = 'offline'
  }
  await scrollBottom()
})

const saveConfig = async () => {
  saving.value = true
  try {
    const res = await fetch('/api/config/ai', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        model: config.value.model,
        temperature: parseFloat(config.value.temperature)
      })
    })
    if (res.ok) {
      modelLabel.value = config.value.model
      showConfig.value = false
    }
  } catch (err) {
    console.error('Failed to save AI config:', err)
  }
  saving.value = false
}
</script>
