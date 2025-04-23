<script lang="ts" setup>
import { CToastMessage, CToastType } from '@cscfi/csc-ui/dist/types'
import { ref, computed, onMounted } from 'vue'
import { EventsOn, EventsEmit } from '../wailsjs/runtime'
import { InitializeAPI, InitFuse, Quit } from '../wailsjs/go/main/App'
import { mdiLogoutVariant } from '@mdi/js'

interface ComponentType {
    name: string
    visible: boolean
    props?: {[key:string]:boolean}
    active?: boolean
    disabled?: boolean
}

const disabled = ref(false)
const initialized = ref(false)
const selected = ref(false)
const accessed = ref(false)

const currentPage = ref("Select")
const componentData = computed<ComponentType[]>(() => ([
    {
        name: "Select",
        visible: !selected.value,
        props: {
            initialized: initialized.value,
            disabled: disabled.value,
        },
    },
    { name: "Access", visible: selected.value, active: selected.value },
    { name: "Export", visible: selected.value, disabled: !accessed.value },
    { name: "Logs", visible: true }
]))

const visibleTabs = computed(() => componentData.value.filter(data => data.visible))

const toasts = ref<HTMLCToastsElement | null>(null);

onMounted(() => {
    InitializeAPI().then((access: boolean) => {
        console.log("Initializing Data Gateway finished");
        initialized.value = true;
        if (!access) {
            disabled.value = true;
            EventsEmit("showToast", "Relogin to SD Desktop", "Your session has expired");
        }
    }).catch(e => {
        disabled.value = true;
        EventsEmit("showToast", "Initializing Data Gateway failed", e as string);
    });
})

EventsOn('showToast', (title: string, err: string) => {
    const message: CToastMessage = {
        title: title,
        message: err,
        type: "error" as CToastType,
        persistent: true,
    };

    toasts.value?.addToast(message);
})

EventsOn('selectFinished', () => {
    selected.value = true;
    currentPage.value = 'Access';
})

EventsOn('fuseReady', () => (accessed.value = true))
</script>

<template>
    <div id="main">
        <c-toolbar>
            <c-csc-logo></c-csc-logo>
            <h4>Data Gateway</h4>
            <c-spacer></c-spacer>
            <div id="tab-wrapper">
                <c-tabs v-model="currentPage" borderless v-control>
                    <c-tab
                        v-for="tab in visibleTabs"
                        :value="tab.name"
                        :active="tab.active"
                        :disabled="tab.disabled"
                    >{{ tab.name }}</c-tab>
                    <c-tab-items>
                    <c-tab-item
                        v-for="tab in visibleTabs"
                        :value="tab.name"
                    ></c-tab-item>
                    </c-tab-items>
                </c-tabs>
            </div>
            <c-spacer></c-spacer>
            <c-button
                id="sign-out-button"
                size="small"
                text
                no-radius
                @click="Quit">
                Disconnect and sign out
                <c-icon :path="mdiLogoutVariant"></c-icon>
            </c-button>
        </c-toolbar>

        <div id="content">
            <component
                v-for="data in componentData"
                v-show="data.name === currentPage"
                :is="data.name"
                v-bind="data.props">
            </component>
        </div>

        <c-toasts ref="toasts"></c-toasts>
    </div>
</template>

<style scoped>
#main {
    display: flex;
    flex-direction: column;
    height: 100vh;
}

c-toolbar {
    position: relative;
    font-size: 0.9em;
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

#sign-out-button {
    margin-right: 2rem;
}
</style>
