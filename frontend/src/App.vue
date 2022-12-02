<script lang="ts" setup>
  import { ref, onMounted } from 'vue'
  import type { Ref } from 'vue'
  import { InitializeAPI } from '../wailsjs/go/main/App'

  import Access from './components/Access.vue'
  import Export from './components/Export.vue'
  import Login from './components/Login.vue'
  import Logs from './components/Logs.vue'
  import { on } from 'events'

  const page: Ref<string> = ref("login")
  const mountpoint: Ref<string> = ref("")
  const loggedIn: Ref<boolean> = ref(false)
  const isProjectManager: Ref<boolean> = ref(false)
  
  onMounted(() => {
    InitializeAPI().then((result: string) => {
      if (result !== "") {
        console.log(result) // Change to popup
      }
    })
  })

</script>

<template>
  <c-main>
    <c-toolbar class="relative">
      <c-csc-logo></c-csc-logo>
      <h4>Data Gateway</h4>
      <c-spacer></c-spacer>
      <c-tabs id="tabs" :value="page" borderless @changeValue="(page = ($event.target as HTMLInputElement).value)">
        <c-tab value="login">Login</c-tab>
        <c-tab value="access" v-show="loggedIn">Access</c-tab>
        <c-tab value="export" v-show="loggedIn">Export</c-tab>
        <c-tab value="logs">Logs</c-tab>
      </c-tabs>
      <c-spacer></c-spacer>
      <c-button size="small" text no-radius icon-end :style="{visibility: loggedIn ? 'visible' : 'hidden'}">
        <i class="material-icons" slot="icon">logout</i>
        Disconnect and sign out
      </c-button>
    </c-toolbar>

    <Login v-if="page === 'login'" />
    <Access v-else-if="page === 'access'" />
    <Export v-else-if="page === 'export'" />
    <Logs v-else-if="page === 'logs'" />
  </c-main>
</template>

<style>
c-main {
  background-color: white;
}

c-toolbar {
  font-size: 0.9em;
}

c-toolbar > h4 {
  white-space: nowrap;
}

c-csc-logo {
  flex-shrink: 0;
}

c-tab {
  width: 80px;
}

c-tabs {
  height: 100%;
  display: flex;
  align-items: flex-end;
  flex-grow: 1;
}
</style>
