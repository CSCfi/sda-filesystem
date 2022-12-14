<script lang="ts" setup>
import { CToastMessage, CToastType } from 'csc-ui/dist/types'
import { ref, onMounted } from 'vue'
import { EventsOn, EventsEmit } from '../wailsjs/runtime'
import { InitializeAPI } from '../wailsjs/go/main/App'

import Access from './components/Access.vue'
import Export from './components/Export.vue'
import Login from './components/Login.vue'
import Logs from './components/Logs.vue'

const page = ref("login")
const disabled = ref(false)
const initialized = ref(false)
const loggedIn = ref(false)
const toast = ref<any>(null)
//<ComponentPublicInstance<typeof CToast> | null>

onMounted(() => {
  InitializeAPI().then(() => {
    console.log("Initializing Data Gateway finished");
    initialized.value = true;
  }).catch(e => {
    disabled.value = true;
    EventsEmit("showToast", "Initializing Data Gateway failed", e as string);
  });
})

EventsOn('showToast', function(title: string, err: string) {
  const message: CToastMessage = {
    title: title,
    message: err,
    type: "error" as CToastType,
    persistent: true,
  };

  toast.value.addToast?.(message);
})
</script>

<template>
  <c-main>
    <c-toolbar class="relative">
      <c-csc-logo></c-csc-logo>
      <h4>Data Gateway</h4>
      <c-spacer></c-spacer>
      <c-tabs id="tabs" :value="page" borderless @changeValue="(page = ($event.target as HTMLInputElement).value)">
        <c-tab value="login" v-show="!loggedIn">Login</c-tab>
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

    <div id="content">
      <Login v-show="page === 'login'" :ready="initialized" :disabled="disabled" @proceed="loggedIn = true"/>
      <Access v-show="page === 'access'" />
      <Export v-show="page === 'export'" />
      <Logs v-show="page === 'logs'" />
    </div>

    <c-toasts ref="toast"></c-toasts>
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

#content {
  margin: 40px;
  display: flex;
}
</style>