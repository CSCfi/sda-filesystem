<script lang="ts" setup>
import { CToastMessage, CToastType } from "@cscfi/csc-ui/dist/types";
import { ref, computed, onMounted } from "vue";
import { EventsOn, EventsEmit } from "../wailsjs/runtime";
import { InitializeAPI, Quit } from "../wailsjs/go/main/App";
import { mdiLogoutVariant } from "@mdi/js";

interface ComponentType {
  name: string
  tab: string
  visible: boolean
  props?: {[key:string]:boolean}
  active?: boolean
  disabled?: boolean
}

const disabled = ref(false);
const initialized = ref(false);
const selected = ref(false);
const accessed = ref(false);

const currentTab = ref("Log in");
const componentData = computed<ComponentType[]>(() => ([
  {
    name: "SelectPage",
    tab: "Log in",
    visible: !selected.value,
    props: {
      initialized: initialized.value,
      disabled: disabled.value,
    },
  },
  { name: "AccessPage", tab: "Access", visible: selected.value, active: selected.value },
  { name: "ExportPage", tab: "Export", visible: accessed.value },
  { name: "LogsPage", tab: "Logs", visible: true }
]));

const visibleTabs = computed(() => componentData.value.filter((data) => data.visible));
// eslint-disable-next-line no-undef
const toasts = ref<HTMLCToastsElement | null>(null);

onMounted(() => {
  InitializeAPI().then((access: boolean) => {
    console.log("Initializing Data Gateway finished");
    initialized.value = true;
    if (!access) {
      disabled.value = true;
      EventsEmit("showToast", "Relogin to SD Desktop", "Your session has expired");
    }
  }).catch((e) => {
    disabled.value = true;
    EventsEmit("showToast", "Initializing Data Gateway failed", e as string);
  });
});

EventsOn("showToast", (title: string, err: string) => {
  const message: CToastMessage = {
    title: title,
    message: err,
    type: "error" as CToastType,
    persistent: true,
  };

  toasts.value?.addToast(message);
});

EventsOn("selectFinished", () => {
  selected.value = true;
  currentTab.value = "Access";
});

EventsOn("fuseReady", () => (accessed.value = true));
</script>

<template>
  <div id="main">
    <c-toolbar>
      <c-csc-logo />
      <h4>Data Gateway</h4>
      <c-spacer />
      <div id="tab-wrapper">
        <c-tabs v-model="currentTab" v-control borderless>
          <c-tab
            v-for="tab in visibleTabs"
            :key="tab.tab"
            :value="tab.tab"
            :active="tab.active"
            :disabled="tab.disabled"
          >
            {{ tab.tab }}
          </c-tab>
          <c-tab-items>
            <c-tab-item
              v-for="tab in visibleTabs"
              :key="tab.tab"
              :value="tab.tab"
            />
          </c-tab-items>
        </c-tabs>
      </div>
      <c-spacer />
      <c-button
        id="sign-out-button"
        size="small"
        text
        no-radius
        @click="Quit"
      >
        {{ accessed ? 'Disconnect and sign out' : 'Sign out' }}
        <c-icon :path="mdiLogoutVariant" />
      </c-button>
    </c-toolbar>

    <div id="content">
      <component
        :is="data.name"
        v-for="data in componentData"
        v-show="data.tab === currentTab"
        :key="data.name"
        v-bind="data.props"
      />
    </div>

    <c-toasts ref="toasts" />
  </div>
</template>

<style scoped>
#main {
  display: flex;
  flex-direction: column;
  height: 100%;
}

c-toolbar {
  position: relative;
}

c-toolbar > h4 {
  white-space: nowrap;
}

c-csc-logo {
  flex-shrink: 0;
}

c-tab-item, c-tab-items {
  display: none;
}

c-tab {
  width: 80px;
}

#tab-wrapper {
  height: 60px;
  display: flex;
  align-items: flex-end;
  flex-grow: 1;
}

c-button {
  align-self: center;
}

#content {
  margin: 40px;
  display: flex;
  flex-direction: column;
  align-items: center;
  /* Keep itemsPerPage dropdown in place */
  overflow-y: auto;
}
</style>
