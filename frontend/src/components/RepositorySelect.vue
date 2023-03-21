<script lang="ts" setup>
import { ref, watch } from 'vue'
import { Authenticate } from '../../wailsjs/go/main/App'
import { EventsEmit } from '../../wailsjs/runtime/runtime'

import LoginForm from './LoginForm.vue';

const props = defineProps<{
    disabled: boolean,
    initialized: boolean,
    useForm: boolean,
    repository: string,
}>()

const emit = defineEmits(['selected'])

const selected = ref(false)
const loading = ref(false)

watch(() => selected.value, (sel: boolean) => { 
    loading.value = sel;
    if (sel && !props.useForm) {
        Authenticate(props.repository).then(() => {
            success();
        }).catch(e => {
            selected.value = false;
            EventsEmit("showToast", "Access refused", e as string);
        }).finally(() => {
            loading.value = false;
        });
    }
})

function success() {
    emit("selected");
}
</script>

<template>
    <c-row align="start">
        <c-row align="center" class="switch-row">
            <c-switch 
                :value="selected"
                :style="{'pointer-events': (selected && !useForm) ? 'none' : 'auto'}"
                :disabled="props.disabled"
                @changeValue="selected = $event.target.value">
            </c-switch>
            <c-loader :hide="!loading"></c-loader>
            <div class="repository-name">{{ props.repository.replace("-", " ") }}</div>
        </c-row>
        <div>
            <c-switch :style="{ visibility: 'hidden' }"></c-switch>
            <LoginForm 
                class="login-form"
                v-if="useForm && selected && loading" 
                @loggedIn="success()"
                :initialized="props.initialized"
                small>
            </LoginForm>
        </div>
    </c-row>
</template>

<style scoped>
c-switch {
    padding-bottom: 8px;
    padding-top: 8px;
}

.switch-row {
    position: relative;
}

.repository-name {
    white-space: nowrap;
    position: absolute;
    padding-left: 10px;
    left: 100%;
}

.login-form {
    width: 400px;
    padding-left: 10px;
}
</style>