<script lang="ts" setup>
import { CToastMessage, CToastType } from 'csc-ui/dist/types'
import { ref, computed, onMounted } from 'vue'
import { EventsOn, EventsEmit, Quit } from '../wailsjs/runtime'
import { InitializeAPI } from '../wailsjs/go/main/App'

interface ComponentType {
    name: string
    visible: boolean
    props?: {[key:string]:boolean}
    active?: boolean
    disabled?: boolean
}

const disabled = ref(false)
const initialized = ref(false)
const loggedIn = ref(false)
const accessed = ref(false)

const currentPage = ref("Login")
const componentData = computed<ComponentType[]>(() => ([
    {
        name: "Login",
        visible: !loggedIn.value,
        props: {initialized: initialized.value, disabled: disabled.value},
    },
    { name: "Access", visible: loggedIn.value, active: loggedIn.value },
    { name: "Export", visible: loggedIn.value, disabled: !accessed.value },
    { name: "Logs", visible: true }
]))

const visibleTabs = computed(() => componentData.value.filter(data => data.visible))

const toasts = ref<HTMLCToastsElement | null>(null);

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

    toasts.value?.addToast(message);
})

EventsOn('loggedIn', () => {loggedIn.value = true; currentPage.value = 'Access'})

EventsOn('fuseReady', () => (accessed.value = true))
</script>

<template>
    <c-main>
        <c-toolbar class="relative">
            <c-csc-logo></c-csc-logo>
            <h4>Data Gateway</h4>
            <c-spacer></c-spacer>
            <c-tabs id="tabs" v-model="currentPage" borderless v-control>
                <c-tab
                    v-for="tab in visibleTabs"
                    :value="tab.name"
                    :active="tab.active"
                    :disabled="tab.disabled"
                >{{ tab.name }}</c-tab>
            </c-tabs>
            <c-spacer></c-spacer>
            <c-button 
                size="small" 
                text 
                no-radius 
                icon-end 
                @click="Quit"
                :style="{visibility: loggedIn ? 'visible' : 'hidden'}">
                <i class="material-icons" slot="icon">logout</i>
                Disconnect and sign out
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
