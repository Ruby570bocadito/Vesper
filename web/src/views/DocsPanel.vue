<template>
  <div class="glass-panel p-4 space-y-4 overflow-y-auto h-full">
    <h3 class="text-sm font-mono text-purple">Vesper — Architecture & Documentation</h3>
    <div class="prose prose-invert max-w-none text-sm">
      <h4 class="text-neon font-mono">Vesper Framework</h4>
      <p class="text-gray-400">
        Semi-autonomous Red Team platform covering the complete cyber kill chain
        with 11 integrated modules and AI-powered decision making.
      </p>

      <h4 class="text-neon font-mono mt-4">Kill Chain Phases</h4>
      <div class="grid grid-cols-2 gap-2 text-xs font-mono mt-2">
        <div v-for="(p, i) in phases" :key="i" class="glass-panel p-2">
          <div class="text-neon">{{ i + 1 }}. {{ p.name }}</div>
          <div class="text-gray-500 text-xs">{{ p.modules }}</div>
        </div>
      </div>

      <h4 class="text-neon font-mono mt-4">Component Registry</h4>
      <table class="w-full text-xs font-mono mt-2">
        <thead><tr class="text-gray-600 border-b border-gray-800"><th class="text-left py-1">Component</th><th class="text-left">Lang</th><th class="text-left">Phase</th></tr></thead>
        <tbody class="text-gray-400">
          <tr v-for="c in components" :key="c.name">
            <td class="py-1 text-purple">{{ c.name }}</td><td>{{ c.lang }}</td><td>{{ c.phase }}</td>
          </tr>
        </tbody>
      </table>

      <h4 class="text-neon font-mono mt-4">Quick Commands</h4>
      <div class="text-gray-500 text-xs font-mono space-y-1 mt-2">
        <div><span class="text-purple">vesper</span> — Launch interactive TUI</div>
        <div><span class="text-purple">vesper console</span> — msfconsole-style shell</div>
        <div><span class="text-purple">vesper dashboard</span> — Start API backend</div>
        <div><span class="text-purple">make build</span> — Build all components</div>
        <div><span class="text-purple">make lab-up</span> — Docker lab environment</div>
      </div>
    </div>
  </div>
</template>

<script setup>
const phases = [
  { name: 'Reconnaissance', modules: 'Horizon-Intel, Specter' },
  { name: 'Weaponization', modules: 'Pulse-C2 Builder, Apex' },
  { name: 'Delivery', modules: 'Breach-Entry, Evasion' },
  { name: 'Exploitation', modules: 'Rise-Privilege, Vault-Kernel' },
  { name: 'Installation', modules: 'Vault-Kernel, Agent' },
  { name: 'Command & Control', modules: 'Pulse-C2, Link-Relay' },
  { name: 'Actions on Objective', modules: 'Wormy-ML, BlueForge, Titan-Ops' },
]

const components = [
  { name: 'CLI vesper', lang: 'Go', phase: 'All' },
  { name: 'Orchestrator', lang: 'Go', phase: 'All' },
  { name: 'Decision Engine', lang: 'Go', phase: 'All' },
  { name: 'PostExploitPipeline', lang: 'Go', phase: '4-7' },
  { name: 'Pulse-C2', lang: 'Go + Vue 3', phase: 'C2' },
  { name: 'Rise-Privilege', lang: 'Go', phase: 'Exploit' },
  { name: 'Vault-Kernel', lang: 'C + Go', phase: 'Install' },
  { name: 'Breach-Entry', lang: 'C + Python', phase: 'Deliver' },
  { name: 'Horizon-Intel', lang: 'Python', phase: 'Recon' },
  { name: 'Specter + Apex', lang: 'Python', phase: 'AI' },
  { name: 'Wormy-ML', lang: 'Python', phase: 'Actions' },
  { name: 'Link-Relay', lang: 'Python', phase: 'C2' },
  { name: 'BlueForge-Suite', lang: 'Python', phase: 'Metrics' },
  { name: 'Python Bridge', lang: 'Python', phase: 'IPC' },
]
</script>
